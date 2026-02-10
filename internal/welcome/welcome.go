// Package welcome displays the post-install dashboard shown
// on every SSH login as the ripsline user. Provides three tabs:
//   - Dashboard: service health, system resources, sync status
//   - Pairing: Zeus and Sparrow wallet connection overview
//   - Logs: journalctl output for tor, bitcoind, lnd
//
// The pairing tab shows side-by-side boxes. Press [z] for full
// Zeus pairing details with QR code, or [s] for full Sparrow details.
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
    qrcode "github.com/skip2/go-qrcode"

    "github.com/ripsline/virtual-private-node/internal/config"
)

// ── Styles (black and white brand) ───────────────────────

var (
    wTitleStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    wActiveTabStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    wInactiveTabStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("250")).
                Background(lipgloss.Color("236")).
                Padding(0, 2)

    wHeaderStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    wLabelStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Width(22)

    wValueStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    wGoodStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    wWarnStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245"))

    wGreenDotStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("10"))

    wRedDotStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("9"))

    wLightningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("135")).
            Bold(true)

    wDimStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    wBorderStyle = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245"))

    wFooterStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    wMonoStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    wWarningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("252")).
            Italic(true)

    wActionStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)
)

// Fixed content width for consistent layout across all tabs
const wContentWidth = 76

// ── Tab and subview definitions ──────────────────────────

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

type subview int

const (
    subviewNone subview = iota
    subviewZeus
    subviewSparrow
    subviewMacaroon
)

// ── Model ────────────────────────────────────────────────

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

func NewModel(cfg *config.AppConfig, version string) Model {
    return Model{
        cfg:       cfg,
        version:   version,
        activeTab: tabDashboard,
        logSource: logBitcoin,
        subview:   subviewNone,
    }
}

// Show launches the welcome TUI.
func Show(cfg *config.AppConfig, version string) {
    m := NewModel(cfg, version)
    p := tea.NewProgram(m, tea.WithAltScreen())
    p.Run()
}

