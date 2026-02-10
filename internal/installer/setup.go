package installer

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
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
// Checks for the config file written at the end of installation.
func NeedsInstall() bool {
    _, err := os.Stat("/etc/rlvpn/config.json")
    return err != nil
}

// Run is the main installation entry point. It gathers config
// from the TUI, installs all components, creates the wallet,
// and launches the welcome TUI.
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
    // Shown in a centered TUI box to distinguish it from
    // the automated install steps above.
    if cfg.components == "bitcoin+lnd" {
        if err := walletCreationPhase(cfg); err != nil {
            return err
        }
    }

    // Configure shell environment so bitcoin-cli and lncli
    // work without long flags for the ripsline user
    fmt.Println("\n  Configuring shell environment...")
    if err := setupShellEnvironment(cfg); err != nil {
        fmt.Printf("  Warning: shell setup failed: %v\n", err)
    } else {
        fmt.Println("  ✓ Shell environment configured")
    }

    // Save persistent config — this file's existence is what
    // NeedsInstall() checks on subsequent logins
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

    // Show brief completion message then launch welcome TUI
    fmt.Println()
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println("    Installation Complete!")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println()
    fmt.Println("  Bitcoin Core is syncing. This takes a few hours.")
    fmt.Println("  Launching dashboard...")
    fmt.Println()

    return nil
}

// step is a named installation step with a function to execute.
type step struct {
    name string
    fn   func() error
}

// buildSteps returns the ordered list of installation steps
// based on the user's component choices.
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

// ── Centered TUI box ─────────────────────────────────────
//
// Used for wallet creation and auto-unlock prompts.
// Creates a mini bubbletea program that shows a centered
// bordered box with a message and waits for Enter.

// boxStyle for the centered information boxes
var setupBoxStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("245")).
    Padding(1, 3)

var setupTitleStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("15")).
    Bold(true)

var setupTextStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("250"))

var setupWarnStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("15")).
    Bold(true)

var setupDimStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("243"))

// infoBoxModel is a minimal bubbletea model that shows a
// centered box and waits for Enter.
type infoBoxModel struct {
    content string
    width   int
    height  int
    done    bool
}

func (m infoBoxModel) Init() tea.Cmd { return nil }

func (m infoBoxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        if msg.String() == "enter" || msg.String() == "ctrl+c" {
            m.done = true
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m infoBoxModel) View() string {
    if m.width == 0 {
        return "Loading..."
    }

    maxWidth := m.width - 8
    if maxWidth > 70 {
        maxWidth = 70
    }

    box := setupBoxStyle.Width(maxWidth).Render(m.content)

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        box,
    )
}

// showInfoBox displays a centered box with content and waits for Enter.
func showInfoBox(content string) {
    m := infoBoxModel{content: content}
    p := tea.NewProgram(m, tea.WithAltScreen())
    p.Run()
}

// walletCreationPhase handles the interactive wallet creation.
// Shows centered TUI boxes before and after handing control
// to lncli create.
func walletCreationPhase(cfg *installConfig) error {
    // Show info box explaining what's about to happen
    walletInfo := setupTitleStyle.Render("Create Your LND Wallet") + "\n\n" +
        setupTextStyle.Render("LND will ask you to:") + "\n\n" +
        setupTextStyle.Render("  1. Enter a wallet password (min 8 characters)") + "\n" +
        setupTextStyle.Render("  2. Confirm the password") + "\n" +
        setupTextStyle.Render("  3. 'n' to create a new seed") + "\n" +
        setupTextStyle.Render("  4. Optionally set a cipher seed passphrase") + "\n" +
        setupTextStyle.Render("     (press Enter to skip)") + "\n" +
        setupTextStyle.Render("  5. Write down your 24-word seed phrase") + "\n\n" +
        setupWarnStyle.Render("⚠️  Your seed phrase is the ONLY way to recover funds.") + "\n" +
        setupWarnStyle.Render("⚠️  No one can help you if you lose it.") + "\n\n" +
        setupDimStyle.Render("Press Enter to continue...")

    showInfoBox(walletInfo)

    // Wait for LND REST to be ready
    fmt.Println()
    fmt.Println("  Waiting for LND to be ready...")
    if err := waitForLND(); err != nil {
        return fmt.Errorf("LND not ready: %w", err)
    }
    fmt.Println("  ✓ LND is ready")
    fmt.Println()

    // Hand terminal to lncli create — this is LND's native
    // interactive wallet creation. We don't customize it.
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

    // Show auto-unlock box — we default to yes, just need the password
    unlockInfo := setupTitleStyle.Render("Auto-Unlock Configuration") + "\n\n" +
        setupTextStyle.Render("Your wallet password will be stored on disk so LND") + "\n" +
        setupTextStyle.Render("can start automatically after a server reboot.") + "\n\n" +
        setupTextStyle.Render("Without auto-unlock, you would need to SSH in and") + "\n" +
        setupTextStyle.Render("manually unlock the wallet after every restart.") + "\n\n" +
        setupDimStyle.Render("Press Enter to continue...")

    showInfoBox(unlockInfo)

    // Prompt for password (outside of bubbletea, raw terminal)
    fmt.Println()
    fmt.Print("  Re-enter your wallet password for auto-unlock: ")
    password := readPassword()
    fmt.Println()

    if password == "" {
        fmt.Println("  No password entered. Skipping auto-unlock.")
        fmt.Println("  You can set this up later by creating /var/lib/lnd/wallet_password")
        return nil
    }

    if err := setupAutoUnlock(password); err != nil {
        fmt.Printf("  Warning: auto-unlock setup failed: %v\n", err)
        fmt.Println("  You can set this up manually later.")
    } else {
        fmt.Println("  ✓ Auto-unlock configured")
    }

    return nil
}

// readLine reads a single line from the reader and trims whitespace.
func readLine(reader *bufio.Reader) string {
    line, _ := reader.ReadString('\n')
    return strings.TrimSpace(line)
}

// readPassword reads a password from stdin with echo disabled
// so the password is not visible as the user types.
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

// detectPublicIP tries to determine the server's public IPv4
// address. Returns empty string if detection fails.
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

// readFileOrDefault reads a file or returns a default value if
// the file doesn't exist or can't be read.
func readFileOrDefault(path, def string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return def
    }
    return string(data)
}

// setupShellEnvironment configures ripsline's shell so that
// bitcoin-cli and lncli work without long path flags or sudo.
// Uses bash functions that wrap the real binaries with the
// correct flags. lncli also gets env vars it reads natively.
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
# Added by rlvpn installer. Do not edit above this line.

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

    // Append to ripsline's .bashrc so it loads on every shell session
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