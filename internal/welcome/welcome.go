// Package welcome displays the post-install dashboard shown
// on every SSH login as the ripsline user. Provides three tabs:
//   - Dashboard: service health, system resources, sync status
//   - Pairing: Zeus and Sparrow wallet connection details
//   - Logs: journalctl output for tor, bitcoind, lnd
//
// Press q to quit and drop to a bash shell.
package welcome

import (
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "strconv"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/ripsline/virtual-private-node/internal/config"
)

// ── Styles (black and white brand) ───────────────────────
//
// All styles use white, black, and grays to match the ripsline
// brand. Green and red are used only for service status dots.
// Purple accent for Lightning-related elements.

var (
    // Title bar — black text on white background
    wTitleStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    // Active tab — black text on white background
    wActiveTabStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    // Inactive tab — light gray on dark gray
    wInactiveTabStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("250")).
                Background(lipgloss.Color("236")).
                Padding(0, 2)

    // Section headers within content
    wHeaderStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    // Left-aligned labels (e.g. "Disk Usage", "Sync Status")
    wLabelStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Width(22)

    // Values next to labels
    wValueStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    // Positive status text (synced, etc)
    wGoodStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    // Neutral/warning status (syncing, waiting)
    wWarnStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245"))

    // Green dot for running services
    wGreenDotStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("10"))

    // Red dot for stopped services
    wRedDotStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("9"))

    // Purple accent for Lightning-related elements
    wLightningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("135")).
            Bold(true)

    // De-emphasized text (instructions, hints)
    wDimStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    // Content box border
    wBorderStyle = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245"))

    // Footer hints
    wFooterStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    // Monospace text for copyable values (onion addresses)
    wMonoStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    // Warning text — white bold with ⚠️ prefix, no background
    wWarningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("252")).
            Italic(true)

    // Actionable hint (press [m] to view macaroon)
    wActionStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)
)

// ── Tab and log source definitions ───────────────────────

type tab int

const (
    tabDashboard tab = iota
    tabPairing
    tabLogs
)

type logSource int

const (
    logTor logSource = iota
    logBitcoin
    logLND
)

// ── Subview for full-screen copyable values ──────────────

type subview int

const (
    subviewNone subview = iota
    subviewMacaroon
)

// ── Model ────────────────────────────────────────────────
//
// Holds all state for the welcome dashboard. Queries system
// and service status on render. The subview field tracks
// whether we're showing a full-screen overlay (e.g. macaroon).

type Model struct {
    cfg       *config.AppConfig
    version   string
    activeTab tab
    logSource logSource
    logLines  string
    subview   subview
    width     int
    height    int
}

// NewModel creates a new welcome screen model.
func NewModel(cfg *config.AppConfig, version string) Model {
    return Model{
        cfg:       cfg,
        version:   version,
        activeTab: tabDashboard,
        logSource: logBitcoin,
        subview:   subviewNone,
    }
}

// Show launches the welcome TUI. Called from cmd/main.go
// on every login after installation is complete.
func Show(cfg *config.AppConfig, version string) {
    m := NewModel(cfg, version)
    p := tea.NewProgram(m, tea.WithAltScreen())
    p.Run()
}

// Init is called once when the TUI starts.
func (m Model) Init() tea.Cmd {
    return nil
}

