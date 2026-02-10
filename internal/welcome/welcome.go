// Package welcome displays the post-install dashboard shown
// on every SSH login as the ripsline user. It provides three
// tabs: Dashboard (health/sync), Logs (tor/bitcoin/lnd), and
// Pairing (Zeus and Sparrow connection details).
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
// All styles use only white, black, and grays to match
// the ripsline brand. No color accents.

var (
    // Title bar — black text on white background
    titleStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    // Active tab — black text on white background (matches title)
    activeTabStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    // Inactive tab — light gray text on dark gray background
    inactiveTabStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("250")).
                Background(lipgloss.Color("236")).
                Padding(0, 2)

    // Section headers within content
    headerStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    // Left-aligned labels (e.g. "CPU Temp", "Sync Status")
    labelStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Width(22)

    // Values next to labels
    valueStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    // Positive status indicators (running, synced)
    goodStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    // Neutral/warning status (syncing, waiting)
    warnStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245"))

    // Error status (stopped, failed)
    errStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("9"))

    // De-emphasized text (instructions, hints)
    dimStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    // Content box border
    borderStyle = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245"))

    // Footer hints
    footerStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    // Monospace-style text for copyable values (onion addresses, macaroons)
    monoStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    // Warning banner — dark text on gray background
    bannerStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("245")).
            Padding(0, 1).
            Bold(true)
)

// ── Tab definitions ──────────────────────────────────────

type tab int

const (
    tabDashboard tab = iota
    tabLogs
    tabPairing
)

// ── Log source selection ─────────────────────────────────

type logSource int

const (
    logTor logSource = iota
    logBitcoin
    logLND
)

// ── Model ────────────────────────────────────────────────
//
// The Model holds all state for the welcome TUI. It reads
// from the install config to know what components are running
// and queries system/service status on render.

type Model struct {
    cfg       *config.AppConfig
    version   string
    activeTab tab
    logSource logSource
    logLines  string
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
    }
}

// Show launches the welcome TUI. This is the main entry point
// called from cmd/main.go on every SSH login after installation.
func Show(cfg *config.AppConfig, version string) {
    m := NewModel(cfg, version)
    p := tea.NewProgram(m, tea.WithAltScreen())
    p.Run()
}

// Init is called once when the TUI starts. No initial commands needed.
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
        switch msg.String() {

        // Quit — drops user to shell
        case "ctrl+c", "q":
            return m, tea.Quit

        // Tab navigation — right
        case "tab", "right", "l":
            if m.activeTab == tabPairing {
                m.activeTab = tabDashboard
            } else {
                m.activeTab++
            }
            return m, nil

        // Tab navigation — left
        case "shift+tab", "left", "h":
            if m.activeTab == tabDashboard {
                m.activeTab = tabPairing
            } else {
                m.activeTab--
            }
            return m, nil

        // Direct tab selection by number
        case "1":
            m.activeTab = tabDashboard
        case "2":
            m.activeTab = tabLogs
        case "3":
            m.activeTab = tabPairing

        // Log source switching (only active on Logs tab)
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
        case "n":
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

// View renders the entire TUI screen. Everything is centered
// in the terminal with the title at top, tabs below, content
// in the middle, and footer hints at the bottom.
func (m Model) View() string {
    if m.width == 0 {
        return "Loading..."
    }

    // Render the active tab's content
    var content string
    switch m.activeTab {
    case tabDashboard:
        content = m.renderDashboard()
    case tabLogs:
        content = m.renderLogs()
    case tabPairing:
        content = m.renderPairing()
    }

    // Compose the full layout: title → tabs → content → footer
    title := titleStyle.Render(" Virtual Private Node v" + m.version + " ")
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

    // Push footer to the bottom of the terminal
    bodyHeight := lipgloss.Height(body)
    footerHeight := 1
    gap := m.height - bodyHeight - footerHeight - 1
    if gap < 0 {
        gap = 0
    }

    full := lipgloss.JoinVertical(lipgloss.Center,
        body,
        strings.Repeat("\n", gap),
        footer,
    )

    // Center everything horizontally and pin to top vertically
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top,
        full,
    )
}

// ── Tab bar rendering ────────────────────────────────────

