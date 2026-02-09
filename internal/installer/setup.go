package installer

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
)

const (
    bitcoinVersion = "29.2"
    lndVersion     = "0.20.0-beta"
    systemUser     = "bitcoin"
)

// installConfig holds all choices made during setup.
type installConfig struct {
    network    *NetworkConfig
    components string // "bitcoin" or "bitcoin+lnd"
    pruneSize  int    // GB
    p2pMode    string // "tor" or "hybrid"
    publicIPv4 string
    sshPort    int
}

// NeedsInstall returns true if the node has not been set up yet.
func NeedsInstall() bool {
    _, err := os.Stat("/etc/rlvpn/config.json")
    return err != nil
}

// Run is the main installation entry point.
func Run() error {
    if err := checkOS(); err != nil {
        return err
    }

    // Launch the TUI to gather configuration
    cfg, err := RunTUI()
    if err != nil {
        return err
    }
    if cfg == nil {
        fmt.Println("\n  Installation cancelled.")
        return nil
    }

    // If hybrid mode, we need the public IP
    if cfg.p2pMode == "hybrid" {
        reader := bufio.NewReader(os.Stdin)
        cfg.publicIPv4 = detectPublicIP()
        if cfg.publicIPv4 != "" {
            fmt.Printf("\n  Detected public IPv4: %s\n", cfg.publicIPv4)
            fmt.Print("  Use this IP? [Y/n]: ")
            confirm := readLine(reader)
            if strings.ToLower(confirm) == "n" {
                fmt.Print("  Enter IPv4 manually: ")
                cfg.publicIPv4 = readLine(reader)
            }
        } else {
            fmt.Print("\n  Could not detect public IP. Enter IPv4: ")
            cfg.publicIPv4 = readLine(reader)
        }
        if cfg.publicIPv4 == "" {
            fmt.Println("  No IP entered — defaulting to Tor only.")
            cfg.p2pMode = "tor"
        }
    }

    // Build and run installation steps
    steps := buildSteps(cfg)
    total := len(steps)

    fmt.Println()
    for i, step := range steps {
        fmt.Printf("  [%d/%d] %s...\n", i+1, total, step.name)
        if err := step.fn(); err != nil {
            return fmt.Errorf("%s failed: %w", step.name, err)
        }
        fmt.Printf("  ✓ %s\n", step.name)
    }

    // LND wallet creation — separate interactive phase
    if cfg.components == "bitcoin+lnd" {
        reader := bufio.NewReader(os.Stdin)
        if err := walletCreationPhase(cfg, reader); err != nil {
            return err
        }
    }

    // Configure shell environment
    fmt.Println("\n  Configuring shell environment...")
    if err := setupShellEnvironment(cfg); err != nil {
        fmt.Printf("  Warning: shell setup failed: %v\n", err)
    } else {
        fmt.Println("  ✓ Shell environment configured")
    }

    // Save persistent config
    appCfg := &config.AppConfig{
        Network:    cfg.network.Name,
        Components: cfg.components,
        PruneSize:  cfg.pruneSize,
        P2PMode:    cfg.p2pMode,
        SSHPort:    cfg.sshPort,
    }
    if err := config.Save(appCfg); err != nil {
        return fmt.Errorf("save config: %w", err)
    }

    printComplete(cfg)
    return nil
}

type step struct {
    name string
    fn   func() error
}

func buildSteps(cfg *installConfig) []step {
    steps := []step{
        {"Creating system user", func() error { return createSystemUser(systemUser) }},
        {"Creating directories", func() error { return createDirs(systemUser, cfg) }},
        {"Disabling IPv6", disableIPv6},
        {"Configuring firewall", func() error { return configureFirewall(cfg) }},
        {"Installing Tor", installTor},
        {"Configuring Tor", func() error { return writeTorConfig(cfg) }},
        {"Adding user to debian-tor group", func() error { return addUserToTorGroup(systemUser) }},
        {"Starting Tor", restartTor},
        {"Installing Bitcoin Core " + bitcoinVersion, func() error { return installBitcoin(bitcoinVersion) }},
        {"Configuring Bitcoin Core", func() error { return writeBitcoinConfig(cfg) }},
        {"Creating bitcoind service", func() error { return writeBitcoindService(systemUser) }},
        {"Starting Bitcoin Core", startBitcoind},
    }

    if cfg.components == "bitcoin+lnd" {
        steps = append(steps,
            step{"Installing LND " + lndVersion, func() error { return installLND(lndVersion) }},
            step{"Configuring LND", func() error { return writeLNDConfig(cfg) }},
            step{"Creating LND service", func() error { return writeLNDServiceInitial(systemUser) }},
            step{"Starting LND", startLND},
        )
    }

    return steps
}