func (m Model) Init() tea.Cmd {
    return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    case tea.KeyMsg:
        // Subview navigation — Enter/Esc/q goes back
        if m.subview != subviewNone {
            switch msg.String() {
            case "enter", "escape":
                m.subview = subviewNone
                return m, nil
            case "q":
                // In macaroon subview, q goes back to zeus
                // In zeus/sparrow subview, q goes back to pairing
                m.subview = subviewNone
                return m, nil
            case "m":
                // From zeus subview, press m for macaroon
                if m.subview == subviewZeus && m.cfg.HasLND() {
                    m.subview = subviewMacaroon
                    return m, nil
                }
            }
            return m, nil
        }

        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit

        // Tab navigation — arrow keys and tab only, no l/h
        case "tab", "right":
            if m.activeTab == tabLogs {
                m.activeTab = tabDashboard
            } else {
                m.activeTab++
            }
            return m, nil

        case "shift+tab", "left":
            if m.activeTab == tabDashboard {
                m.activeTab = tabLogs
            } else {
                m.activeTab--
            }
            return m, nil

        case "1":
            m.activeTab = tabDashboard
        case "2":
            m.activeTab = tabPairing
        case "3":
            m.activeTab = tabLogs

        // Zeus pairing screen
        case "z":
            if m.activeTab == tabPairing && m.cfg.HasLND() {
                m.subview = subviewZeus
                return m, nil
            }

        // Sparrow pairing screen
        case "s":
            if m.activeTab == tabPairing {
                m.subview = subviewSparrow
                return m, nil
            }

        // Macaroon full view (from pairing tab)
        case "m":
            if m.activeTab == tabPairing && m.cfg.HasLND() {
                m.subview = subviewMacaroon
                return m, nil
            }

        // Log source switching — l for LND logs
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
        case "l":
            if m.activeTab == tabLogs && m.cfg.HasLND() {
                m.logSource = logLND
                m.logLines = fetchLogs("lnd", m.height-12)
            }

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

func (m Model) View() string {
    if m.width == 0 {
        return "Loading..."
    }

    // Handle subviews
    switch m.subview {
    case subviewZeus:
        return m.renderZeusScreen()
    case subviewSparrow:
        return m.renderSparrowScreen()
    case subviewMacaroon:
        return m.renderMacaroonView()
    }

    // Render active tab content
    var content string
    switch m.activeTab {
    case tabDashboard:
        content = m.renderDashboard()
    case tabPairing:
        content = m.renderPairing()
    case tabLogs:
        content = m.renderLogs()
    }

    // Use responsive width but cap it
    boxWidth := minInt(m.width-4, wContentWidth)

    // Title and tabs — same width as content box
    title := wTitleStyle.Width(boxWidth).Align(lipgloss.Center).
        Render(" Virtual Private Node v" + m.version + " ")
    tabs := m.renderTabs(boxWidth)
    footer := m.renderFooter()

    body := lipgloss.JoinVertical(lipgloss.Center,
        "",
        title,
        "",
        tabs,
        "",
        content,
    )

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

func (m Model) renderTabs(totalWidth int) string {
    tabs := []struct {
        name string
        id   tab
    }{
        {"Dashboard", tabDashboard},
        {"Pairing", tabPairing},
        {"Logs", tabLogs},
    }

    tabWidth := totalWidth / len(tabs)

    var rendered []string
    for _, t := range tabs {
        if t.id == m.activeTab {
            rendered = append(rendered,
                wActiveTabStyle.Width(tabWidth).Align(lipgloss.Center).Render(t.name))
        } else {
            rendered = append(rendered,
                wInactiveTabStyle.Width(tabWidth).Align(lipgloss.Center).Render(t.name))
        }
    }

    return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m Model) renderFooter() string {
    var hint string
    switch m.activeTab {
    case tabDashboard:
        hint = "← → switch tabs • q quit to shell"
    case tabPairing:
        if m.cfg.HasLND() {
            hint = "z zeus • s sparrow • ← → switch tabs • q quit"
        } else {
            hint = "s sparrow • ← → switch tabs • q quit"
        }
    case tabLogs:
        if m.cfg.HasLND() {
            hint = "t tor • b bitcoin • l lnd • r refresh • ← → tabs • q quit"
        } else {
            hint = "t tor • b bitcoin • r refresh • ← → tabs • q quit"
        }
    }
    return wFooterStyle.Render("  " + hint + "  ")
}

// ── Dashboard tab ────────────────────────────────────────

func (m Model) renderDashboard() string {
    var sections []string

    sections = append(sections, wHeaderStyle.Render("Services"))
    sections = append(sections, "")
    sections = append(sections, m.renderServiceRow("tor"))
    sections = append(sections, m.renderServiceRow("bitcoind"))
    if m.cfg.HasLND() {
        sections = append(sections, m.renderServiceRow("lnd"))
    }

    sections = append(sections, "")
    sections = append(sections, wHeaderStyle.Render("System"))
    sections = append(sections, "")
    sections = append(sections, m.renderSystemStats()...)

    sections = append(sections, "")
    sections = append(sections, wHeaderStyle.Render("Blockchain"))
    sections = append(sections, "")
    sections = append(sections, m.renderBlockchainInfo()...)

    content := lipgloss.JoinVertical(lipgloss.Left, sections...)

    return wBorderStyle.
        Width(minInt(m.width-4, wContentWidth)).
        Padding(1, 2).
        Render(content)
}

func (m Model) renderServiceRow(name string) string {
    label := wLabelStyle.Render(name)
    cmd := exec.Command("systemctl", "is-active", "--quiet", name)
    if cmd.Run() == nil {
        return label + wGreenDotStyle.Render("●") + " running"
    }
    return label + wRedDotStyle.Render("●") + " stopped"
}

func (m Model) renderSystemStats() []string {
    var rows []string

    total, used, pct := diskUsage("/")
    rows = append(rows,
        wLabelStyle.Render("Disk Usage")+
            wValueStyle.Render(fmt.Sprintf("%s / %s (%s)", used, total, pct)))

    ramTotal, ramUsed, ramPct := memUsage()
    rows = append(rows,
        wLabelStyle.Render("RAM Usage")+
            wValueStyle.Render(fmt.Sprintf("%s / %s (%s)", ramUsed, ramTotal, ramPct)))

    btcSize := dirSize("/var/lib/bitcoin")
    rows = append(rows,
        wLabelStyle.Render("Bitcoin Data")+wValueStyle.Render(btcSize))

    if m.cfg.HasLND() {
        lndSize := dirSize("/var/lib/lnd")
        rows = append(rows,
            wLabelStyle.Render("LND Data")+wValueStyle.Render(lndSize))
    }

    return rows
}

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

// ── Pairing tab (overview with side-by-side boxes) ───────

func (m Model) renderPairing() string {
    halfWidth := (minInt(m.width-4, wContentWidth) - 3) / 2

    // Zeus box (left)
    var zeusContent string
    if m.cfg.HasLND() {
        restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
        status := wGreenDotStyle.Render("●") + " ready"
        if restOnion == "" {
            status = wRedDotStyle.Render("●") + " waiting for Tor"
        }

        zeusContent = wLightningStyle.Render("⚡ Zeus Wallet") + "\n\n" +
            wDimStyle.Render("LND REST over Tor") + "\n\n" +
            wLabelStyle.Render("Status") + status + "\n\n" +
            wActionStyle.Render("Press [z] for setup")
    } else {
        zeusContent = wDimStyle.Render("Zeus Wallet") + "\n\n" +
            wDimStyle.Render("LND not installed") + "\n\n" +
            wDimStyle.Render("Install LND to use Zeus")
    }

    zeusBox := wBorderStyle.
        Width(halfWidth).
        Padding(1, 2).
        Render(zeusContent)

    // Sparrow box (right)
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    sparrowStatus := wGreenDotStyle.Render("●") + " ready"
    if btcRPC == "" {
        sparrowStatus = wRedDotStyle.Render("●") + " waiting for Tor"
    }

    sparrowContent := wHeaderStyle.Render("Sparrow Wallet") + "\n\n" +
        wDimStyle.Render("Bitcoin Core RPC over Tor") + "\n\n" +
        wLabelStyle.Render("Status") + sparrowStatus + "\n\n" +
        wActionStyle.Render("Press [s] for setup")

    sparrowBox := wBorderStyle.
        Width(halfWidth).
        Padding(1, 2).
        Render(sparrowContent)

    return lipgloss.JoinHorizontal(lipgloss.Top, zeusBox, " ", sparrowBox)
}

// ── Zeus full pairing screen ─────────────────────────────

func (m Model) renderZeusScreen() string {
    var sections []string

    sections = append(sections, wLightningStyle.Render("⚡ Zeus Wallet — LND REST over Tor"))
    sections = append(sections, "")

    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        sections = append(sections, wWarnStyle.Render("LND REST onion not available yet. Wait for Tor to start."))
    } else {
        // QR code
        mac := readMacaroonHex(m.cfg)
        if mac != "" {
            lndconnectURI := fmt.Sprintf("lndconnect://%s:8080?macaroon=%s",
                restOnion, hexToBase64URL(mac))

            qr := renderQRCode(lndconnectURI)
            if qr != "" {
                sections = append(sections, wDimStyle.Render("Scan with Zeus:"))
                sections = append(sections, "")
                sections = append(sections, qr)
                sections = append(sections, "")
            }
        }

        // Manual connection details
        sections = append(sections, wHeaderStyle.Render("Manual Connection"))
        sections = append(sections, "")
        sections = append(sections, wLabelStyle.Render("Wallet interface:")+wMonoStyle.Render("LND (REST)"))
        sections = append(sections, wLabelStyle.Render("Server address:")+wMonoStyle.Render(restOnion))
        sections = append(sections, wLabelStyle.Render("REST Port:")+wMonoStyle.Render("8080"))
        sections = append(sections, "")

        if mac != "" {
            preview := mac
            if len(preview) > 40 {
                preview = preview[:40] + "..."
            }
            sections = append(sections, wLabelStyle.Render("Macaroon (hex):")+wMonoStyle.Render(preview))
            sections = append(sections, "")
            sections = append(sections, wActionStyle.Render("Press [m] to view full copyable macaroon"))
        } else {
            sections = append(sections, wWarnStyle.Render("Macaroon not available. Create wallet first."))
        }
    }

    sections = append(sections, "")
    sections = append(sections, wDimStyle.Render("Steps:"))
    sections = append(sections, wDimStyle.Render("1. Install Zeus on your phone"))
    sections = append(sections, wDimStyle.Render("2. Enable Tor in Zeus settings"))
    sections = append(sections, wDimStyle.Render("3. Scan QR code above, or add node manually"))
    sections = append(sections, wDimStyle.Render("4. For manual: paste the host, port, and macaroon"))

    content := lipgloss.JoinVertical(lipgloss.Left, sections...)

    box := wBorderStyle.
        Width(minInt(m.width-4, wContentWidth)).
        Padding(1, 2).
        Render(content)

    footer := wFooterStyle.Render("  m macaroon • enter back • q back  ")

    full := lipgloss.JoinVertical(lipgloss.Center,
        "",
        wTitleStyle.Width(minInt(m.width-4, wContentWidth)).Align(lipgloss.Center).
            Render(" Zeus Wallet Setup "),
        "",
        box,
        "",
        footer,
    )

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top,
        full,
    )
}