func (m Model) renderTabs() string {
    tabs := []struct {
        name string
        id   tab
    }{
        {"Dashboard", tabDashboard},
        {"Logs", tabLogs},
        {"Pairing", tabPairing},
    }

    var rendered []string
    for _, t := range tabs {
        if t.id == m.activeTab {
            rendered = append(rendered, activeTabStyle.Render(t.name))
        } else {
            rendered = append(rendered, inactiveTabStyle.Render(t.name))
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
    case tabLogs:
        hint = "t tor • b bitcoin • n lnd • r refresh • tab switch • q quit"
    case tabPairing:
        hint = "tab/← → switch tabs • q quit to shell"
    }
    return footerStyle.Render("  " + hint + "  ")
}

// ── Dashboard tab ────────────────────────────────────────
//
// Shows service status, system resources (CPU temp, disk, RAM),
// blockchain data sizes, and sync status.

func (m Model) renderDashboard() string {
    var sections []string

    // Service status section
    sections = append(sections, headerStyle.Render("Services"))
    sections = append(sections, "")
    sections = append(sections, m.renderServiceRow("tor"))
    sections = append(sections, m.renderServiceRow("bitcoind"))
    if m.cfg.HasLND() {
        sections = append(sections, m.renderServiceRow("lnd"))
    }

    // System resources section
    sections = append(sections, "")
    sections = append(sections, headerStyle.Render("System"))
    sections = append(sections, "")
    sections = append(sections, m.renderSystemStats()...)

    // Blockchain status section
    sections = append(sections, "")
    sections = append(sections, headerStyle.Render("Blockchain"))
    sections = append(sections, "")
    sections = append(sections, m.renderBlockchainInfo()...)

    content := lipgloss.JoinVertical(lipgloss.Left, sections...)

    return borderStyle.
        Width(70).
        Padding(1, 2).
        Render(content)
}

// renderServiceRow checks systemd and shows ● running or ● stopped.
func (m Model) renderServiceRow(name string) string {
    label := labelStyle.Render(name)
    cmd := exec.Command("systemctl", "is-active", "--quiet", name)
    if cmd.Run() == nil {
        return label + goodStyle.Render("● running")
    }
    return label + errStyle.Render("● stopped")
}

// renderSystemStats gathers CPU temp, disk, RAM, and data dir sizes.
func (m Model) renderSystemStats() []string {
    var rows []string

    // CPU temperature (may not be available on all VPS providers)
    temp := readCPUTemp()
    rows = append(rows,
        labelStyle.Render("CPU Temp")+valueStyle.Render(temp))

    // Root disk usage
    total, used, pct := diskUsage("/")
    rows = append(rows,
        labelStyle.Render("Disk Usage")+
            valueStyle.Render(fmt.Sprintf("%s / %s (%s)", used, total, pct)))

    // RAM usage from /proc/meminfo
    ramTotal, ramUsed, ramPct := memUsage()
    rows = append(rows,
        labelStyle.Render("RAM Usage")+
            valueStyle.Render(fmt.Sprintf("%s / %s (%s)", ramUsed, ramTotal, ramPct)))

    // Bitcoin Core data directory size
    btcSize := dirSize("/var/lib/bitcoin")
    rows = append(rows,
        labelStyle.Render("Bitcoin Data")+valueStyle.Render(btcSize))

    // LND data directory size (only if LND is installed)
    if m.cfg.HasLND() {
        lndSize := dirSize("/var/lib/lnd")
        rows = append(rows,
            labelStyle.Render("LND Data")+valueStyle.Render(lndSize))
    }

    return rows
}

// renderBlockchainInfo calls bitcoin-cli getblockchaininfo and
// parses the JSON output to show sync status, block height, and progress.
func (m Model) renderBlockchainInfo() []string {
    var rows []string

    cmd := exec.Command("sudo", "-u", "bitcoin", "bitcoin-cli",
        "-datadir=/var/lib/bitcoin",
        "-conf=/etc/bitcoin/bitcoin.conf",
        "getblockchaininfo")
    output, err := cmd.CombinedOutput()
    if err != nil {
        rows = append(rows,
            labelStyle.Render("Status")+warnStyle.Render("Bitcoin Core not responding"))
        return rows
    }

    info := string(output)

    // Extract key fields from the JSON response
    blocks := extractJSON(info, "blocks")
    headers := extractJSON(info, "headers")
    ibd := strings.Contains(info, `"initialblockdownload": true`)

    if ibd {
        rows = append(rows,
            labelStyle.Render("Sync Status")+warnStyle.Render("⟳ syncing"))
    } else {
        rows = append(rows,
            labelStyle.Render("Sync Status")+goodStyle.Render("✓ synced"))
    }

    rows = append(rows,
        labelStyle.Render("Block Height")+
            valueStyle.Render(blocks+" / "+headers))

    // Verification progress as percentage
    progress := extractJSON(info, "verificationprogress")
    if progress != "" {
        pct, err := strconv.ParseFloat(progress, 64)
        if err == nil {
            rows = append(rows,
                labelStyle.Render("Progress")+
                    valueStyle.Render(fmt.Sprintf("%.2f%%", pct*100)))
        }
    }

    rows = append(rows,
        labelStyle.Render("Network")+valueStyle.Render(m.cfg.Network))
    rows = append(rows,
        labelStyle.Render("Prune Size")+
            valueStyle.Render(fmt.Sprintf("%d GB", m.cfg.PruneSize)))

    return rows
}

// ── Logs tab ─────────────────────────────────────────────
//
// Shows journalctl output for the selected service. User can
// switch between tor, bitcoind, and lnd with keyboard shortcuts.

func (m Model) renderLogs() string {
    // Build the log source selector tabs
    var sources []string
    torStyle := dimStyle
    btcStyle := dimStyle
    lndStyle := dimStyle

    switch m.logSource {
    case logTor:
        torStyle = activeTabStyle
    case logBitcoin:
        btcStyle = activeTabStyle
    case logLND:
        lndStyle = activeTabStyle
    }

    sources = append(sources, torStyle.Render(" [t] Tor "))
    sources = append(sources, btcStyle.Render(" [b] Bitcoin "))
    if m.cfg.HasLND() {
        sources = append(sources, lndStyle.Render(" [n] LND "))
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

    // Truncate to fit the available screen height
    maxLines := m.height - 12
    if maxLines < 5 {
        maxLines = 5
    }
    lines := strings.Split(logs, "\n")
    if len(lines) > maxLines {
        lines = lines[len(lines)-maxLines:]
    }

    logContent := dimStyle.Render(strings.Join(lines, "\n"))

    content := lipgloss.JoinVertical(lipgloss.Left,
        sourceTabs,
        "",
        logContent,
    )

    return borderStyle.
        Width(minInt(m.width-4, 100)).
        Padding(1, 2).
        Render(content)
}

// ── Pairing tab ──────────────────────────────────────────
//
// Shows connection details for Zeus (LND REST over Tor) and
// Sparrow (Bitcoin Core RPC over Tor with cookie auth).

func (m Model) renderPairing() string {
    var sections []string

    // Zeus section (only if LND is installed)
    if m.cfg.HasLND() {
        sections = append(sections, m.renderZeus()...)
        sections = append(sections, "")
    }

    // Sparrow section (always available)
    sections = append(sections, m.renderSparrow()...)

    content := lipgloss.JoinVertical(lipgloss.Left, sections...)

    return borderStyle.
        Width(minInt(m.width-4, 100)).
        Padding(1, 2).
        Render(content)
}

// renderZeus shows LND REST connection details for Zeus wallet.
// The macaroon is displayed as a single unbroken hex line so
// the user can copy-paste it directly into Zeus.
func (m Model) renderZeus() []string {
    var rows []string

    rows = append(rows, headerStyle.Render("Zeus Wallet (LND REST over Tor)"))
    rows = append(rows, "")

    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        rows = append(rows, warnStyle.Render("LND REST onion not available yet. Wait for Tor to start."))
        return rows
    }

    rows = append(rows, labelStyle.Render("Server address")+monoStyle.Render(restOnion))
    rows = append(rows, labelStyle.Render("REST Port")+monoStyle.Render("8080"))
    rows = append(rows, labelStyle.Render("Macaroon (Hex format)")+monoStyle.Render("LND (REST)"))
    rows = append(rows, "")

    // Macaroon as a single unbroken hex string for clean copy-paste.
    // No line breaks, no spaces, no chunking.
    mac := readMacaroonHex(m.cfg)
    if mac != "" {
        rows = append(rows, labelStyle.Render("Macaroon (hex):"))
        rows = append(rows, "")
        rows = append(rows, monoStyle.Render(mac))
    } else {
        rows = append(rows, warnStyle.Render("Macaroon not available. Create wallet first."))
        rows = append(rows, "")
        rows = append(rows, dimStyle.Render("After wallet creation, get it with:"))
        rows = append(rows, monoStyle.Render(
            "xxd -ps -c 1000 /var/lib/lnd/data/chain/bitcoin/*/admin.macaroon"))
    }

    rows = append(rows, "")
    rows = append(rows, dimStyle.Render("Steps:"))
    rows = append(rows, dimStyle.Render("1. Download & Verify Zeus on your phone"))
    rows = append(rows, dimStyle.Render("2. Advanced Set-Up"))
    rows = append(rows, dimStyle.Render("3. Create or connect a wallet"))
    rows = append(rows, dimStyle.Render("4. Wallet interface dropdown → LND(REST)"))
    rows = append(rows, dimStyle.Render("5. Paste the Server address, REST Port, and Macaroon above"))

    return rows
}

// renderSparrow shows Bitcoin Core RPC connection details for
// Sparrow wallet using __cookie__ authentication over Tor.
//
// Cookie auth is more secure than rpcauth because the token
// rotates on every bitcoind restart. The tradeoff is that the
// user must reconnect Sparrow after a restart.
func (m Model) renderSparrow() []string {
    var rows []string

    rows = append(rows, headerStyle.Render("Sparrow Wallet (Bitcoin Core RPC over Tor)"))
    rows = append(rows, "")

    // Warning banner about cookie rotation
    rows = append(rows, bannerStyle.Render(
        " ⚠️  Cookie changes on Bitcoin Core restart. Reconnect Sparrow after any restart. "))
    rows = append(rows, "")

    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC == "" {
        rows = append(rows, warnStyle.Render("Bitcoin RPC onion not available yet."))
        return rows
    }

    // RPC port depends on network
    port := "8332"
    if !m.cfg.IsMainnet() {
        port = "48332"
    }

    // Read the current cookie value
    cookieValue := readCookieValue(m.cfg)

    rows = append(rows, labelStyle.Render("URL")+monoStyle.Render(btcRPC))
    rows = append(rows, labelStyle.Render("Port")+monoStyle.Render(port))
    rows = append(rows, labelStyle.Render("User")+monoStyle.Render("__cookie__"))

    if cookieValue != "" {
        rows = append(rows, labelStyle.Render("Password")+monoStyle.Render(cookieValue))
    } else {
        rows = append(rows, labelStyle.Render("Password")+
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
// single hex string. This is the format Zeus expects for manual
// connection setup.
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
// The cookie file format is: __cookie__:randomhexvalue
func readCookieValue(cfg *config.AppConfig) string {
    // Cookie path depends on network
    cookiePath := "/var/lib/bitcoin/.cookie"
    if !cfg.IsMainnet() {
        cookiePath = fmt.Sprintf("/var/lib/bitcoin/%s/.cookie", cfg.Network)
    }

    data, err := os.ReadFile(cookiePath)
    if err != nil {
        return ""
    }

    // Format is __cookie__:hexvalue — extract just the hex part
    parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
    if len(parts) != 2 {
        return ""
    }

    return parts[1]
}

// readCPUTemp reads the CPU temperature from sysfs.
// Returns "N/A" if not available (common on VPS providers
// that don't expose thermal sensors).
func readCPUTemp() string {
    data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
    if err != nil {
        return "N/A"
    }
    milliC := strings.TrimSpace(string(data))
    temp, err := strconv.Atoi(milliC)
    if err != nil {
        return "N/A"
    }
    return fmt.Sprintf("%.1f°C", float64(temp)/1000.0)
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

// dirSize runs du -sh to get the size of a directory.
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

// formatKB converts kilobytes to a human-readable string.
func formatKB(kb int) string {
    if kb >= 1048576 {
        return fmt.Sprintf("%.1f GB", float64(kb)/1048576.0)
    }
    return fmt.Sprintf("%.0f MB", float64(kb)/1024.0)
}

// fetchLogs fetches the last N lines from a systemd service's journal.
func fetchLogs(service string, lines int) string {
    if lines < 10 {
        lines = 10
    }
    cmd := exec.Command("journalctl", "-u", service,
        "-n", fmt.Sprintf("%d", lines),
        "--no-pager", "--plain")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "Could not fetch logs: " + err.Error()
    }
    return strings.TrimSpace(string(output))
}

// extractJSON is a simple helper to pull a value from JSON text
// without importing encoding/json. Works for flat key-value pairs
// in bitcoin-cli output. Not suitable for nested JSON.
func extractJSON(json, key string) string {
    search := fmt.Sprintf(`"%s":`, key)
    idx := strings.Index(json, search)
    if idx == -1 {
        // Try with space before colon (some JSON formatters add it)
        search = fmt.Sprintf(`"%s" :`, key)
        idx = strings.Index(json, search)
        if idx == -1 {
            return ""
        }
    }

    rest := json[idx+len(search):]
    rest = strings.TrimSpace(rest)

    // Handle string values (wrapped in quotes)
    if strings.HasPrefix(rest, `"`) {
        end := strings.Index(rest[1:], `"`)
        if end == -1 {
            return ""
        }
        return rest[1 : end+1]
    }

    // Handle numeric/boolean values (terminated by comma, brace, or newline)
    end := strings.IndexAny(rest, ",}\n")
    if end == -1 {
        return strings.TrimSpace(rest)
    }
    return strings.TrimSpace(rest[:end])
}

// minInt returns the smaller of two ints.
func minInt(a, b int) int {
    if a < b {
        return a
    }
    return b
}