// Update handles all keyboard input and window resize events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    case tea.KeyMsg:
        // If we're in a subview, Enter or Esc goes back
        if m.subview != subviewNone {
            switch msg.String() {
            case "enter", "escape", "q":
                m.subview = subviewNone
                return m, nil
            }
            return m, nil
        }

        switch msg.String() {
        // Quit — drops user to shell
        case "ctrl+c", "q":
            return m, tea.Quit

        // Tab navigation — right
        case "tab", "right", "l":
            if m.activeTab == tabLogs {
                m.activeTab = tabDashboard
            } else {
                m.activeTab++
            }
            return m, nil

        // Tab navigation — left
        case "shift+tab", "left", "h":
            if m.activeTab == tabDashboard {
                m.activeTab = tabLogs
            } else {
                m.activeTab--
            }
            return m, nil

        // Direct tab selection by number
        case "1":
            m.activeTab = tabDashboard
        case "2":
            m.activeTab = tabPairing
        case "3":
            m.activeTab = tabLogs

        // Macaroon full view (only on pairing tab)
        case "m":
            if m.activeTab == tabPairing && m.cfg.HasLND() {
                m.subview = subviewMacaroon
                return m, nil
            }

        // Log source switching (only on logs tab)
        case "t":
            if m.activeTab == tabLogs {
                m.logSource = logTor
                m.logLines = fetchLogs("tor", m.height-12)
            }
        case "b":
            if m.activeTab == tabLogs {
                m.logSource = logBitcoin
                m.logLines = fetchLogs("bitcoind", m.height-12)
            }

        // [l] for LND logs
        case "L":
            if m.activeTab == tabLogs && m.cfg.HasLND() {
                m.logSource = logLND
                m.logLines = fetchLogs("lnd", m.height-12)
            }

        // Refresh logs
        case "r":
            if m.activeTab == tabLogs {
                switch m.logSource {
                case logTor:
                    m.logLines = fetchLogs("tor", m.height-12)
                case logBitcoin:
                    m.logLines = fetchLogs("bitcoind", m.height-12)
                case logLND:
                    m.logLines = fetchLogs("lnd", m.height-12)
                }
            }
        }
    }

    return m, nil
}

// View renders the entire TUI screen centered in the terminal.
func (m Model) View() string {
    if m.width == 0 {
        return "Loading..."
    }

    // Handle subviews (full-screen overlays)
    if m.subview == subviewMacaroon {
        return m.renderMacaroonView()
    }

    // Render the active tab's content
    var content string
    switch m.activeTab {
    case tabDashboard:
        content = m.renderDashboard()
    case tabPairing:
        content = m.renderPairing()
    case tabLogs:
        content = m.renderLogs()
    }

    // Compose: title → tabs → content → footer
    title := wTitleStyle.Render(" Virtual Private Node v" + m.version + " ")
    tabs := m.renderTabs()
    footer := m.renderFooter()

    body := lipgloss.JoinVertical(lipgloss.Center,
        "",
        title,
        "",
        tabs,
        "",
        content,
    )

    // Push footer to bottom
    bodyHeight := lipgloss.Height(body)
    gap := m.height - bodyHeight - 2
    if gap < 0 {
        gap = 0
    }

    full := lipgloss.JoinVertical(lipgloss.Center,
        body,
        strings.Repeat("\n", gap),
        footer,
    )

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top,
        full,
    )
}

// ── Tab bar ──────────────────────────────────────────────