// ── Sparrow full pairing screen ──────────────────────────

func (m Model) renderSparrowScreen() string {
    var sections []string

    sections = append(sections, wHeaderStyle.Render("Sparrow Wallet — Bitcoin Core RPC over Tor"))
    sections = append(sections, "")

    // Warning about cookie rotation
    sections = append(sections, wWarningStyle.Render(
        "WARNING: Cookie changes on Bitcoin Core restart. Reconnect Sparrow after any restart."))
    sections = append(sections, "")

    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC == "" {
        sections = append(sections, wWarnStyle.Render("Bitcoin RPC onion not available yet."))
    } else {
        port := "8332"
        if !m.cfg.IsMainnet() {
            port = "48332"
        }

        cookieValue := readCookieValue(m.cfg)

        sections = append(sections, wLabelStyle.Render("URL:")+wMonoStyle.Render(btcRPC))
        sections = append(sections, wLabelStyle.Render("Port:")+wMonoStyle.Render(port))
        sections = append(sections, wLabelStyle.Render("User:")+wMonoStyle.Render("__cookie__"))

        if cookieValue != "" {
            sections = append(sections, wLabelStyle.Render("Password:")+wMonoStyle.Render(cookieValue))
        } else {
            sections = append(sections, wLabelStyle.Render("Password:")+
                wWarnStyle.Render("Cookie not available — is bitcoind running?"))
        }
    }

    sections = append(sections, "")
    sections = append(sections, wDimStyle.Render("Steps:"))
    sections = append(sections, wDimStyle.Render("1. In Sparrow: File → Preferences → Server"))
    sections = append(sections, wDimStyle.Render("2. Select Bitcoin Core tab"))
    sections = append(sections, wDimStyle.Render("3. Enter the URL, port, user, and password above"))
    sections = append(sections, wDimStyle.Render("4. Select Test Connection"))
    sections = append(sections, wDimStyle.Render("5. Sparrow needs Tor running on your local machine"))
    sections = append(sections, wDimStyle.Render("   (SOCKS5 proxy: localhost:9050)"))

    content := lipgloss.JoinVertical(lipgloss.Left, sections...)

    box := wBorderStyle.
        Width(minInt(m.width-4, wContentWidth)).
        Padding(1, 2).
        Render(content)

    footer := wFooterStyle.Render("  enter back • q back  ")

    full := lipgloss.JoinVertical(lipgloss.Center,
        "",
        wTitleStyle.Width(minInt(m.width-4, wContentWidth)).Align(lipgloss.Center).
            Render(" Sparrow Wallet Setup "),
        "",
        box,
        "",
        footer,
    )

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top,
        full,
    )
}

