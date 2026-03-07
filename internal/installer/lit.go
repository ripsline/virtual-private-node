// internal/installer/lit.go

package installer

import (
	"fmt"
	"os"
	"strings"

	"github.com/ripsline/virtual-private-node/internal/config"
	"github.com/ripsline/virtual-private-node/internal/paths"
	"github.com/ripsline/virtual-private-node/internal/system"
)

func downloadLIT(version string) error {
	filename := fmt.Sprintf("lightning-terminal-linux-amd64-v%s.tar.gz", version)
	url := fmt.Sprintf(
		"https://github.com/lightninglabs/lightning-terminal/releases/download/v%s/%s",
		version, filename)
	manifestURL := fmt.Sprintf(
		"https://github.com/lightninglabs/lightning-terminal/releases/download/v%s/manifest-v%s.txt",
		version, version)
	if err := system.DownloadRequireTor(url, "/tmp/"+filename); err != nil {
		return err
	}
	if err := system.DownloadRequireTor(manifestURL, "/tmp/lit-manifest.txt"); err != nil {
		return fmt.Errorf("download LIT manifest: %w", err)
	}
	return nil
}

func extractAndInstallLIT(version string) error {
	filename := fmt.Sprintf("lightning-terminal-linux-amd64-v%s.tar.gz", version)
	if err := system.Run("tar", "-xzf", "/tmp/"+filename, "-C", "/tmp"); err != nil {
		return err
	}
	extractDir := fmt.Sprintf("/tmp/lightning-terminal-linux-amd64-v%s", version)
	if err := system.SudoRun("install", "-m", "0755", "-o", "root", "-g", "root",
		extractDir+"/litd", "/usr/local/bin/"); err != nil {
		return err
	}
	os.Remove("/tmp/" + filename)
	os.Remove("/tmp/lit-manifest.txt")
	os.RemoveAll(extractDir)
	return nil
}

func createLITDirs() error {
	dirs := []struct {
		path  string
		owner string
		mode  os.FileMode
	}{
		{paths.LITDir, "root:" + systemUser, 0750},
		{paths.LITDataDir, systemUser + ":" + systemUser, 0750},
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

func enableRPCMiddleware() error {
	output, err := system.SudoRunOutput("cat", paths.LNDConf)
	if err != nil {
		return err
	}
	if strings.Contains(output, "rpcmiddleware.enable=true") {
		return nil
	}
	// Append as its own INI section. This must NOT go inside [Tor]
	// or any other section — LND's config parser would treat it as
	// an unknown option for that section.
	output += "\n[rpcmiddleware]\nrpcmiddleware.enable=true\n"
	if err := system.SudoWriteFile(paths.LNDConf, []byte(output), 0640); err != nil {
		return err
	}
	return system.SudoRun("chown", "root:"+systemUser, paths.LNDConf)
}

func writeLITConfig(cfg *config.AppConfig, uiPassword string) error {
	macaroonNetwork := cfg.Network
	if cfg.IsMainnet() {
		macaroonNetwork = "mainnet"
	}
	macaroonPath := paths.LNDMacaroon(macaroonNetwork)
	content := fmt.Sprintf(`# Virtual Private Node — Lightning Terminal
uipassword=%s
lnd-mode=remote
network=%s
lit-dir=/var/lib/lit

remote.lnd.rpcserver=localhost:10009
remote.lnd.macaroonpath=%s
remote.lnd.tlscertpath=/var/lib/lnd/tls.cert

faraday-mode=disable
loop-mode=disable
pool-mode=disable
taproot-assets-mode=disable

autopilot.disable=true

httpslisten=127.0.0.1:8443
`, uiPassword, cfg.Network, macaroonPath)

	if err := system.SudoWriteFile(paths.LITConf, []byte(content), 0640); err != nil {
		return err
	}
	return system.SudoRun("chown", "root:"+systemUser, paths.LITConf)
}

func writeLITDService(username string) error {
	content := fmt.Sprintf(`[Unit]
Description=Lightning Terminal
After=lnd.service
Wants=lnd.service

[Service]
Type=simple
User=%s
Group=%s
ExecStart=/usr/local/bin/litd --configfile=/etc/lit/lit.conf
Restart=on-failure
RestartSec=30
TimeoutStopSec=120
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`, username, username)
	return system.SudoWriteFile(paths.LITDService, []byte(content), 0644)
}

func startLITD() error {
	if err := system.SudoRun("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := system.SudoRun("systemctl", "enable", "litd"); err != nil {
		return err
	}
	return system.SudoRun("systemctl", "start", "litd")
}
