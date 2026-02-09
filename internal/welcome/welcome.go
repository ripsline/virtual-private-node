package welcome

import (
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
)

// Show prints the welcome message with network-appropriate commands.
func Show(cfg *config.AppConfig, version string) {
    fmt.Println()
    fmt.Println("  ╔══════════════════════════════════════════╗")
    fmt.Printf("  ║  Virtual Private Node v%-17s ║\n", version)
    fmt.Printf("  ║  Network: %-31s ║\n", cfg.Network)
    fmt.Println("  ╚══════════════════════════════════════════╝")
    fmt.Println()

    printServiceStatus(cfg)

    fmt.Println()
    fmt.Println("  ── Node Commands ──────────────────────────")
    fmt.Println()
    printBitcoinCommands()

    if cfg.HasLND() {
        fmt.Println()
        printLNDCommands()
    }

    fmt.Println()
    fmt.Println("  ── Service Management ─────────────────────")
    fmt.Println()
    fmt.Println("    sudo systemctl status bitcoind")
    fmt.Println("    sudo systemctl restart bitcoind")
    fmt.Println("    sudo journalctl -u bitcoind -f")

    if cfg.HasLND() {
        fmt.Println()
        fmt.Println("    sudo systemctl status lnd")
        fmt.Println("    sudo systemctl restart lnd")
        fmt.Println("    sudo journalctl -u lnd -f")
    }

    fmt.Println()
    fmt.Println("  ── Tor Hidden Services ────────────────────")
    fmt.Println()
    printOnionAddresses(cfg)

    if cfg.HasLND() {
        fmt.Println()
        fmt.Println("  ── Connect Wallets ────────────────────────")
        fmt.Println()
        printZeusInstructions(cfg)
        fmt.Println()
        printSparrowInstructions(cfg)
    } else {
        fmt.Println()
        fmt.Println("  ── Connect Sparrow Wallet ─────────────────")
        fmt.Println()
        printSparrowInstructions(cfg)
    }

    fmt.Println()
}

func printServiceStatus(cfg *config.AppConfig) {
    btcStatus := serviceStatus("bitcoind")
    fmt.Printf("    bitcoind:  %s\n", btcStatus)

    if cfg.HasLND() {
        lndStatus := serviceStatus("lnd")
        fmt.Printf("    lnd:       %s\n", lndStatus)
    }

    torStatus := serviceStatus("tor")
    fmt.Printf("    tor:       %s\n", torStatus)
}

func serviceStatus(name string) string {
    cmd := exec.Command("systemctl", "is-active", "--quiet", name)
    if cmd.Run() == nil {
        return "✓ running"
    }
    return "✗ stopped"
}

func printBitcoinCommands() {
    fmt.Println("    bitcoin-cli getblockchaininfo")
    fmt.Println("    bitcoin-cli getpeerinfo")
    fmt.Println("    bitcoin-cli getnetworkinfo")
}

func printLNDCommands() {
    fmt.Println("    lncli getinfo")
    fmt.Println("    lncli walletbalance")
    fmt.Println("    lncli channelbalance")
    fmt.Println("    lncli newaddress p2wkh")
    fmt.Println("    lncli listchannels")
    fmt.Println("    lncli addinvoice --amt=<sats> --memo=\"<memo>\"")
    fmt.Println("    lncli payinvoice <bolt11>")
}

func printOnionAddresses(cfg *config.AppConfig) {
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    btcP2P := readOnion("/var/lib/tor/bitcoin-p2p/hostname")

    if btcP2P != "" {
        fmt.Printf("    Bitcoin P2P:   %s\n", btcP2P)
    }
    if btcRPC != "" {
        fmt.Printf("    Bitcoin RPC:   %s\n", btcRPC)
    }

    if cfg.HasLND() {
        grpc := readOnion("/var/lib/tor/lnd-grpc/hostname")
        rest := readOnion("/var/lib/tor/lnd-rest/hostname")

        if grpc != "" {
            fmt.Printf("    LND gRPC:      %s:10009\n", grpc)
        }
        if rest != "" {
            fmt.Printf("    LND REST:      %s:8080\n", rest)
        }
    }
}

func printZeusInstructions(cfg *config.AppConfig) {
    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        fmt.Println("    Zeus: LND REST onion address not available yet.")
        fmt.Println("    Try again after Tor finishes starting.")
        return
    }

    macaroonHex := readMacaroonHex(cfg)

    fmt.Println("    Zeus Wallet (connect over Tor):")
    fmt.Println("    ─────────────────────────────────────")
    fmt.Println("    1. Install Zeus on your phone")
    fmt.Println("    2. Select Advanced Set-Up")
    fmt.Println("    3. Select Create or connect a wallet")
    fmt.Println("    4. In Wallet interface drowdown, select LND (REST):")
    fmt.Println()
    fmt.Printf("       Server address:     %s\n", restOnion)
    fmt.Println("      REST Port:     8080")

    if macaroonHex != "" {
        fmt.Println()
        fmt.Println("       Macaroon (Hex format):")
        // Print macaroon in chunks for readability
        for i := 0; i < len(macaroonHex); i += 76 {
            end := i + 76
            if end > len(macaroonHex) {
                end = len(macaroonHex)
            }
            fmt.Printf("       %s\n", macaroonHex[i:end])
        }
    } else {
        fmt.Println()
        fmt.Println("       Macaroon: not available yet (wallet not created?)")
        fmt.Println("       After wallet creation, find it with:")
        fmt.Println("       xxd -ps -c 1000 /var/lib/lnd/data/chain/bitcoin/*/admin.macaroon")
    }
}

func printSparrowInstructions(cfg *config.AppConfig) {
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC == "" {
        fmt.Println("    Sparrow: Bitcoin RPC onion address not available yet.")
        fmt.Println("    Try again after Tor finishes starting.")
        return
    }

    port := "8332"
    if !cfg.IsMainnet() {
        port = "48332"
    }

    fmt.Println("    Sparrow Wallet (connect over Tor):")
    fmt.Println("    ─────────────────────────────────────")
    fmt.Println("    1. In Sparrow: Settings → Server")
    fmt.Println("    2. Select 'Bitcoin Core' tab")
    fmt.Println("    3. Enter:")
    fmt.Println()
    fmt.Printf("       URL:      http://%s\n", btcRPC)
    fmt.Printf("       Port:     %s\n", port)
    fmt.Println("       Auth:     Cookie file (default)")
    fmt.Println()
    fmt.Println("    Sparrow has its own Tor proxy running locally.")
}

// readMacaroonHex reads the admin macaroon and returns it as hex.
func readMacaroonHex(cfg *config.AppConfig) string {
    network := cfg.Network
    if cfg.IsMainnet() {
        network = "mainnet"
    }

    path := fmt.Sprintf(
        "/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon",
        network,
    )

    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }

    return hex.EncodeToString(data)
}

func readOnion(path string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(data))
}