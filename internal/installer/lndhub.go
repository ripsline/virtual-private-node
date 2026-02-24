// internal/installer/lndhub.go

package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/ripsline/virtual-private-node/internal/config"
	"github.com/ripsline/virtual-private-node/internal/logger"
	"github.com/ripsline/virtual-private-node/internal/paths"
	"github.com/ripsline/virtual-private-node/internal/system"
)

const (
	lndhubVersion = "1.0.2"
	lndhubRepo    = "https://github.com/getAlby/lndhub.go.git"
	goVersion     = "1.26.0"
	goTarball     = "go1.26.0.linux-amd64.tar.gz"
	goDownloadURL = "https://go.dev/dl/go1.26.0.linux-amd64.tar.gz"
	goInstallDir  = "/usr/local/go"
)

func LndHubVersionStr() string { return lndhubVersion }

// ── Go toolchain ─────────────────────────────────────────

func installGoToolchain() error {
	goPath := goInstallDir + "/bin/go"
	if _, err := os.Stat(goPath); err == nil {
		output, err := system.RunContext(5e9, goPath, "version")
		if err == nil && output != "" {
			logger.Install("Go already installed: %s", output)
			return nil
		}
	}

	tarball := "/tmp/" + goTarball
	if err := system.Download(goDownloadURL, tarball); err != nil {
		return fmt.Errorf("download Go: %w", err)
	}
	defer os.Remove(tarball)

	system.SudoRunSilent("rm", "-rf", goInstallDir)

	if err := system.SudoRun("tar", "-C", "/usr/local", "-xzf", tarball); err != nil {
		return fmt.Errorf("extract Go: %w", err)
	}

	logger.Install("Go toolchain installed: %s", goVersion)
	return nil
}

// ── PostgreSQL ───────────────────────────────────────────

func installPostgreSQL() error {
	if err := system.SudoRun("apt-get", "install", "-y", "-qq",
		"postgresql", "postgresql-client"); err != nil {
		return err
	}
	if err := system.SudoRun("systemctl", "enable", "postgresql"); err != nil {
		return err
	}
	return system.SudoRun("systemctl", "start", "postgresql")
}

func createLndHubDatabase(dbPassword string) error {
	checkCmd := exec.Command("sudo", "-u", "postgres", "psql", "-tAc",
		"SELECT 1 FROM pg_roles WHERE rolname='lndhub'")
	checkOutput, _ := checkCmd.CombinedOutput()
	if string(checkOutput) == "1\n" {
		logger.Install("PostgreSQL user lndhub already exists")
		return nil
	}

	createUser := fmt.Sprintf("CREATE USER lndhub WITH PASSWORD '%s'", dbPassword)
	if err := system.SudoRun("sudo", "-u", "postgres", "psql", "-c", createUser); err != nil {
		return fmt.Errorf("create postgres user: %w", err)
	}
	if err := system.SudoRun("sudo", "-u", "postgres", "psql", "-c",
		"CREATE DATABASE lndhub OWNER lndhub"); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	logger.Install("PostgreSQL database and user created")
	return nil
}

// ── Build from source ────────────────────────────────────

func cloneLndHub() error {
	os.RemoveAll("/tmp/lndhub.go")

	if err := system.Run("git", "clone", "--branch", lndhubVersion,
		"--depth", "1", lndhubRepo, "/tmp/lndhub.go"); err != nil {
		return fmt.Errorf("clone lndhub.go: %w", err)
	}
	logger.Install("Cloned lndhub.go at tag %s", lndhubVersion)
	return nil
}

func buildLndHub() error {
	goPath := goInstallDir + "/bin/go"

	cmd := exec.Command(goPath, "build", "-trimpath",
		"-ldflags=-s -w",
		"-o", "/tmp/lndhub.go/lndhub",
		"./cmd/server/")
	cmd.Dir = "/tmp/lndhub.go"
	cmd.Env = append(os.Environ(),
		"GOPATH=/tmp/go-build",
		"GOCACHE=/tmp/go-cache",
		"PATH="+goInstallDir+"/bin:"+os.Getenv("PATH"),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build lndhub: %s: %s", err, output)
	}

	logger.Install("Built lndhub from source")
	return nil
}

func installLndHubBinary() error {
	if err := system.SudoRun("install", "-m", "0755", "-o", "root", "-g", "root",
		"/tmp/lndhub.go/lndhub", "/usr/local/bin/lndhub"); err != nil {
		return err
	}

	os.RemoveAll("/tmp/lndhub.go")
	os.RemoveAll("/tmp/go-build")
	os.RemoveAll("/tmp/go-cache")

	logger.Install("Installed lndhub binary")
	return nil
}