func (m Model) renderTabs() string {
    tabs := []struct {
        name string
        id   tab
    }{
        {"Dashboard", tabDashboard},
        {"Pairing", tabPairing},
        {"Logs", tabLogs},
    }

    var rendered []string
    for _, t := range tabs {
        if t.id == m.activeTab {
            rendered = append(rendered, wActiveTabStyle.Render(t.name))
        } else {
            rendered = append(rendered, wInactiveTabStyle.Render(t.name))
        }
    }

    return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// renderFooter shows context-sensitive keyboard hints.
func (m Model) renderFooter() string {
    var hint string
    switch m.activeTab {
    case tabDashboard:
        hint = "tab/← → switch tabs • q quit to shell"
    case tabPairing:
        if m.cfg.HasLND() {
            hint = "m view macaroon • tab switch • q quit to shell"
        } else {
            hint = "tab/← → switch tabs • q quit to shell"
        }
    case tabLogs:
        if m.cfg.HasLND() {
            hint = "t tor • b bitcoin • L lnd • r refresh • tab switch • q quit"
        } else {
            hint = "t tor • b bitcoin • r refresh • tab switch • q quit"
        }
    }
    return wFooterStyle.Render("  " + hint + "  ")
}

// ── Dashboard tab ────────────────────────────────────────

func (m Model) renderDashboard() string {
    var sections []string

    // Service status
    sections = append(sections, wHeaderStyle.Render("Services"))
    sections = append(sections, "")
    sections = append(sections, m.renderServiceRow("tor"))
    sections = append(sections, m.renderServiceRow("bitcoind"))
    if m.cfg.HasLND() {
        sections = append(sections, m.renderServiceRow("lnd"))
    }

    // System resources
    sections = append(sections, "")
    sections = append(sections, wHeaderStyle.Render("System"))
    sections = append(sections, "")
    sections = append(sections, m.renderSystemStats()...)

    // Blockchain info
    sections = append(sections, "")
    sections = append(sections, wHeaderStyle.Render("Blockchain"))
    sections = append(sections, "")
    sections = append(sections, m.renderBlockchainInfo()...)

    content := lipgloss.JoinVertical(lipgloss.Left, sections...)

    return wBorderStyle.
        Width(minInt(m.width-4, 70)).
        Padding(1, 2).
        Render(content)
}

// renderServiceRow shows a green or red dot with running/stopped status.
func (m Model) renderServiceRow(name string) string {
    label := wLabelStyle.Render(name)
    cmd := exec.Command("systemctl", "is-active", "--quiet", name)
    if cmd.Run() == nil {
        return label + wGreenDotStyle.Render("●") + " running"
    }
    return label + wRedDotStyle.Render("●") + " stopped"
}

// renderSystemStats gathers disk, RAM, and data directory sizes.
func (m Model) renderSystemStats() []string {
    var rows []string

    // Disk usage
    total, used, pct := diskUsage("/")
    rows = append(rows,
        wLabelStyle.Render("Disk Usage")+
            wValueStyle.Render(fmt.Sprintf("%s / %s (%s)", used, total, pct)))

    // RAM usage
    ramTotal, ramUsed, ramPct := memUsage()
    rows = append(rows,
        wLabelStyle.Render("RAM Usage")+
            wValueStyle.Render(fmt.Sprintf("%s / %s (%s)", ramUsed, ramTotal, ramPct)))

    // Bitcoin Core data directory size
    btcSize := dirSize("/var/lib/bitcoin")
    rows = append(rows,
        wLabelStyle.Render("Bitcoin Data")+wValueStyle.Render(btcSize))

    // LND data directory size (only if LND installed)
    if m.cfg.HasLND() {
        lndSize := dirSize("/var/lib/lnd")
        rows = append(rows,
            wLabelStyle.Render("LND Data")+wValueStyle.Render(lndSize))
    }

    return rows
}

// renderBlockchainInfo calls bitcoin-cli and parses the JSON output
// to show sync status, block height, and verification progress.
func (m Model) renderBlockchainInfo() []string {
    var rows []string

    cmd := exec.Command("sudo", "-u", "bitcoin", "bitcoin-cli",
        "-datadir=/var/lib/bitcoin",
        "-conf=/etc/bitcoin/bitcoin.conf",
        "getblockchaininfo")
    output, err := cmd.CombinedOutput()
    if err != nil {
        rows = append(rows,
            wLabelStyle.Render("Status")+wWarnStyle.Render("Bitcoin Core not responding"))
        return rows
    }

    info := string(output)

    blocks := extractJSON(info, "blocks")
    headers := extractJSON(info, "headers")
    ibd := strings.Contains(info, `"initialblockdownload": true`)

    if ibd {
        rows = append(rows,
            wLabelStyle.Render("Sync Status")+wWarnStyle.Render("⟳ syncing"))
    } else {
        rows = append(rows,
            wLabelStyle.Render("Sync Status")+wGoodStyle.Render("✓ synced"))
    }

    rows = append(rows,
        wLabelStyle.Render("Block Height")+
            wValueStyle.Render(blocks+" / "+headers))

    progress := extractJSON(info, "verificationprogress")
    if progress != "" {
        pct, err := strconv.ParseFloat(progress, 64)
        if err == nil {
            rows = append(rows,
                wLabelStyle.Render("Progress")+
                    wValueStyle.Render(fmt.Sprintf("%.2f%%", pct*100)))
        }
    }

    rows = append(rows,
        wLabelStyle.Render("Network")+wValueStyle.Render(m.cfg.Network))
    rows = append(rows,
        wLabelStyle.Render("Prune Size")+
            wValueStyle.Render(fmt.Sprintf("%d GB", m.cfg.PruneSize)))

    return rows
}

// ── Pairing tab ──────────────────────────────────────────

func (m Model) renderPairing() string {
    var sections []string

    // Zeus section (only if LND installed)
    if m.cfg.HasLND() {
        sections = append(sections, m.renderZeus()...)
        sections = append(sections, "")
    }

    // Sparrow section (always available)
    sections = append(sections, m.renderSparrow()...)

    content := lipgloss.JoinVertical(lipgloss.Left, sections...)

    return wBorderStyle.
        Width(minInt(m.width-4, 80)).
        Padding(1, 2).
        Render(content)
}

// renderZeus shows LND REST connection details for Zeus wallet.
// Macaroon is shown as a truncated preview — press [m] for full view.
func (m Model) renderZeus() []string {
    var rows []string

    rows = append(rows, wLightningStyle.Render("⚡️ Zeus Wallet (LND REST over Tor)"))
    rows = append(rows, "")

    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        rows = append(rows, warnStyle.Render("LND REST onion not available yet. Wait for Tor to start."))
        return rows
    }

    rows = append(rows, labelStyle.Render("Wallet interface:")+monoStyle.Render("LND (REST)"))
    rows = append(rows, "")
    rows = append(rows, labelStyle.Render("Server address:")+monoStyle.Render(restOnion))
    rows = append(rows, "")
    rows = append(rows, labelStyle.Render("REST Port:")+monoStyle.Render("8080"))
    rows = append(rows, "")

    // Macaroon preview — truncated to avoid line-wrapping issues
    mac := readMacaroonHex(m.cfg)
    if mac != "" {
        preview := mac
        if len(preview) > 40 {
            preview = preview[:40] + "..."
        }
        rows = append(rows, wLabelStyle.Render("Macaroon (Hex format):")+wMonoStyle.Render(preview))
        rows = append(rows, "")
        rows = append(rows, wActionStyle.Render("Press [m] to view full copyable macaroon"))
    } else {
        rows = append(rows, wWarnStyle.Render("Macaroon not available. Create wallet first."))
        rows = append(rows, "")
        rows = append(rows, wDimStyle.Render("After wallet creation, get it with:"))
        rows = append(rows, wMonoStyle.Render(
            "xxd -ps -c 1000 /var/lib/lnd/data/chain/bitcoin/*/admin.macaroon"))
    }

    rows = append(rows, "")
    rows = append(rows, wDimStyle.Render("Steps:"))
    rows = append(rows, wDimStyle.Render("1. Install Zeus on your phone"))
    rows = append(rows, wDimStyle.Render("2. Enable Tor in Zeus settings"))
    rows = append(rows, wDimStyle.Render("3. Add node → Manual setup"))
    rows = append(rows, wDimStyle.Render("4. Paste the host, port, and macaroon"))

    return rows
}

