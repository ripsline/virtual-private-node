package installer

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "golang.org/x/term"

    "github.com/ripsline/virtual-private-node/internal/config"
)

const (
    bitcoinVersion = "29.3"
    lndVersion     = "0.20.0-beta"
    litVersion     = "0.16.0-alpha"
    systemUser     = "bitcoin"
    appVersion     = "0.1.0"
)

func LitVersionStr() string { return litVersion }

type installConfig struct {
    network    *NetworkConfig
    components string
    pruneSize  int
    p2pMode    string
    publicIPv4 string
}

func NeedsInstall() bool {
    _, err := os.Stat("/etc/rlvpn/config.json")
    return err != nil
}

// ── Install progress TUI ─────────────────────────────────

type stepStatus int

const (
    stepPending stepStatus = iota
    stepRunning
    stepDone
    stepFailed
)

type installStep struct {
    name   string
    fn     func() error
    status stepStatus
    err    error
}

type stepDoneMsg struct{ index int; err error }

type installModel struct {
    steps         []installStep
    current       int
    done, failed  bool
    version       string
    width, height int
}

var (
    progTitleStyle = lipgloss.NewStyle().Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).Padding(0, 2)
    progBoxStyle = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245")).Padding(1, 2)
    progDoneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
    progRunStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
    progPendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
    progFailStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
    progDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
    progGoodStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
)

func (m installModel) Init() tea.Cmd { return m.runStep(0) }

func (m installModel) runStep(i int) tea.Cmd {
    return func() tea.Msg {
        if i >= len(m.steps) {
            return stepDoneMsg{index: i}
        }
        return stepDoneMsg{index: i, err: m.steps[i].fn()}
    }
}

func (m installModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        if msg.String() == "enter" && m.done {
            return m, tea.Quit
        }
        if msg.String() == "ctrl+c" {
            return m, tea.Quit
        }
    case stepDoneMsg:
        if msg.index < len(m.steps) {
            if msg.err != nil {
                m.steps[msg.index].status = stepFailed
                m.steps[msg.index].err = msg.err
                m.failed = true
                m.done = true
                return m, nil
            }
            m.steps[msg.index].status = stepDone
            next := msg.index + 1
            if next < len(m.steps) {
                m.current = next
                m.steps[next].status = stepRunning
                return m, m.runStep(next)
            }
            m.done = true
        }
    }
    return m, nil
}

func (m installModel) View() string {
    if m.width == 0 {
        return "Loading..."
    }
    bw := min(m.width-4, 76)
    title := progTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(fmt.Sprintf(" Virtual Private Node v%s ", m.version))
    var lines []string
    for i, s := range m.steps {
        var sty lipgloss.Style
        var ind string
        switch s.status {
        case stepDone:
            sty, ind = progDoneStyle, "✓"
        case stepRunning:
            sty, ind = progRunStyle, "⟳"
        case stepFailed:
            sty, ind = progFailStyle, "✗"
        default:
            sty, ind = progPendingStyle, "○"
        }
        lines = append(lines, sty.Render(fmt.Sprintf("  %s [%d/%d] %s",
            ind, i+1, len(m.steps), s.name)))
        if s.status == stepFailed && s.err != nil {
            lines = append(lines, progFailStyle.Render(
                fmt.Sprintf("      Error: %v", s.err)))
        }
    }
    box := progBoxStyle.Width(bw).Render(strings.Join(lines, "\n"))
    var footer string
    if m.done && !m.failed {
        footer = progGoodStyle.Render("  ✓ Complete — press Enter to continue  ")
    } else if m.failed {
        footer = progFailStyle.Render("  Failed. Press ctrl+c to exit.  ")
    } else {
        footer = progDimStyle.Render("  Installing... please wait  ")
    }
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}

