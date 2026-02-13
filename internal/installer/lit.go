package installer

import (
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
)

func downloadLIT(version string) error {
    filename := fmt.Sprintf("lightning-terminal-linux-amd64-v%s.tar.gz", version)
    url := fmt.Sprintf(
        "https://github.com/lightninglabs/lightning-terminal/releases/download/v%s/%s",
        version, filename)
    manifestURL := fmt.Sprintf(
        "https://github.com/lightninglabs/lightning-terminal/releases/download/v%s/manifest-v%s.txt",
        version, version)
    if err := download(url, "/tmp/"+filename); err != nil {
        return err
    }
    // Hard-fail if manifest download fails
    if err := download(manifestURL, "/tmp/lit-manifest.txt"); err != nil {
        return fmt.Errorf("download LIT manifest: %w", err)
    }
    return nil
}

func verifyLIT(version string) error {
    if _, err := os.Stat("/tmp/lit-manifest.txt"); err != nil {
        return fmt.Errorf("LIT manifest not found")
    }
    cmd := exec.Command("sha256sum", "--ignore-missing",
        "--check", "lit-manifest.txt")
    cmd.Dir = "/tmp"
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("checksum: %s: %s", err, output)
    }
    return nil
}

func extractAndInstallLIT(version string) error {
    filename := fmt.Sprintf("lightning-terminal-linux-amd64-v%s.tar.gz", version)
    cmd := exec.Command("tar", "-xzf", "/tmp/"+filename, "-C", "/tmp")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("extract: %s: %s", err, output)
    }
    extractDir := fmt.Sprintf("/tmp/lightning-terminal-linux-amd64-v%s", version)
    cmd = exec.Command("install", "-m", "0755", "-o", "root", "-g", "root",
        extractDir+"/litd", "/usr/local/bin/")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install: %s: %s", err, output)
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
        if output, err := exec.Command("chown", d.owner, d.path).CombinedOutput(); err != nil {
            return fmt.Errorf("chown %s: %s: %s", d.path, err, output)
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
    exec.Command("chown", "root:"+systemUser, "/etc/lnd/lnd.conf").Run()
    return nil
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
    exec.Command("chown", "root:"+systemUser, "/etc/lit/lit.conf").Run()
    return nil
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

func addLITTorService() error {
    data, err := os.ReadFile("/etc/tor/torrc")
    if err != nil {
        return err
    }
    if strings.Contains(string(data), "lnd-lit") {
        return nil
    }
    addition := `
# Lightning Terminal web UI (Tor only)
HiddenServiceDir /var/lib/tor/lnd-lit/
HiddenServicePort 8443 127.0.0.1:8443
`
    return os.WriteFile("/etc/tor/torrc", append(data, []byte(addition)...), 0644)
}

func startLITD() error {
    for _, args := range [][]string{
        {"systemctl", "daemon-reload"},
        {"systemctl", "enable", "litd"},
        {"systemctl", "start", "litd"},
    } {
        cmd := exec.Command(args[0], args[1:]...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("%v: %s: %s", args, err, output)
        }
    }
    return nil
}