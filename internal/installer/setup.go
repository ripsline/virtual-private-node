package installer

import (
    "fmt"
    "os"
    "os/exec"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "golang.org/x/term"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"
    "github.com/ripsline/virtual-private-node/internal/theme"
)

const (
    bitcoinVersion = "29.3"
    lndVersion     = "0.20.0-beta"
    litVersion     = "0.16.0-alpha"
    systemUser     = "bitcoin"
)

var appVersion = "dev"

func SetVersion(v string) { appVersion = v }
func LitVersionStr() string { return litVersion }
func LndVersionStr() string { return lndVersion }

func NeedsInstall() bool {
    _, err := os.Stat("/usr/local/bin/bitcoind")
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
    bw := min(m.width-4, theme.ContentWidth)
    title := theme.ProgTitle.Width(bw).Align(lipgloss.Center).
        Render(fmt.Sprintf(" Virtual Private Node v%s ", m.version))
    var lines []string
    for i, s := range m.steps {
        var sty lipgloss.Style
        var ind string
        switch s.status {
        case stepDone:
            sty, ind = theme.ProgDone, "✓"
        case stepRunning:
            sty, ind = theme.ProgRunning, "⟳"
        case stepFailed:
            sty, ind = theme.ProgFail, "✗"
        default:
            sty, ind = theme.ProgPending, "○"
        }
        lines = append(lines, sty.Render(fmt.Sprintf("  %s [%d/%d] %s",
            ind, i+1, len(m.steps), s.name)))
        if s.status == stepFailed && s.err != nil {
            lines = append(lines, theme.ProgFail.Render(
                fmt.Sprintf("      Error: %v", s.err)))
        }
    }
    box := theme.ProgBox.Width(bw).Render(strings.Join(lines, "\n"))
    var footer string
    if m.done && !m.failed {
        footer = theme.Success.Render("  ✓ Complete — press Enter to continue  ")
    } else if m.failed {
        footer = theme.ProgFail.Render("  Failed. Press ctrl+c to exit.  ")
    } else {
        footer = theme.Dim.Render("  Installing... please wait  ")
    }
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}