// renderSparrow shows Bitcoin Core RPC connection details
// using __cookie__ authentication over Tor.
func (m Model) renderSparrow() []string {
    var rows []string

    rows = append(rows, wHeaderStyle.Render("Sparrow Wallet (Bitcoin Core RPC over Tor)"))
    rows = append(rows, "")

    // Warning about cookie rotation — light gray italic, no background
    rows = append(rows, wWarningStyle.Render(
        "⚠️  Cookie changes on Bitcoin Core restart. Reconnect Sparrow after any restart."))
    rows = append(rows, "")

    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC == "" {
        rows = append(rows, wWarnStyle.Render("Bitcoin RPC onion not available yet."))
        return rows
    }

    // RPC port depends on network
    port := "8332"
    if !m.cfg.IsMainnet() {
        port = "48332"
    }

    // Read the current cookie value
    cookieValue := readCookieValue(m.cfg)

    rows = append(rows, labelStyle.Render("URL:")+monoStyle.Render(btcRPC))
    rows = append(rows, labelStyle.Render("Port:")+monoStyle.Render(port))
    rows = append(rows, labelStyle.Render("User:")+monoStyle.Render("__cookie__"))

    if cookieValue != "" {
        rows = append(rows, labelStyle.Render("Password:")+monoStyle.Render(cookieValue))
    } else {
        rows = append(rows, labelStyle.Render("Password:")+
            warnStyle.Render("Cookie not available — is bitcoind running?"))
    }

    rows = append(rows, "")
    rows = append(rows, dimStyle.Render("Steps:"))
    rows = append(rows, dimStyle.Render("1. In Sparrow Wallet: Sparrow → Settings → Server"))
    rows = append(rows, dimStyle.Render("2. Select Bitcoin Core tab"))
    rows = append(rows, dimStyle.Render("3. Enter the URL, port, user, and password above"))
    rows = append(rows, dimStyle.Render("4. Select Test Connection"))

    return rows
}