func runInstallTUI(steps []installStep, version string) error {
    if len(steps) == 0 {
        return nil
    }
    steps[0].status = stepRunning
    m := installModel{steps: steps, version: version}
    p := tea.NewProgram(m, tea.WithAltScreen())
    result, err := p.Run()
    if err != nil {
        return err
    }
    final := result.(installModel)
    if final.failed {
        for _, s := range final.steps {
            if s.status == stepFailed {
                return fmt.Errorf("%s: %w", s.name, s.err)
            }
        }
    }
    return nil
}

// ── Info and Confirm boxes ───────────────────────────────

var (
    setupBoxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("245")).Padding(1, 3)
    setupTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
    setupTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
    setupWarnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
    setupDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

type infoBoxModel struct {
    content       string
    width, height int
}

func (m infoBoxModel) Init() tea.Cmd { return nil }
func (m infoBoxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        if msg.String() == "enter" || msg.String() == "ctrl+c" {
            return m, tea.Quit
        }
    }
    return m, nil
}
func (m infoBoxModel) View() string {
    if m.width == 0 {
        return "Loading..."
    }
    box := setupBoxStyle.Width(min(m.width-8, 70)).Render(m.content)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
func showInfoBox(content string) {
    p := tea.NewProgram(infoBoxModel{content: content}, tea.WithAltScreen())
    p.Run()
}

// confirmBoxModel distinguishes Enter (proceed) from backspace (cancel)
type confirmBoxModel struct {
    content       string
    confirmed     bool
    width, height int
}

func (m confirmBoxModel) Init() tea.Cmd { return nil }
func (m confirmBoxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        switch msg.String() {
        case "enter":
            m.confirmed = true
            return m, tea.Quit
        case "backspace", "ctrl+c", "q":
            m.confirmed = false
            return m, tea.Quit
        }
    }
    return m, nil
}
func (m confirmBoxModel) View() string {
    if m.width == 0 {
        return "Loading..."
    }
    box := setupBoxStyle.Width(min(m.width-8, 70)).Render(m.content)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
func showConfirmBox(content string) bool {
    m := confirmBoxModel{content: content}
    p := tea.NewProgram(m, tea.WithAltScreen())
    result, _ := p.Run()
    return result.(confirmBoxModel).confirmed
}

// ── Main install flow ────────────────────────────────────

func Run() error {
    if err := checkOS(); err != nil {
        return err
    }
    cfg, err := RunTUI(appVersion)
    if err != nil {
        return err
    }
    if cfg == nil {
        return nil
    }
    steps := buildSteps(cfg)
    if err := runInstallTUI(steps, appVersion); err != nil {
        return err
    }
    if err := setupShellEnvironment(cfg); err != nil {
        fmt.Printf("  Warning: shell setup failed: %v\n", err)
    }
    appCfg := &config.AppConfig{
        Network: cfg.network.Name, Components: cfg.components,
        PruneSize: cfg.pruneSize, P2PMode: cfg.p2pMode,
    }
    return config.Save(appCfg)
}

func buildSteps(cfg *installConfig) []installStep {
    steps := []installStep{
        {name: "Creating system user", fn: func() error { return createSystemUser(systemUser) }},
        {name: "Creating directories", fn: func() error { return createDirs(systemUser, cfg) }},
        {name: "Disabling IPv6", fn: disableIPv6},
        {name: "Configuring firewall", fn: func() error { return configureFirewall(cfg) }},
        {name: "Installing GPG", fn: ensureGPG},
        {name: "Importing Bitcoin Core signing keys", fn: importBitcoinCoreKeys},
        {name: "Installing Tor", fn: installTor},
        {name: "Configuring Tor", fn: func() error { return writeTorConfig(cfg) }},
        {name: "Adding user to debian-tor group", fn: func() error { return addUserToTorGroup(systemUser) }},
        {name: "Starting Tor", fn: restartTor},
        {name: "Downloading Bitcoin Core " + bitcoinVersion, fn: func() error { return downloadBitcoin(bitcoinVersion) }},
        {name: "Downloading Bitcoin Core signatures", fn: func() error { return downloadBitcoinSigFile(bitcoinVersion) }},
        {name: "Verifying Bitcoin Core signatures (2/5)", fn: func() error { return verifyBitcoinCoreSigs(2) }},
        {name: "Verifying Bitcoin Core checksum", fn: func() error { return verifyBitcoin(bitcoinVersion) }},
        {name: "Installing Bitcoin Core", fn: func() error { return extractAndInstallBitcoin(bitcoinVersion) }},
        {name: "Configuring Bitcoin Core", fn: func() error { return writeBitcoinConfig(cfg) }},
        {name: "Creating bitcoind service", fn: func() error { return writeBitcoindService(systemUser) }},
        {name: "Starting Bitcoin Core", fn: startBitcoind},
        {name: "Installing unattended-upgrades", fn: installUnattendedUpgrades},
        {name: "Configuring auto-security-updates", fn: configureUnattendedUpgrades},
        {name: "Installing fail2ban", fn: installFail2ban},
        {name: "Configuring fail2ban", fn: configureFail2ban},
    }
    if cfg.components == "bitcoin+lnd" {
        steps = append(steps,
            installStep{name: "Importing LND signing key", fn: importLNDKey},
            installStep{name: "Downloading LND " + lndVersion, fn: func() error { return downloadLND(lndVersion) }},
            installStep{name: "Verifying LND signature", fn: func() error { return verifyLNDSig(lndVersion) }},
            installStep{name: "Verifying LND checksum", fn: func() error { return verifyLND(lndVersion) }},
            installStep{name: "Installing LND", fn: func() error { return extractAndInstallLND(lndVersion) }},
            installStep{name: "Configuring LND", fn: func() error { return writeLNDConfig(cfg) }},
            installStep{name: "Creating LND service", fn: func() error { return writeLNDServiceInitial(systemUser) }},
            installStep{name: "Starting LND", fn: startLND},
        )
    }
    return steps
}

// ── Wallet creation ──────────────────────────────────────

func RunWalletCreation(networkName string) error {
    net := NetworkConfigFromName(networkName)
    info := setupTitleStyle.Render("Create Your LND Wallet") + "\n\n" +
        setupTextStyle.Render("LND will ask you to:") + "\n\n" +
        setupTextStyle.Render("  1. Enter a wallet password (min 8 characters)") + "\n" +
        setupTextStyle.Render("  2. Confirm the password") + "\n" +
        setupTextStyle.Render("  3. 'n' to create a new seed") + "\n" +
        setupTextStyle.Render("  4. Optionally set a cipher seed passphrase") + "\n" +
        setupTextStyle.Render("     (press Enter to skip)") + "\n" +
        setupTextStyle.Render("  5. Write down your 24-word seed phrase") + "\n\n" +
        setupWarnStyle.Render("WARNING: Your seed is the ONLY way to recover funds.") + "\n" +
        setupWarnStyle.Render("WARNING: No one can help you if you lose it.") + "\n\n" +
        setupDimStyle.Render("Enter to proceed • backspace to cancel")
    if !showConfirmBox(info) {
        return nil
    }

    fmt.Print("\033[2J\033[H")
    fmt.Println("\n  ═══════════════════════════════════════════")
    fmt.Println("    LND Wallet Creation")
    fmt.Println("  ═══════════════════════════════════════════\n")
    fmt.Println("  Waiting for LND...")
    if err := waitForLND(); err != nil {
        return err
    }
    fmt.Println("  ✓ LND is ready\n")

    cmd := exec.Command("sudo", "-u", systemUser, "lncli",
        "--lnddir=/var/lib/lnd", "--network="+net.LNCLINetwork, "create")
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("lncli create: %w", err)
    }

    seedMsg := setupTitleStyle.Render("Seed Phrase Confirmation") + "\n\n" +
        setupWarnStyle.Render("Have you written down your 24-word seed phrase?") + "\n\n" +
        setupTextStyle.Render("Scroll up in your terminal to see your seed.") + "\n" +
        setupTextStyle.Render("Once you close this session, it will no longer") + "\n" +
        setupTextStyle.Render("be visible anywhere.") + "\n\n" +
        setupTextStyle.Render("Make sure you saved it in a secure location.") + "\n\n" +
        setupDimStyle.Render("Press Enter to confirm...")
    showInfoBox(seedMsg)

    unlockMsg := setupTitleStyle.Render("Auto-Unlock Configuration") + "\n\n" +
        setupTextStyle.Render("Your wallet password will be stored so LND") + "\n" +
        setupTextStyle.Render("starts automatically after reboot.") + "\n\n" +
        setupDimStyle.Render("Press Enter to continue...")
    showInfoBox(unlockMsg)

    fmt.Print("\033[2J\033[H")
    fmt.Println("\n  ═══════════════════════════════════════════")
    fmt.Println("    Auto-Unlock Password")
    fmt.Println("  ═══════════════════════════════════════════\n")
    fmt.Print("  Re-enter your wallet password: ")
    pw := readPassword()
    fmt.Println()

    if pw != "" {
        if err := setupAutoUnlock(pw); err != nil {
            fmt.Printf("  Warning: %v\n", err)
        } else {
            fmt.Println("  ✓ Auto-unlock configured")
        }
        appCfg, err := config.Load()
        if err == nil {
            appCfg.AutoUnlock = true
            config.Save(appCfg)
        }
    }
    return nil
}