// ── Macaroon full-screen view ────────────────────────────

func (m Model) renderMacaroonView() string {
    mac := readMacaroonHex(m.cfg)
    if mac == "" {
        mac = "Macaroon not available."
    }

    title := wLightningStyle.Render("⚡ Admin Macaroon (hex)")
    hint := wDimStyle.Render("Select and copy the text below. Press Enter to go back.")

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
        sources = append(sources, lndS.Render(" [l] LND "))
    }

    sourceTabs := lipgloss.JoinHorizontal(lipgloss.Top, sources...)

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
        Width(minInt(m.width-4, wContentWidth)).
        Padding(1, 2).
        Render(content)
}

// ── QR Code rendering ────────────────────────────────────

// renderQRCode generates a terminal-renderable QR code using
// Unicode half-block characters. Returns empty string on failure.
func renderQRCode(data string) string {
    qr, err := qrcode.New(data, qrcode.Medium)
    if err != nil {
        return ""
    }

    bitmap := qr.Bitmap()
    rows := len(bitmap)
    cols := len(bitmap[0])

    var b strings.Builder

    // Use Unicode half-block characters to render 2 rows per line.
    // ▀ = top half, ▄ = bottom half, █ = full block, ' ' = empty
    for y := 0; y < rows; y += 2 {
        for x := 0; x < cols; x++ {
            top := bitmap[y][x]
            bottom := false
            if y+1 < rows {
                bottom = bitmap[y+1][x]
            }

            switch {
            case top && bottom:
                b.WriteString("█")
            case top && !bottom:
                b.WriteString("▀")
            case !top && bottom:
                b.WriteString("▄")
            default:
                b.WriteString(" ")
            }
        }
        if y+2 < rows {
            b.WriteString("\n")
        }
    }

    return b.String()
}