// ── Macaroon full-screen view ────────────────────────────
//
// Shows the full macaroon hex as raw text with no box wrapping.
// This ensures clean copy-paste without line breaks.

func (m Model) renderMacaroonView() string {
    mac := readMacaroonHex(m.cfg)
    if mac == "" {
        mac = "Macaroon not available."
    }

    title := wLightningStyle.Render("⚡️ Admin Macaroon (hex)")
    hint := wDimStyle.Render("Select and copy the text below. Press Enter to go back.")

    // Raw macaroon with no styling that could add padding/wrapping
    content := lipgloss.JoinVertical(lipgloss.Left,
        "",
        title,
        "",
        hint,
        "",
        mac,
        "",
    )

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        content,
    )
}

// ── Logs tab ─────────────────────────────────────────────

func (m Model) renderLogs() string {
    // Build log source selector
    var sources []string
    torS := wDimStyle
    btcS := wDimStyle
    lndS := wDimStyle

    switch m.logSource {
    case logTor:
        torS = wActiveTabStyle
    case logBitcoin:
        btcS = wActiveTabStyle
    case logLND:
        lndS = wActiveTabStyle
    }

    sources = append(sources, torS.Render(" [t] Tor "))
    sources = append(sources, btcS.Render(" [b] Bitcoin "))
    if m.cfg.HasLND() {
        sources = append(sources, lndS.Render(" [L] LND "))
    }

    sourceTabs := lipgloss.JoinHorizontal(lipgloss.Top, sources...)

    // Fetch logs if not already loaded
    logs := m.logLines
    if logs == "" {
        switch m.logSource {
        case logTor:
            logs = fetchLogs("tor", m.height-12)
        case logBitcoin:
            logs = fetchLogs("bitcoind", m.height-12)
        case logLND:
            logs = fetchLogs("lnd", m.height-12)
        }
    }

    // Truncate to fit screen
    maxLines := m.height - 12
    if maxLines < 5 {
        maxLines = 5
    }
    lines := strings.Split(logs, "\n")
    if len(lines) > maxLines {
        lines = lines[len(lines)-maxLines:]
    }

    logContent := wDimStyle.Render(strings.Join(lines, "\n"))

    content := lipgloss.JoinVertical(lipgloss.Left,
        sourceTabs,
        "",
        logContent,
    )

    return wBorderStyle.
        Width(minInt(m.width-4, 100)).
        Padding(1, 2).
        Render(content)
}

// ── Helper functions ─────────────────────────────────────

// readOnion reads a Tor hidden service hostname file.
func readOnion(path string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(data))
}

// readMacaroonHex reads the admin macaroon and returns it as a
// single hex string — the format Zeus expects.
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