func RunInstallTUI(steps []installStep, version string) error {
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
    box := theme.Box.Padding(1, 3).Width(min(m.width-8, 70)).Render(m.content)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
func ShowInfoBox(content string) {
    p := tea.NewProgram(infoBoxModel{content: content}, tea.WithAltScreen())
    p.Run()
}

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
    box := theme.Box.Padding(1, 3).Width(min(m.width-8, 70)).Render(m.content)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
func ShowConfirmBox(content string) bool {
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

    // Determine network from pre-seeded config or default
    cfg := config.Default()
    if preCfg, err := config.Load(); err == nil {
        cfg = preCfg
    }

    net := cfg.NetworkConfig()
    steps := buildSteps(cfg, net)

    if err := RunInstallTUI(steps, appVersion); err != nil {
        return err
    }
    if err := setupShellEnvironment(cfg); err != nil {
        fmt.Printf("  Warning: shell setup failed: %v\n", err)
    }
    return config.Save(cfg)
}

func buildSteps(cfg *config.AppConfig, net *config.NetworkConfig) []installStep {
    return []installStep{
        {name: "Creating system user", fn: func() error { return createSystemUser(systemUser) }},
        {name: "Creating directories", fn: func() error { return createBitcoinDirs(systemUser) }},
        {name: "Disabling IPv6", fn: disableIPv6},
        {name: "Configuring firewall", fn: func() error { return configureFirewall(cfg) }},
        {name: "Installing GPG", fn: ensureGPG},
        {name: "Importing Bitcoin Core signing keys", fn: importBitcoinCoreKeys},
        {name: "Installing Tor", fn: installTor},
        {name: "Configuring Tor", fn: func() error { return RebuildTorConfig(cfg) }},
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
}

// ── Wallet creation ──────────────────────────────────────

func RunWalletCreation(cfg *config.AppConfig) error {
    net := cfg.NetworkConfig()
    info := theme.Header.Render("Create Your LND Wallet") + "\n\n" +
        theme.Value.Render("LND will ask you to:") + "\n\n" +
        theme.Value.Render("  1. Enter a wallet password (min 8 characters)") + "\n" +
        theme.Value.Render("  2. Confirm the password") + "\n" +
        theme.Value.Render("  3. 'n' to create a new seed") + "\n" +
        theme.Value.Render("  4. Optionally set a cipher seed passphrase") + "\n" +
        theme.Value.Render("     (press Enter to skip)") + "\n" +
        theme.Value.Render("  5. Write down your 24-word seed phrase") + "\n\n" +
        theme.Warning.Render("WARNING: Your seed is the ONLY way to recover funds.") + "\n" +
        theme.Warning.Render("WARNING: No one can help you if you lose it.") + "\n\n" +
        theme.Dim.Render("Enter to proceed • backspace to cancel")
    if !ShowConfirmBox(info) {
        return nil
    }

    fmt.Print("\033[2J\033[H")
    fmt.Println("\n  ═══════════════════════════════════════════")
    fmt.Println("    LND Wallet Creation")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println("  Waiting for LND...")
    if err := waitForLND(); err != nil {
        return err
    }
    fmt.Println("  ✓ LND is ready")

    cmd := exec.Command("sudo", "-u", systemUser, "lncli",
        "--lnddir=/var/lib/lnd", "--network="+net.LNCLINetwork, "create")
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("lncli create: %w", err)
    }

    seedMsg := theme.Header.Render("Seed Phrase Confirmation") + "\n\n" +
        theme.Warning.Render("Have you written down your 24-word seed phrase?") + "\n\n" +
        theme.Value.Render("Scroll up in your terminal to see your seed.") + "\n" +
        theme.Value.Render("Once you close this session, it will no longer") + "\n" +
        theme.Value.Render("be visible anywhere.") + "\n\n" +
        theme.Value.Render("Make sure you saved it in a secure location.") + "\n\n" +
        theme.Dim.Render("Press Enter to confirm...")
    ShowInfoBox(seedMsg)

    unlockMsg := theme.Header.Render("Auto-Unlock Configuration") + "\n\n" +
        theme.Value.Render("Your wallet password will be stored so LND") + "\n" +
        theme.Value.Render("starts automatically after reboot.") + "\n\n" +
        theme.Dim.Render("Press Enter to continue...")
    ShowInfoBox(unlockMsg)

    fmt.Print("\033[2J\033[H")
    fmt.Println("\n  ═══════════════════════════════════════════")
    fmt.Println("    Auto-Unlock Password")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Print("  Re-enter your wallet password: ")
    pw := readPassword()
    fmt.Println()

    if pw != "" {
        if err := setupAutoUnlock(pw); err != nil {
            fmt.Printf("  Warning: %v\n", err)
        } else {
            fmt.Println("  ✓ Auto-unlock configured")
        }
        cfg.AutoUnlock = true
        config.Save(cfg)
    }
    return nil
}

// ── LND installation ─────────────────────────────────────

func RunLNDInstall(cfg *config.AppConfig) error {
    confirmMsg := theme.Header.Render("Install LND "+lndVersion) + "\n\n" +
        theme.Value.Render("This will:") + "\n\n" +
        theme.Value.Render("  • Download and verify LND v"+lndVersion) + "\n" +
        theme.Value.Render("  • Configure LND for "+cfg.Network) + "\n" +
        theme.Value.Render("  • Create Tor hidden services for LND") + "\n" +
        theme.Value.Render("  • Restart Tor") + "\n\n" +
        theme.Dim.Render("Enter to proceed • backspace to cancel")
    if !ShowConfirmBox(confirmMsg) {
        return nil
    }

    // Ask P2P mode
    p2pMode := "tor"
    p2pMsg := theme.Header.Render("LND P2P Mode") + "\n\n" +
        theme.Value.Render("  [1] Tor only — Maximum privacy") + "\n" +
        theme.Value.Render("  [2] Hybrid  — Tor + clearnet, better routing") + "\n\n" +
        theme.Dim.Render("Press 1 or 2 • backspace to cancel")
    p2pChoice := showChoiceBox(p2pMsg, []string{"1", "2"})
    if p2pChoice == "" {
        return nil
    }
    if p2pChoice == "2" {
        p2pMode = "hybrid"
    }

    publicIPv4 := ""
    if p2pMode == "hybrid" {
        publicIPv4 = system.PublicIPv4()
        if publicIPv4 == "" {
            p2pMode = "tor"
        }
    }

    cfg.P2PMode = p2pMode

    // Set LND as installed BEFORE building steps so torrc includes control port
    cfg.LNDInstalled = true
    cfg.Components = "bitcoin+lnd"

    steps := []installStep{
        {name: "Importing LND signing key", fn: importLNDKey},
        {name: "Downloading LND " + lndVersion, fn: func() error { return downloadLND(lndVersion) }},
        {name: "Verifying LND signature", fn: func() error { return verifyLNDSig(lndVersion) }},
        {name: "Verifying LND checksum", fn: func() error { return verifyLND(lndVersion) }},
        {name: "Installing LND", fn: func() error { return extractAndInstallLND(lndVersion) }},
        {name: "Creating LND directories", fn: func() error { return createLNDDirs(systemUser) }},
        {name: "Configuring LND", fn: func() error { return writeLNDConfig(cfg, publicIPv4) }},
        {name: "Creating LND service", fn: func() error { return writeLNDServiceInitial(systemUser) }},
        {name: "Configuring firewall", fn: func() error { return configureFirewall(cfg) }},
        {name: "Rebuilding Tor config", fn: func() error { return RebuildTorConfig(cfg) }},
        {name: "Restarting Tor", fn: restartTor},
        {name: "Starting LND", fn: startLND},
    }
    if err := RunInstallTUI(steps, appVersion); err != nil {
        cfg.LNDInstalled = false
        cfg.Components = "bitcoin"
        return err
    }
    return config.Save(cfg)
}

// ── LIT installation ─────────────────────────────────────

func RunLITInstall(cfg *config.AppConfig) error {
    confirmMsg := theme.Header.Render("Install Lightning Terminal") + "\n\n" +
        theme.Value.Render("This will:") + "\n\n" +
        theme.Value.Render("  • Download Lightning Terminal v"+litVersion) + "\n" +
        theme.Value.Render("  • Modify LND config (enable rpcmiddleware)") + "\n" +
        theme.Value.Render("  • Restart LND") + "\n" +
        theme.Value.Render("  • Create Tor hidden service for LIT web UI") + "\n" +
        theme.Value.Render("  • Restart Tor") + "\n\n" +
        theme.Dim.Render("Enter to proceed • backspace to cancel")
    if !ShowConfirmBox(confirmMsg) {
        return nil
    }

    passBytes := make([]byte, 12)
    if _, err := randRead(passBytes); err != nil {
        return fmt.Errorf("generate password: %w", err)
    }
    litPassword := hexEncode(passBytes)

    cfg.LITInstalled = true

    steps := []installStep{
        {name: "Importing LIT signing key", fn: importLITKey},
        {name: "Downloading Lightning Terminal " + litVersion, fn: func() error { return downloadLIT(litVersion) }},
        {name: "Verifying LIT signature", fn: func() error { return verifyLITSig(litVersion) }},
        {name: "Verifying LIT checksum", fn: func() error { return verifyLIT(litVersion) }},
        {name: "Installing Lightning Terminal", fn: func() error { return extractAndInstallLIT(litVersion) }},
        {name: "Enabling RPC middleware in LND", fn: enableRPCMiddleware},
        {name: "Restarting LND", fn: func() error { return system.Run("systemctl", "restart", "lnd") }},
        {name: "Creating LIT directories", fn: createLITDirs},
        {name: "Creating LIT configuration", fn: func() error { return writeLITConfig(cfg, litPassword) }},
        {name: "Creating litd service", fn: func() error { return writeLITDService(systemUser) }},
        {name: "Rebuilding Tor config", fn: func() error { return RebuildTorConfig(cfg) }},
        {name: "Restarting Tor", fn: restartTor},
        {name: "Starting Lightning Terminal", fn: startLITD},
    }
    if err := RunInstallTUI(steps, appVersion); err != nil {
        cfg.LITInstalled = false
        return err
    }
    cfg.LITPassword = litPassword
    return config.Save(cfg)
}

// ── Syncthing installation ───────────────────────────────

func RunSyncthingInstall(cfg *config.AppConfig) error {
    confirmMsg := theme.Header.Render("Install Syncthing") + "\n\n" +
        theme.Value.Render("This will:") + "\n\n" +
        theme.Value.Render("  • Install Syncthing from official repository") + "\n" +
        theme.Value.Render("  • Create Tor hidden service for web UI") + "\n" +
        theme.Value.Render("  • Auto-configure LND channel backup sync") + "\n" +
        theme.Value.Render("  • Restart Tor") + "\n\n" +
        theme.Dim.Render("Enter to proceed • backspace to cancel")
    if !ShowConfirmBox(confirmMsg) {
        return nil
    }

    passBytes := make([]byte, 12)
    if _, err := randRead(passBytes); err != nil {
        return fmt.Errorf("generate password: %w", err)
    }
    syncPassword := hexEncode(passBytes)

    cfg.SyncthingInstalled = true

    steps := []installStep{
        {name: "Adding Syncthing repository", fn: installSyncthingRepo},
        {name: "Installing Syncthing", fn: installSyncthingPackage},
        {name: "Creating Syncthing directories", fn: createSyncthingDirs},
        {name: "Creating Syncthing service", fn: writeSyncthingService},
        {name: "Configuring Syncthing authentication", fn: func() error { return configureSyncthingAuth(syncPassword) }},
        {name: "Rebuilding Tor config", fn: func() error { return RebuildTorConfig(cfg) }},
        {name: "Restarting Tor", fn: restartTor},
        {name: "Starting Syncthing", fn: startSyncthing},
        {name: "Setting up channel backup watcher", fn: func() error { return setupChannelBackupWatcher(cfg) }},
    }
    if err := RunInstallTUI(steps, appVersion); err != nil {
        cfg.SyncthingInstalled = false
        return err
    }
    cfg.SyncthingPassword = syncPassword
    return config.Save(cfg)
}

// ── Prune size change ────────────────────────────────────

func RunPruneSizeChange(cfg *config.AppConfig, newSize int) error {
    cfg.PruneSize = newSize
    if err := writeBitcoinConfig(cfg); err != nil {
        return err
    }
    if err := system.Run("systemctl", "restart", "bitcoind"); err != nil {
        return err
    }
    return config.Save(cfg)
}

// ── Choice box ───────────────────────────────────────────

type choiceBoxModel struct {
    content       string
    choices       []string
    result        string
    width, height int
}

func (m choiceBoxModel) Init() tea.Cmd { return nil }
func (m choiceBoxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        switch msg.String() {
        case "backspace", "ctrl+c":
            return m, tea.Quit
        default:
            for _, c := range m.choices {
                if msg.String() == c {
                    m.result = c
                    return m, tea.Quit
                }
            }
        }
    }
    return m, nil
}
func (m choiceBoxModel) View() string {
    if m.width == 0 {
        return "Loading..."
    }
    box := theme.Box.Padding(1, 3).Width(min(m.width-8, 70)).Render(m.content)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
func showChoiceBox(content string, choices []string) string {
    m := choiceBoxModel{content: content, choices: choices}
    p := tea.NewProgram(m, tea.WithAltScreen())
    result, _ := p.Run()
    return result.(choiceBoxModel).result
}

// ── Self-update ──────────────────────────────────────────

func RunSelfUpdate(cfg *config.AppConfig, newVersion string) error {
    confirmMsg := theme.Header.Render("Update Virtual Private Node") + "\n\n" +
        theme.Value.Render("Current: v"+appVersion) + "\n" +
        theme.Value.Render("Latest:  v"+newVersion) + "\n\n" +
        theme.Value.Render("This will download and verify the new binary.") + "\n" +
        theme.Value.Render("The update takes effect on next SSH login.") + "\n\n" +
        theme.Dim.Render("Enter to proceed • backspace to cancel")
    if !ShowConfirmBox(confirmMsg) {
        return nil
    }

    baseURL := fmt.Sprintf(
        "https://github.com/ripsline/virtual-private-node/releases/download/v%s", newVersion)
    pubkeyURL := "https://raw.githubusercontent.com/ripsline/virtual-private-node/main/docs/ripsline-signing-key.asc"
    tarball := fmt.Sprintf("rlvpn-%s-amd64.tar.gz", newVersion)

    steps := []installStep{
        {name: "Downloading v" + newVersion, fn: func() error {
            return system.Download(baseURL+"/"+tarball, "/tmp/"+tarball)
        }},
        {name: "Downloading checksums", fn: func() error {
            if err := system.Download(baseURL+"/SHA256SUMS", "/tmp/rlvpn-SHA256SUMS"); err != nil {
                return err
            }
            return system.Download(baseURL+"/SHA256SUMS.asc", "/tmp/rlvpn-SHA256SUMS.asc")
        }},
        {name: "Importing release key", fn: func() error {
            keyFile := "/tmp/rlvpn-release.pub.asc"
            if err := system.Download(pubkeyURL, keyFile); err != nil {
                return err
            }
            defer os.Remove(keyFile)
            return system.SudoRun("gpg", "--batch", "--import", keyFile)
        }},
        {name: "Verifying signature", fn: func() error {
            cmd := exec.Command("gpg", "--batch", "--verify",
                "/tmp/rlvpn-SHA256SUMS.asc", "/tmp/rlvpn-SHA256SUMS")
            output, err := cmd.CombinedOutput()
            if err != nil {
                return fmt.Errorf("signature verification failed: %s", output)
            }
            return nil
        }},
        {name: "Verifying checksum", fn: func() error {
            cmd := exec.Command("sha256sum", "--ignore-missing", "--check", "rlvpn-SHA256SUMS")
            cmd.Dir = "/tmp"
            output, err := cmd.CombinedOutput()
            if err != nil {
                return fmt.Errorf("checksum failed: %s", output)
            }
            return nil
        }},
        {name: "Installing new binary", fn: func() error {
            if err := system.Run("tar", "-xzf", "/tmp/"+tarball, "-C", "/tmp"); err != nil {
                return err
            }
            if err := system.SudoRun("install", "-m", "755", "/tmp/rlvpn", "/usr/local/bin/rlvpn"); err != nil {
                return err
            }
            // Cleanup
            os.Remove("/tmp/" + tarball)
            os.Remove("/tmp/rlvpn-SHA256SUMS")
            os.Remove("/tmp/rlvpn-SHA256SUMS.asc")
            os.Remove("/tmp/rlvpn")
            return nil
        }},
    }

    return RunInstallTUI(steps, appVersion)
}

func CheckLatestVersion() string {
    output, err := system.RunContext(10e9, "curl", "-sL",
        "https://api.github.com/repos/ripsline/virtual-private-node/releases/latest")
    if err != nil {
        return ""
    }
    // Simple parse — look for "tag_name": "v0.2.1"
    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, `"tag_name"`) {
            parts := strings.Split(line, `"`)
            for _, p := range parts {
                if len(p) > 1 && p[0] == 'v' {
                    return p[1:] // strip the v
                }
            }
        }
    }
    return ""
}