// ── LIT installation ─────────────────────────────────────

func RunLITInstall(cfg *config.AppConfig) error {
    confirmMsg := setupTitleStyle.Render("Install Lightning Terminal") + "\n\n" +
        setupTextStyle.Render("This will:") + "\n\n" +
        setupTextStyle.Render("  • Download Lightning Terminal v" + litVersion) + "\n" +
        setupTextStyle.Render("  • Modify LND config (enable rpcmiddleware)") + "\n" +
        setupTextStyle.Render("  • Restart LND") + "\n" +
        setupTextStyle.Render("  • Create Tor hidden service for LIT web UI") + "\n" +
        setupTextStyle.Render("  • Restart Tor") + "\n\n" +
        setupDimStyle.Render("Enter to proceed • backspace to cancel")
    if !showConfirmBox(confirmMsg) {
        return nil
    }

    passBytes := make([]byte, 12)
    if _, err := rand.Read(passBytes); err != nil {
        return fmt.Errorf("generate password: %w", err)
    }
    litPassword := hex.EncodeToString(passBytes)

    steps := []installStep{
        {name: "Importing LIT signing key", fn: importLITKey},
        {name: "Downloading Lightning Terminal " + litVersion,
            fn: func() error { return downloadLIT(litVersion) }},
        {name: "Verifying LIT signature",
            fn: func() error { return verifyLITSig(litVersion) }},
        {name: "Verifying LIT checksum",
            fn: func() error { return verifyLIT(litVersion) }},
        {name: "Installing Lightning Terminal",
            fn: func() error { return extractAndInstallLIT(litVersion) }},
        {name: "Enabling RPC middleware in LND", fn: enableRPCMiddleware},
        {name: "Restarting LND",
            fn: func() error { return exec.Command("systemctl", "restart", "lnd").Run() }},
        {name: "Creating LIT directories", fn: createLITDirs},
        {name: "Creating LIT configuration",
            fn: func() error { return writeLITConfig(cfg, litPassword) }},
        {name: "Creating litd service",
            fn: func() error { return writeLITDService(systemUser) }},
        {name: "Configuring Tor for LIT", fn: addLITTorService},
        {name: "Restarting Tor", fn: restartTor},
        {name: "Starting Lightning Terminal", fn: startLITD},
    }
    if err := runInstallTUI(steps, appVersion); err != nil {
        return err
    }
    cfg.LITInstalled = true
    cfg.LITPassword = litPassword
    return config.Save(cfg)
}