// ── Macaroon ─────────────────────────────────────────────

func bakeLndHubMacaroon(cfg *config.AppConfig) error {
	net := cfg.NetworkConfig()

	cmd := exec.Command("sudo", "-u", systemUser, "lncli",
		"--lnddir="+paths.LNDDataDir,
		"--network="+net.LNCLINetwork,
		"bakemacaroon",
		"--save_to="+paths.LndHubMacaroon,
		"info:read", "invoices:read", "invoices:write",
		"offchain:read", "offchain:write")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bake macaroon: %s: %s", err, output)
	}

	if err := system.SudoRun("chmod", "0640", paths.LndHubMacaroon); err != nil {
		return err
	}
	if err := system.SudoRun("chown", systemUser+":"+systemUser, paths.LndHubMacaroon); err != nil {
		return err
	}

	logger.Install("Baked LndHub macaroon with restricted permissions")
	return nil
}

// ── Directories ──────────────────────────────────────────

func createLndHubDirs() error {
	dirs := []struct {
		path  string
		owner string
		mode  os.FileMode
	}{
		{paths.LndHubDir, "root:" + systemUser, 0750},
		{paths.LndHubDataDir, systemUser + ":" + systemUser, 0750},
	}
	for _, d := range dirs {
		if err := system.SudoRun("mkdir", "-p", d.path); err != nil {
			return err
		}
		if err := system.SudoRun("chown", d.owner, d.path); err != nil {
			return err
		}
		if err := system.SudoRun("chmod", fmt.Sprintf("%o", d.mode), d.path); err != nil {
			return err
		}
	}
	return nil
}

// ── Configuration ────────────────────────────────────────

func writeLndHubConfig(cfg *config.AppConfig, dbPassword, jwtSecret, adminToken string) error {
	content := fmt.Sprintf(`# Virtual Private Node — LndHub.go
DATABASE_URI=postgresql://lndhub:%s@localhost:5432/lndhub?sslmode=disable
JWT_SECRET=%s
JWT_ACCESS_EXPIRY=172800
JWT_REFRESH_EXPIRY=604800
LND_ADDRESS=localhost:10009
LND_MACAROON_FILE=%s
LND_CERT_FILE=%s
HOST=127.0.0.1
PORT=3000
ENABLE_PROMETHEUS=false
ALLOW_ACCOUNT_CREATION=true
ADMIN_TOKEN=%s
FEE_RESERVE=false
`, dbPassword, jwtSecret, paths.LndHubMacaroon, paths.LNDTLSCert, adminToken)

	if err := system.SudoWriteFile(paths.LndHubEnv, []byte(content), 0640); err != nil {
		return err
	}
	return system.SudoRun("chown", "root:"+systemUser, paths.LndHubEnv)
}

// ── Systemd ──────────────────────────────────────────────

func writeLndHubService() error {
	content := fmt.Sprintf(`[Unit]
Description=LndHub.go Lightning Accounts
After=lnd.service postgresql.service
Wants=lnd.service postgresql.service

[Service]
Type=simple
User=%s
Group=%s
EnvironmentFile=%s
ExecStart=/usr/local/bin/lndhub
Restart=on-failure
RestartSec=30
TimeoutStopSec=120
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`, systemUser, systemUser, paths.LndHubEnv)
	return system.SudoWriteFile(paths.LndHubService, []byte(content), 0644)
}

func startLndHub() error {
	if err := system.SudoRun("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := system.SudoRun("systemctl", "enable", "lndhub"); err != nil {
		return err
	}
	return system.SudoRun("systemctl", "start", "lndhub")
}

// ── Account creation ─────────────────────────────────────

type LndHubAccount struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func CreateLndHubAccount(adminToken string) (*LndHubAccount, error) {
	output, err := system.RunContext(10e9, "curl", "-s",
		"-X", "POST",
		"-H", "Content-Type: application/json",
		"-H", "Authorization: Bearer "+adminToken,
		"-d", "{}",
		"http://127.0.0.1:3000/create")
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	var account LndHubAccount
	if err := json.Unmarshal([]byte(output), &account); err != nil {
		return nil, fmt.Errorf("parse response: %w (%s)", err, output)
	}

	if account.Login == "" {
		return nil, fmt.Errorf("empty login in response: %s", output)
	}

	return &account, nil
}