// readCookieValue reads the Bitcoin Core RPC cookie file and
// returns just the password portion (after the colon).
// Format: __cookie__:randomhexvalue
func readCookieValue(cfg *config.AppConfig) string {
    cookiePath := "/var/lib/bitcoin/.cookie"
    if !cfg.IsMainnet() {
        cookiePath = fmt.Sprintf("/var/lib/bitcoin/%s/.cookie", cfg.Network)
    }

    data, err := os.ReadFile(cookiePath)
    if err != nil {
        return ""
    }

    parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
    if len(parts) != 2 {
        return ""
    }

    return parts[1]
}

// diskUsage runs df to get total, used, and percentage for a path.
func diskUsage(path string) (string, string, string) {
    cmd := exec.Command("df", "-h", "--output=size,used,pcent", path)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "N/A", "N/A", "N/A"
    }
    lines := strings.Split(strings.TrimSpace(string(output)), "\n")
    if len(lines) < 2 {
        return "N/A", "N/A", "N/A"
    }
    fields := strings.Fields(lines[1])
    if len(fields) < 3 {
        return "N/A", "N/A", "N/A"
    }
    return fields[0], fields[1], fields[2]
}

// memUsage reads /proc/meminfo to calculate RAM usage.
func memUsage() (string, string, string) {
    data, err := os.ReadFile("/proc/meminfo")
    if err != nil {
        return "N/A", "N/A", "N/A"
    }

    var total, available int
    for _, line := range strings.Split(string(data), "\n") {
        if strings.HasPrefix(line, "MemTotal:") {
            fmt.Sscanf(line, "MemTotal: %d kB", &total)
        }
        if strings.HasPrefix(line, "MemAvailable:") {
            fmt.Sscanf(line, "MemAvailable: %d kB", &available)
        }
    }

    if total == 0 {
        return "N/A", "N/A", "N/A"
    }

    used := total - available
    pct := float64(used) / float64(total) * 100

    return formatKB(total), formatKB(used), fmt.Sprintf("%.0f%%", pct)
}

// dirSize runs du -sh to get the human-readable size of a directory.
func dirSize(path string) string {
    cmd := exec.Command("du", "-sh", path)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "N/A"
    }
    fields := strings.Fields(string(output))
    if len(fields) < 1 {
        return "N/A"
    }
    return fields[0]
}

// formatKB converts kilobytes to a human-readable string (MB or GB).
func formatKB(kb int) string {
    if kb >= 1048576 {
        return fmt.Sprintf("%.1f GB", float64(kb)/1048576.0)
    }
    return fmt.Sprintf("%.0f MB", float64(kb)/1024.0)
}

// fetchLogs fetches the last N lines from a systemd service journal.
// Uses sudo to ensure permission to read journal entries.
func fetchLogs(service string, lines int) string {
    if lines < 10 {
        lines = 10
    }
    cmd := exec.Command("sudo", "journalctl", "-u", service,
        "-n", fmt.Sprintf("%d", lines),
        "--no-pager", "--plain")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "Could not fetch logs: " + err.Error()
    }
    return strings.TrimSpace(string(output))
}

// extractJSON pulls a value from flat JSON text without importing
// encoding/json. Handles both string and numeric values.
// Only suitable for the simple key-value JSON from bitcoin-cli.
func extractJSON(json, key string) string {
    search := fmt.Sprintf(`"%s":`, key)
    idx := strings.Index(json, search)
    if idx == -1 {
        search = fmt.Sprintf(`"%s" :`, key)
        idx = strings.Index(json, search)
        if idx == -1 {
            return ""
        }
    }

    rest := json[idx+len(search):]
    rest = strings.TrimSpace(rest)

    // String values
    if strings.HasPrefix(rest, `"`) {
        end := strings.Index(rest[1:], `"`)
        if end == -1 {
            return ""
        }
        return rest[1 : end+1]
    }

    // Numeric/boolean values
    end := strings.IndexAny(rest, ",}\n")
    if end == -1 {
        return strings.TrimSpace(rest)
    }
    return strings.TrimSpace(rest[:end])
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
    if a < b {
        return a
    }
    return b
}