// ── Syncthing installation ───────────────────────────────

func RunSyncthingInstall(cfg *config.AppConfig) error {
    confirmMsg := setupTitleStyle.Render("Install Syncthing") + "\n\n" +
        setupTextStyle.Render("This will:") + "\n\n" +
        setupTextStyle.Render("  • Install Syncthing from official repository") + "\n" +
        setupTextStyle.Render("  • Create Tor hidden service for web UI") + "\n" +
        setupTextStyle.Render("  • Auto-configure LND channel backup sync") + "\n" +
        setupTextStyle.Render("  • Restart Tor") + "\n\n" +
        setupDimStyle.Render("Enter to proceed • backspace to cancel")
    if !showConfirmBox(confirmMsg) {
        return nil
    }

    passBytes := make([]byte, 12)
    if _, err := rand.Read(passBytes); err != nil {
        return fmt.Errorf("generate password: %w", err)
    }
    syncPassword := hex.EncodeToString(passBytes)

    steps := []installStep{
        {name: "Adding Syncthing repository", fn: installSyncthingRepo},
        {name: "Installing Syncthing", fn: installSyncthingPackage},
        {name: "Creating Syncthing directories", fn: createSyncthingDirs},
        {name: "Creating Syncthing service", fn: writeSyncthingService},
        {name: "Configuring Syncthing authentication",
            fn: func() error { return configureSyncthingAuth(syncPassword) }},
        {name: "Configuring Tor for Syncthing", fn: addSyncthingTorService},
        {name: "Restarting Tor", fn: restartTor},
        {name: "Starting Syncthing", fn: startSyncthing},
        {name: "Setting up channel backup watcher",
            fn: func() error { return setupChannelBackupWatcher(cfg) }},
    }
    if err := runInstallTUI(steps, appVersion); err != nil {
        return err
    }
    cfg.SyncthingInstalled = true
    cfg.SyncthingPassword = syncPassword
    return config.Save(cfg)
}

