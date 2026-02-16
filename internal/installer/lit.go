package installer

import (
    "fmt"
    "os"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
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
    if err := system.Download(url, "/tmp/"+filename); err != nil {
        return err
    }
    if err := system.Download(manifestURL, "/tmp/lit-manifest.txt"); err != nil {
        return fmt.Errorf("download LIT manifest: %w", err)
    }
    return nil
}

func verifyLIT(version string) error {
    if _, err := os.Stat("/tmp/lit-manifest.txt"); err != nil {
        return fmt.Errorf("LIT manifest not found")
    }
    if err := system.Run("sha256sum", "--ignore-missing", "--check", "/tmp/lit-manifest.txt"); err != nil {
        return fmt.Errorf("checksum failed: %w", err)
    }
    return nil
}

func extractAndInstallLIT(version string) error {
    filename := fmt.Sprintf("lightning-terminal-linux-amd64-v%s.tar.gz", version)
    if err := system.Run("tar", "-xzf", "/tmp/"+filename, "-C", "/tmp"); err != nil {
        return err
    }
    extractDir := fmt.Sprintf("/tmp/lightning-terminal-linux-amd64-v%s", version)
    if err := system.Run("install", "-m", "0755", "-o", "root", "-g", "root",
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
        {"/etc/lit", "root:" + systemUser, 0750},
        {"/var/lib/lit", systemUser + ":" + systemUser, 0750},
    }
    for _, d := range dirs {
        if err := os.MkdirAll(d.path, d.mode); err != nil {
            return err
        }
        if err := system.Run("chown", d.owner, d.path); err != nil {
            return err
        }
        os.Chmod(d.path, d.mode)
    }
    return nil
}

func enableRPCMiddleware() error {
    data, err := os.ReadFile("/etc/lnd/lnd.conf")
    if err != nil {
        return err
    }
    content := string(data)
    if strings.Contains(content, "rpcmiddleware.enable=true") {
        return nil
    }
    addition := "\n# Required for Lightning Terminal\nrpcmiddleware.enable=true\n"
    if idx := strings.Index(content, "[Application Options]"); idx != -1 {
        lineEnd := strings.Index(content[idx:], "\n")
        if lineEnd != -1 {
            insertAt := idx + lineEnd + 1
            content = content[:insertAt] + addition + content[insertAt:]
        }
    } else {
        content += addition
    }
    if err := os.WriteFile("/etc/lnd/lnd.conf", []byte(content), 0640); err != nil {
        return err
    }
    return system.Run("chown", "root:"+systemUser, "/etc/lnd/lnd.conf")
}

func writeLITConfig(cfg *config.AppConfig, uiPassword string) error {
    network := cfg.Network
    if cfg.IsMainnet() {
        network = "mainnet"
    }
    macaroonPath := fmt.Sprintf(
        "/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon", network)
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

    if err := os.WriteFile("/etc/lit/lit.conf", []byte(content), 0640); err != nil {
        return err
    }
    return system.Run("chown", "root:"+systemUser, "/etc/lit/lit.conf")
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
    return os.WriteFile("/etc/systemd/system/litd.service", []byte(content), 0644)
}

func startLITD() error {
    if err := system.Run("systemctl", "daemon-reload"); err != nil {
        return err
    }
    if err := system.Run("systemctl", "enable", "litd"); err != nil {
        return err
    }
    return system.Run("systemctl", "start", "litd")
}