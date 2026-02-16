package installer

import (
    "fmt"
    "os"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"
)

func installTor() error {
    return system.Run("apt-get", "install", "-y", "-qq", "tor")
}

// RebuildTorConfig generates the complete torrc from current AppConfig state.
// Called on initial install and every time an add-on is installed.
func RebuildTorConfig(cfg *config.AppConfig) error {
    net := cfg.NetworkConfig()

    var b strings.Builder
    b.WriteString("# Virtual Private Node — Tor Configuration\n")
    b.WriteString("SOCKSPort 9050\n")

    // Control port needed for LND
    if cfg.HasLND() {
        b.WriteString("\n# Control port for LND P2P onion management\n")
        b.WriteString("ControlPort 9051\n")
        b.WriteString("CookieAuthentication 1\n")
        b.WriteString("CookieAuthFileGroupReadable 1\n")
    }

    // Bitcoin hidden services — always
    b.WriteString(fmt.Sprintf(`
# Bitcoin Core RPC (for wallet connections like Sparrow)
HiddenServiceDir /var/lib/tor/bitcoin-rpc/
HiddenServicePort %d 127.0.0.1:%d

# Bitcoin Core P2P (static onion address for peers)
HiddenServiceDir /var/lib/tor/bitcoin-p2p/
HiddenServicePort %d 127.0.0.1:%d
`, net.RPCPort, net.RPCPort, net.P2PPort, net.P2PPort))

    // LND hidden services
    if cfg.HasLND() {
        b.WriteString(`
# LND gRPC (wallet connections over Tor)
HiddenServiceDir /var/lib/tor/lnd-grpc/
HiddenServicePort 10009 127.0.0.1:10009

# LND REST (wallet connections over Tor)
HiddenServiceDir /var/lib/tor/lnd-rest/
HiddenServicePort 8080 127.0.0.1:8080
`)
    }

    // LIT hidden service
    if cfg.LITInstalled {
        b.WriteString(`
# Lightning Terminal web UI (Tor only)
HiddenServiceDir /var/lib/tor/lnd-lit/
HiddenServicePort 8443 127.0.0.1:8443
`)
    }

    // Syncthing hidden services
    if cfg.SyncthingInstalled {
        b.WriteString(`
# Syncthing web UI (Tor only, HTTP)
HiddenServiceDir /var/lib/tor/syncthing/
HiddenServicePort 8384 127.0.0.1:8384

# Syncthing sync protocol (Tor only)
HiddenServiceDir /var/lib/tor/syncthing-sync/
HiddenServicePort 22000 127.0.0.1:22000
`)
    }

    return os.WriteFile("/etc/tor/torrc", []byte(b.String()), 0644)
}

func addUserToTorGroup(username string) error {
    return system.Run("usermod", "-aG", "debian-tor", username)
}

func restartTor() error {
    if err := system.Run("systemctl", "enable", "tor"); err != nil {
        return err
    }
    return system.Run("systemctl", "restart", "tor")
}