// ── Helpers ──────────────────────────────────────────────

// readPassword uses golang.org/x/term for robust password input.
// Terminal echo is always restored even if the process crashes.
func readPassword() string {
    pw, err := term.ReadPassword(int(os.Stdin.Fd()))
    if err != nil {
        return ""
    }
    return string(pw)
}

func detectPublicIP() string {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, "curl", "-4", "-s", "--max-time", "5", "ifconfig.me")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    ip := strings.TrimSpace(string(output))
    if len(strings.Split(ip, ".")) != 4 {
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

func setupShellEnvironment(cfg *installConfig) error {
    btcNetFlag := ""
    if cfg.network.Name == "testnet4" {
        btcNetFlag = "\n        -testnet4 \\"
    }

    lndBlock := ""
    if cfg.components == "bitcoin+lnd" {
        lndNetFlag := ""
        if cfg.network.Name != "mainnet" {
            lndNetFlag = fmt.Sprintf("\n        --network=%s \\",
                cfg.network.LNCLINetwork)
        }
        lndBlock = fmt.Sprintf(`
lncli() {
    sudo -u bitcoin /usr/local/bin/lncli \
        --lnddir=/var/lib/lnd \%s
        --macaroonpath=/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon \
        --tlscertpath=/var/lib/lnd/tls.cert \
        "$@"
}
export -f lncli
`, lndNetFlag, cfg.network.LNCLINetwork)
    }

    content := fmt.Sprintf(`
# ── Virtual Private Node ──────────────────────
bitcoin-cli() {
    sudo -u bitcoin /usr/local/bin/bitcoin-cli \
        -datadir=/var/lib/bitcoin \
        -conf=/etc/bitcoin/bitcoin.conf \%s
        "$@"
}
export -f bitcoin-cli
%s`, btcNetFlag, lndBlock)

    f, err := os.OpenFile("/home/ripsline/.bashrc",
        os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = f.WriteString(content)
    return err
}