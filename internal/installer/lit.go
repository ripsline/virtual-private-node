package installer

import (
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
)

// downloadLIT fetches the Lightning Terminal tarball and manifest.
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
    // Manifest is best-effort
    download(manifestURL, "/tmp/lit-manifest.txt")
    return nil
}

// verifyLIT checks the manifest checksum if available.
func verifyLIT(version string) error {
    if _, err := os.Stat("/tmp/lit-manifest.txt"); err != nil {
        return nil // no manifest, skip
    }
    cmd := exec.Command("sha256sum", "--ignore-missing",
        "--check", "lit-manifest.txt")
    cmd.Dir = "/tmp"
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("checksum failed: %s: %s", err, output)
    }
    return nil
}

// extractAndInstallLIT extracts and installs the litd binary.
func extractAndInstallLIT(version string) error {
    filename := fmt.Sprintf("lightning-terminal-linux-amd64-v%s.tar.gz", version)

    cmd := exec.Command("tar", "-xzf", "/tmp/"+filename, "-C", "/tmp")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("extract: %s: %s", err, output)
    }

    extractDir := fmt.Sprintf("/tmp/lightning-terminal-linux-amd64-v%s", version)

    // Install litd binary
    src := extractDir + "/litd"
    cmd = exec.Command("install", "-m", "0755", "-o", "root", "-g", "root",
        src, "/usr/local/bin/")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install litd: %s: %s", err, output)
    }

    // Clean up
    os.Remove("/tmp/" + filename)
    os.Remove("/tmp/lit-manifest.txt")
    os.RemoveAll(extractDir)

    return nil
}

// enableRPCMiddleware adds rpcmiddleware.enable=true to lnd.conf
// if not already present. Required for LIT remote mode.
func enableRPCMiddleware() error {
    data, err := os.ReadFile("/etc/lnd/lnd.conf")
    if err != nil {
        return err
    }

    content := string(data)

    // Check if already enabled
    if strings.Contains(content, "rpcmiddleware.enable=true") {
        return nil
    }

    // Add to the Application Options section
    addition := "\n# Required for Lightning Terminal remote mode\nrpcmiddleware.enable=true\n"

    // Try to add after [Application Options]
    if idx := strings.Index(content, "[Application Options]"); idx != -1 {
        lineEnd := strings.Index(content[idx:], "\n")
        if lineEnd != -1 {
            insertAt := idx + lineEnd + 1
            content = content[:insertAt] + addition + content[insertAt:]
        }
    } else {
        // Append to end
        content += addition
    }

    if err := os.WriteFile("/etc/lnd/lnd.conf", []byte(content), 0640); err != nil {
        return err
    }

    cmd := exec.Command("chown", "root:"+systemUser, "/etc/lnd/lnd.conf")
    cmd.Run()

    return nil
}

// writeLITConfig creates the lit.conf configuration file for
// remote mode connection to our local LND instance.
func writeLITConfig(cfg *config.AppConfig, uiPassword string) error {
    network := cfg.Network
    if cfg.IsMainnet() {
        network = "mainnet"
    }

    macaroonPath := fmt.Sprintf(
        "/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon", network)

    content := fmt.Sprintf(`# Virtual Private Node — Lightning Terminal Configuration
#
# Remote mode: connects to our local LND instance
# Access via Tor Browser at the onion address shown in the dashboard

# UI password (auto-generated)
uipassword=%s

# LND remote mode
lnd-mode=remote
network=%s

# Connect to local LND
remote.lnd.rpcserver=localhost:10009
remote.lnd.macaroonpath=%s
remote.lnd.tlscertpath=/var/lib/lnd/tls.cert

# Disable sub-services (can be enabled later)
faraday-mode=disable
loop-mode=disable
pool-mode=disable
taproot-assets-mode=disable

# Listening
httpslisten=127.0.0.1:8443
`, uiPassword, cfg.Network, macaroonPath)

    // Create config directory
    os.MkdirAll("/etc/lit", 0750)

    if err := os.WriteFile("/etc/lit/lit.conf", []byte(content), 0640); err != nil {
        return err
    }

    cmd := exec.Command("chown", "root:"+systemUser, "/etc/lit/lit.conf")
    cmd.Run()

    return nil
}

// writeLITDService creates the systemd service file for litd.
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

// addLITTorService adds a Tor hidden service for the LIT web UI.
// Port 8443 stays on localhost, accessible only via Tor.
func addLITTorService() error {
    data, err := os.ReadFile("/etc/tor/torrc")
    if err != nil {
        return err
    }

    content := string(data)

    // Check if already configured
    if strings.Contains(content, "lnd-lit") {
        return nil
    }

    addition := `
# Lightning Terminal web UI (Tor only)
HiddenServiceDir /var/lib/tor/lnd-lit/
HiddenServicePort 8443 127.0.0.1:8443
`
    content += addition

    return os.WriteFile("/etc/tor/torrc", []byte(content), 0644)
}

// startLITD enables and starts the litd systemd service.
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