// hexToBase64URL converts a hex string to base64url encoding
// for the lndconnect:// URI format.
func hexToBase64URL(hexStr string) string {
    data, err := hex.DecodeString(hexStr)
    if err != nil {
        return ""
    }

    // Standard base64 encoding
    const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
    result := make([]byte, 0, (len(data)*4/3)+4)
    padding := (3 - len(data)%3) % 3

    padded := make([]byte, len(data)+padding)
    copy(padded, data)

    for i := 0; i < len(padded); i += 3 {
        n := uint(padded[i])<<16 | uint(padded[i+1])<<8 | uint(padded[i+2])
        result = append(result, chars[(n>>18)&63])
        result = append(result, chars[(n>>12)&63])
        result = append(result, chars[(n>>6)&63])
        result = append(result, chars[n&63])
    }

    if padding > 0 {
        result = result[:len(result)-padding]
    }

    // Convert to base64url: replace + with -, / with _, remove =
    s := string(result)
    s = strings.ReplaceAll(s, "+", "-")
    s = strings.ReplaceAll(s, "/", "_")

    return s
}

// ── Helper functions ─────────────────────────────────────

func readOnion(path string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(data))
}

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

func formatKB(kb int) string {
    if kb >= 1048576 {
        return fmt.Sprintf("%.1f GB", float64(kb)/1048576.0)
    }
    return fmt.Sprintf("%.0f MB", float64(kb)/1024.0)
}

// fetchLogs fetches the last N lines from a systemd service journal.
// No --plain flag as it causes exit code 1 on some Debian installs.
func fetchLogs(service string, lines int) string {
    if lines < 10 {
        lines = 10
    }
    cmd := exec.Command("journalctl", "-u", service,
        "-n", fmt.Sprintf("%d", lines),
        "--no-pager")
    output, err := cmd.CombinedOutput()
    // If we got output despite an error (e.g. truncated journal warning),
    // show the output anyway rather than an error message
    if err != nil && len(output) == 0 {
        return "Could not fetch logs: " + err.Error()
    }
    return strings.TrimSpace(string(output))
}

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

    if strings.HasPrefix(rest, `"`) {
        end := strings.Index(rest[1:], `"`)
        if end == -1 {
            return ""
        }
        return rest[1 : end+1]
    }

    end := strings.IndexAny(rest, ",}\n")
    if end == -1 {
        return strings.TrimSpace(rest)
    }
    return strings.TrimSpace(rest[:end])
}

func minInt(a, b int) int {
    if a < b {
        return a
    }
    return b
}