func GetVersion() string {
    return appVersion
}

// ── Helpers ──────────────────────────────────────────────

func readPassword() string {
    pw, err := term.ReadPassword(int(os.Stdin.Fd()))
    if err != nil {
        return ""
    }
    return string(pw)
}

func readFileOrDefault(path, def string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return def
    }
    return string(data)
}

func setupShellEnvironment(cfg *config.AppConfig) error {
    net := cfg.NetworkConfig()
    btcNetFlag := ""
    if net.Name == "testnet4" {
        btcNetFlag = "\n        -testnet4 \\"
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
`, btcNetFlag)

    f, err := os.OpenFile("/home/ripsline/.bashrc",
        os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = f.WriteString(content)
    return err
}

// appendLNCLIToShell adds the lncli wrapper after LND is installed.
func AppendLNCLIToShell(cfg *config.AppConfig) error {
    net := cfg.NetworkConfig()
    lndNetFlag := ""
    if net.Name != "mainnet" {
        lndNetFlag = fmt.Sprintf("\n        --network=%s \\", net.LNCLINetwork)
    }
    content := fmt.Sprintf(`
lncli() {
    sudo -u bitcoin /usr/local/bin/lncli \
        --lnddir=/var/lib/lnd \%s
        --macaroonpath=/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon \
        --tlscertpath=/var/lib/lnd/tls.cert \
        "$@"
}
export -f lncli
`, lndNetFlag, net.LNCLINetwork)

    f, err := os.OpenFile("/home/ripsline/.bashrc",
        os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = f.WriteString(content)
    return err
}

// randRead and hexEncode to avoid importing crypto/rand and encoding/hex
// in the setup file header clutter. They're thin wrappers.
func randRead(b []byte) (int, error) {
    // import is at file level
    return randReadImpl(b)
}

func hexEncode(b []byte) string {
    return hexEncodeImpl(b)
}