func walletCreationPhase(cfg *installConfig, reader *bufio.Reader) error {
    fmt.Println()
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println()
    fmt.Println("  Automated setup complete.")
    fmt.Println()
    fmt.Println("  Next: Create your LND wallet.")
    fmt.Println()
    fmt.Println("  LND will ask you to:")
    fmt.Println("    1. Enter a wallet password (min 8 characters)")
    fmt.Println("    2. Confirm the password")
	fmt.Println("    3. 'n' to create a new seed")
    fmt.Println("    4. Optionally set a cipher seed passphrase")
    fmt.Println("       (press Enter to skip)")
    fmt.Println("    5. Write down your 24-word seed phrase")
    fmt.Println()
    fmt.Println("  ⚠️  Your seed phrase is the ONLY way to recover funds.")
    fmt.Println("  ⚠️  No one can help you if you lose it.")
    fmt.Println()
    fmt.Print("  Press Enter to continue...")
    reader.ReadString('\n')

    fmt.Println()
    fmt.Println("  Waiting for LND to be ready...")
    if err := waitForLND(); err != nil {
        return fmt.Errorf("LND not ready: %w", err)
    }
    fmt.Println("  ✓ LND is ready")
    fmt.Println()

    // Hand terminal to lncli create
    lncliArgs := []string{
        "-u", systemUser, "lncli",
        "--lnddir=/var/lib/lnd",
        "--network=" + cfg.network.LNCLINetwork,
        "create",
    }
    cmd := exec.Command("sudo", lncliArgs...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("lncli create failed: %w", err)
    }

    fmt.Println()
    fmt.Println("  ✓ Wallet created")

    // Auto-unlock option
    fmt.Println()
    fmt.Println("  ? Auto-unlock LND wallet on reboot?")
    fmt.Println("    This stores your wallet password on disk so LND")
    fmt.Println("    can start without manual intervention after reboot.")
    fmt.Println()
    fmt.Println("    1) Yes (recommended for always-on nodes)")
    fmt.Println("    2) No (you must unlock manually after every restart)")
    fmt.Print("    Select [1/2]: ")
    choice := readLine(reader)

    if choice != "2" {
        fmt.Println()
        fmt.Print("  ? Re-enter your wallet password for auto-unlock: ")
        password := readPassword()
        fmt.Println()

        if err := setupAutoUnlock(password); err != nil {
            fmt.Printf("  Warning: auto-unlock setup failed: %v\n", err)
            fmt.Println("  You can set this up manually later.")
        } else {
            fmt.Println("  ✓ Auto-unlock configured")
        }
    }

    return nil
}

func printComplete(cfg *installConfig) {
    fmt.Println()
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println("    Installation Complete!")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println()
    fmt.Println("  Bitcoin Core is syncing. This takes a few hours.")
    fmt.Println()

    btcRPCOnion := readFileOrDefault("/var/lib/tor/bitcoin-rpc/hostname", "")
    btcP2POnion := readFileOrDefault("/var/lib/tor/bitcoin-p2p/hostname", "")

    if btcP2POnion != "" {
        fmt.Printf("  Bitcoin P2P:   %s:%d\n",
            strings.TrimSpace(btcP2POnion), cfg.network.P2PPort)
    }
    if btcRPCOnion != "" {
        fmt.Printf("  Bitcoin RPC:   %s:%d\n",
            strings.TrimSpace(btcRPCOnion), cfg.network.RPCPort)
    }

    if cfg.components == "bitcoin+lnd" {
        grpcOnion := readFileOrDefault("/var/lib/tor/lnd-grpc/hostname", "")
        restOnion := readFileOrDefault("/var/lib/tor/lnd-rest/hostname", "")
        if grpcOnion != "" {
            fmt.Printf("  LND gRPC:      %s:10009\n", strings.TrimSpace(grpcOnion))
        }
        if restOnion != "" {
            fmt.Printf("  LND REST:      %s:8080\n", strings.TrimSpace(restOnion))
        }
    }

    fmt.Println()
    fmt.Println("  Log out and SSH back in to see the welcome message")
    fmt.Println("  with all available commands.")
    fmt.Println()
}

func readLine(reader *bufio.Reader) string {
    line, _ := reader.ReadString('\n')
    return strings.TrimSpace(line)
}

func readPassword() string {
    sttyOff := exec.Command("stty", "-echo")
    sttyOff.Stdin = os.Stdin
    sttyOff.Run()

    reader := bufio.NewReader(os.Stdin)
    password, _ := reader.ReadString('\n')

    sttyOn := exec.Command("stty", "echo")
    sttyOn.Stdin = os.Stdin
    sttyOn.Run()

    return strings.TrimSpace(password)
}

func detectPublicIP() string {
    cmd := exec.Command("curl", "-4", "-s", "--max-time", "5", "ifconfig.me")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    ip := strings.TrimSpace(string(output))
    parts := strings.Split(ip, ".")
    if len(parts) != 4 {
        return ""
    }
    return ip
}

func readFileOrDefault(path, def string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return def
    }
    return string(data)
}

// setupShellEnvironment configures ripsline's shell so that
// bitcoin-cli and lncli work without path flags or sudo prefix.
func setupShellEnvironment(cfg *installConfig) error {
    networkFlag := ""
    if cfg.network.Name != "mainnet" {
        networkFlag = fmt.Sprintf(
            "\nexport LNCLI_NETWORK=%s", cfg.network.LNCLINetwork,
        )
    }

    lndBlock := ""
    if cfg.components == "bitcoin+lnd" {
        lndBlock = fmt.Sprintf(`
# LND — lncli reads these env vars natively
export LNCLI_LNDDIR=/var/lib/lnd%s
export LNCLI_MACAROONPATH=/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon
export LNCLI_TLSCERTPATH=/var/lib/lnd/tls.cert
`, networkFlag, cfg.network.LNCLINetwork)
    }

    content := fmt.Sprintf(`
# ── Virtual Private Node ──────────────────────
# Added by rlvpn installer

# Bitcoin Core — wrapper so bitcoin-cli just works
bitcoin-cli() {
    sudo -u bitcoin /usr/local/bin/bitcoin-cli \
        -datadir=/var/lib/bitcoin \
        -conf=/etc/bitcoin/bitcoin.conf \
        "$@"
}
export -f bitcoin-cli
%s
# LND — wrapper for sudo
lncli() {
    sudo -u bitcoin /usr/local/bin/lncli "$@"
}
export -f lncli
`, lndBlock)

    f, err := os.OpenFile("/home/ripsline/.bashrc",
        os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        return fmt.Errorf("open .bashrc: %w", err)
    }
    defer f.Close()

    if _, err := f.WriteString(content); err != nil {
        return fmt.Errorf("write .bashrc: %w", err)
    }

    return nil
}