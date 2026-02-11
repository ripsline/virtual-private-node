// Package welcome displays the post-install dashboard.
// Four tabs: Dashboard, Pairing, Logs, Software.
// Dashboard has four navigable cards in a 2x2 grid.
// q quits to shell. Backspace goes back from subviews.
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
    "github.com/ripsline/virtual-private-node/internal/installer"
)

// ── Styles ───────────────────────────────────────────────

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
            Foreground(lipgloss.Color("245"))

    wValueStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    wGoodStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    wWarnStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245"))

    // Yellow for selected/highlighted items
    wSelectedBorder = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("220"))

    wNormalBorder = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245"))

    wGrayedBorder = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("240"))

    wGreenDotStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("10"))

    wRedDotStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("9"))

    wLightningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("135")).
            Bold(true)

    wDimStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    wGrayedStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("240"))

    wFooterStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    wMonoStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15"))

    wWarningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Bold(true)

    wActionStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("220")).
            Bold(true)

    // Outer box that wraps all tab content
    wOuterBox = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245"))
)

// Layout constants — 76 is divisible by 4 tabs
const (
    wContentWidth = 76
    wBoxHeight    = 20
)

// ── Enums ────────────────────────────────────────────────

type wTab int

const (
    tabDashboard wTab = iota
    tabPairing
    tabLogs
    tabSoftware
)

type wSubview int

const (
    svNone wSubview = iota
    svLightning  // hidden Lightning detail screen
    svZeus       // Zeus pairing detail
    svSparrow    // Sparrow pairing detail
    svMacaroon   // full macaroon view
    svQR         // QR code view
    svServiceMgr // service manager shell
    svLogView    // log viewer shell
    svWalletCreate // wallet creation flow
    svLITInstall // LIT install flow
)

// Dashboard card positions for arrow navigation
type cardPos int

const (
    cardServices  cardPos = iota // top-left
    cardSystem                   // top-right
    cardBitcoin                  // bottom-left
    cardLightning                // bottom-right
)

// Logs tab selection
type logSelection int

const (
    logSelTor logSelection = iota
    logSelBitcoin
    logSelLND
    logSelLIT
)

// ── Model ────────────────────────────────────────────────

type Model struct {
    cfg          *config.AppConfig
    version      string
    activeTab    wTab
    subview      wSubview
    dashCard     cardPos    // which card is selected on dashboard
    logSel       logSelection
    pairingFocus int        // 0=zeus, 1=sparrow on pairing tab
    width        int
    height       int

    // Signals to re-launch after shell exit
    shellAction wSubview
}

func NewModel(cfg *config.AppConfig, version string) Model {
    return Model{
        cfg:       cfg,
        version:   version,
        activeTab: tabDashboard,
        subview:   svNone,
        dashCard:  cardServices,
    }
}

// Show launches the welcome TUI. May re-launch after shell actions.
func Show(cfg *config.AppConfig, version string) {
    for {
        m := NewModel(cfg, version)
        p := tea.NewProgram(m, tea.WithAltScreen())
        result, err := p.Run()
        if err != nil {
            return
        }

        final := result.(Model)

        switch final.shellAction {
        case svWalletCreate:
            installer.RunWalletCreation(cfg.Network)
            // Reload config after wallet creation
            if updated, err := config.Load(); err == nil {
                cfg = updated
            }
            continue // re-launch TUI

        case svServiceMgr:
            runServiceManager()
            continue

        case svLogView:
            runLogViewer(final.logSel, cfg)
            continue

        case svLITInstall:
            installer.RunLITInstall(cfg)
            if updated, err := config.Load(); err == nil {
                cfg = updated
            }
            continue

        default:
            return // q was pressed, exit to shell
        }
    }
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    case tea.KeyMsg:
        return m.handleKey(msg)
    }
    return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    key := msg.String()

    // Subview navigation
    if m.subview != svNone {
        switch key {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "backspace":
            switch m.subview {
            case svMacaroon, svQR:
                m.subview = svZeus
            default:
                m.subview = svNone
            }
            return m, nil
        case "m":
            if m.subview == svZeus || m.subview == svLightning {
                m.subview = svMacaroon
                return m, nil
            }
        case "r":
            if m.subview == svZeus {
                m.subview = svQR
                return m, nil
            }
        }
        return m, nil
    }

    // Main screen keys
    switch key {
    case "q", "ctrl+c":
        return m, tea.Quit

    // Tab navigation
    case "tab":
        if m.activeTab == tabSoftware {
            m.activeTab = tabDashboard
        } else {
            m.activeTab++
        }
        return m, nil
    case "shift+tab":
        if m.activeTab == tabDashboard {
            m.activeTab = tabSoftware
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
    case "4":
        m.activeTab = tabSoftware

    // Arrow navigation within tabs
    case "up", "k":
        m = m.handleUp()
    case "down", "j":
        m = m.handleDown()
    case "left", "h":
        m = m.handleLeft()
    case "right", "l":
        m = m.handleRight()

    // Enter — select current item
    case "enter":
        return m.handleEnter()
    }

    return m, nil
}

func (m Model) handleUp() Model {
    switch m.activeTab {
    case tabDashboard:
        switch m.dashCard {
        case cardBitcoin:
            m.dashCard = cardServices
        case cardLightning:
            m.dashCard = cardSystem
        }
    case tabPairing:
        // only two items, no up needed
    case tabLogs:
        if m.logSel > 0 {
            m.logSel--
        }
    }
    return m
}

func (m Model) handleDown() Model {
    switch m.activeTab {
    case tabDashboard:
        switch m.dashCard {
        case cardServices:
            m.dashCard = cardBitcoin
        case cardSystem:
            m.dashCard = cardLightning
        }
    case tabLogs:
        max := logSelLND
        if m.cfg.LITInstalled {
            max = logSelLIT
        }
        if m.logSel < max {
            m.logSel++
        }
    }
    return m
}

func (m Model) handleLeft() Model {
    switch m.activeTab {
    case tabDashboard:
        switch m.dashCard {
        case cardSystem:
            m.dashCard = cardServices
        case cardLightning:
            m.dashCard = cardBitcoin
        }
    case tabPairing:
        m.pairingFocus = 0
    }
    return m
}

func (m Model) handleRight() Model {
    switch m.activeTab {
    case tabDashboard:
        switch m.dashCard {
        case cardServices:
            m.dashCard = cardSystem
        case cardBitcoin:
            m.dashCard = cardLightning
        }
    case tabPairing:
        m.pairingFocus = 1
    }
    return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
    switch m.activeTab {
    case tabDashboard:
        switch m.dashCard {
        case cardServices:
            m.shellAction = svServiceMgr
            return m, tea.Quit
        case cardLightning:
            if m.cfg.HasLND() {
                if !m.cfg.WalletExists() {
                    m.shellAction = svWalletCreate
                    return m, tea.Quit
                }
                m.subview = svLightning
            }
        }

    case tabPairing:
        if m.pairingFocus == 0 && m.cfg.HasLND() {
            m.subview = svZeus
        } else if m.pairingFocus == 1 {
            m.subview = svSparrow
        }

    case tabLogs:
        m.shellAction = svLogView
        return m, tea.Quit

    case tabSoftware:
        if m.cfg.HasLND() && m.cfg.WalletExists() && !m.cfg.LITInstalled {
            m.shellAction = svLITInstall
            return m, tea.Quit
        }
    }

    return m, nil
}

// ── Main View ────────────────────────────────────────────

func (m Model) View() string {
    if m.width == 0 {
        return "Loading..."
    }

    // Handle subviews
    switch m.subview {
    case svLightning:
        return m.viewLightning()
    case svZeus:
        return m.viewZeus()
    case svSparrow:
        return m.viewSparrow()
    case svMacaroon:
        return m.viewMacaroon()
    case svQR:
        return m.viewQR()
    }

    bw := wMin(m.width-4, wContentWidth)

    // Tab content
    var content string
    switch m.activeTab {
    case tabDashboard:
        content = m.viewDashboard(bw)
    case tabPairing:
        content = m.viewPairing(bw)
    case tabLogs:
        content = m.viewLogs(bw)
    case tabSoftware:
        content = m.viewSoftware(bw)
    }

    title := wTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(fmt.Sprintf(" Virtual Private Node v%s ", m.version))
    tabs := m.viewTabs(bw)
    footer := m.viewFooter()

    body := lipgloss.JoinVertical(lipgloss.Center,
        "", title, "", tabs, "", content)

    gap := m.height - lipgloss.Height(body) - 2
    if gap < 0 {
        gap = 0
    }

    full := lipgloss.JoinVertical(lipgloss.Center,
        body, strings.Repeat("\n", gap), footer)

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top, full)
}

func (m Model) viewTabs(totalWidth int) string {
    tabs := []struct {
        name string
        id   wTab
    }{
        {"Dashboard", tabDashboard},
        {"Pairing", tabPairing},
        {"Logs", tabLogs},
        {"Software", tabSoftware},
    }
    tw := totalWidth / len(tabs)
    var out []string
    for _, t := range tabs {
        if t.id == m.activeTab {
            out = append(out, wActiveTabStyle.Width(tw).
                Align(lipgloss.Center).Render(t.name))
        } else {
            out = append(out, wInactiveTabStyle.Width(tw).
                Align(lipgloss.Center).Render(t.name))
        }
    }
    return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}

func (m Model) viewFooter() string {
    var hint string
    switch m.activeTab {
    case tabDashboard:
        hint = "↑↓←→ navigate • enter select • tab switch • q quit"
    case tabPairing:
        hint = "←→ select • enter open • tab switch • q quit"
    case tabLogs:
        hint = "↑↓ select • enter view • tab switch • q quit"
    case tabSoftware:
        hint = "enter install • tab switch • q quit"
    }
    return wFooterStyle.Render("  " + hint + "  ")
}

// ── Dashboard tab — four cards in 2x2 grid ───────────────

func (m Model) viewDashboard(bw int) string {
    halfW := (bw - 4) / 2
    cardH := wBoxHeight / 2

    // Build four cards
    svcCard := m.cardServices(halfW, cardH)
    sysCard := m.cardSystem(halfW, cardH)
    btcCard := m.cardBitcoin(halfW, cardH)
    lnCard := m.cardLightning(halfW, cardH)

    topRow := lipgloss.JoinHorizontal(lipgloss.Top, svcCard, "  ", sysCard)
    botRow := lipgloss.JoinHorizontal(lipgloss.Top, btcCard, "  ", lnCard)

    grid := lipgloss.JoinVertical(lipgloss.Left, topRow, "", botRow)

    return grid
}

// cardBorder returns the appropriate border style for a card
// based on whether it's selected, grayed, or normal.
func (m Model) cardBorder(pos cardPos, enabled bool) lipgloss.Style {
    if !enabled {
        return wGrayedBorder
    }
    if m.activeTab == tabDashboard && m.dashCard == pos {
        return wSelectedBorder
    }
    return wNormalBorder
}

func (m Model) cardServices(w, h int) string {
    var lines []string
    lines = append(lines, wHeaderStyle.Render("Services"))
    lines = append(lines, "")
    lines = append(lines, svcDot("tor"))
    lines = append(lines, svcDot("bitcoind"))
    if m.cfg.HasLND() {
        lines = append(lines, svcDot("lnd"))
    }
    if m.cfg.LITInstalled {
        lines = append(lines, svcDot("litd"))
    }

    content := padLines(lines, h)
    border := m.cardBorder(cardServices, true)
    return border.Width(w).Padding(0, 1).Render(content)
}

func (m Model) cardSystem(w, h int) string {
    var lines []string
    lines = append(lines, wHeaderStyle.Render("System"))
    lines = append(lines, "")

    total, used, pct := diskUsage("/")
    lines = append(lines, wLabelStyle.Render("Disk: ")+
        wValueStyle.Render(fmt.Sprintf("%s / %s (%s)", used, total, pct)))

    ramT, ramU, ramP := memUsage()
    lines = append(lines, wLabelStyle.Render("RAM:  ")+
        wValueStyle.Render(fmt.Sprintf("%s / %s (%s)", ramU, ramT, ramP)))

    btcSize := dirSize("/var/lib/bitcoin")
    lines = append(lines, wLabelStyle.Render("Bitcoin: ")+
        wValueStyle.Render(btcSize))

    if m.cfg.HasLND() {
        lndSize := dirSize("/var/lib/lnd")
        lines = append(lines, wLabelStyle.Render("LND: ")+
            wValueStyle.Render(lndSize))
    }

    content := padLines(lines, h)
    border := m.cardBorder(cardSystem, true)
    return border.Width(w).Padding(0, 1).Render(content)
}

func (m Model) cardBitcoin(w, h int) string {
    var lines []string
    lines = append(lines, wHeaderStyle.Render("Bitcoin"))
    lines = append(lines, "")

    cmd := exec.Command("sudo", "-u", "bitcoin", "bitcoin-cli",
        "-datadir=/var/lib/bitcoin",
        "-conf=/etc/bitcoin/bitcoin.conf",
        "getblockchaininfo")
    output, err := cmd.CombinedOutput()
    if err != nil {
        lines = append(lines, wWarnStyle.Render("Not responding"))
    } else {
        info := string(output)
        blocks := extractJSON(info, "blocks")
        headers := extractJSON(info, "headers")
        ibd := strings.Contains(info, `"initialblockdownload": true`)

        if ibd {
            lines = append(lines, wLabelStyle.Render("Sync: ")+
                wWarnStyle.Render("⟳ syncing"))
        } else {
            lines = append(lines, wLabelStyle.Render("Sync: ")+
                wGoodStyle.Render("✓ synced"))
        }
        lines = append(lines, wLabelStyle.Render("Height: ")+
            wValueStyle.Render(blocks+" / "+headers))

        progress := extractJSON(info, "verificationprogress")
        if progress != "" {
            pct, e := strconv.ParseFloat(progress, 64)
            if e == nil {
                lines = append(lines, wLabelStyle.Render("Progress: ")+
                    wValueStyle.Render(fmt.Sprintf("%.2f%%", pct*100)))
            }
        }
        lines = append(lines, wLabelStyle.Render("Network: ")+
            wValueStyle.Render(m.cfg.Network))
    }

    content := padLines(lines, h)
    border := m.cardBorder(cardBitcoin, true)
    return border.Width(w).Padding(0, 1).Render(content)
}

func (m Model) cardLightning(w, h int) string {
    hasLND := m.cfg.HasLND()

    var lines []string
    if hasLND {
        lines = append(lines, wLightningStyle.Render("⚡ Lightning"))
    } else {
        lines = append(lines, wGrayedStyle.Render("⚡ Lightning"))
    }
    lines = append(lines, "")

    if !hasLND {
        lines = append(lines, wGrayedStyle.Render("LND not installed"))
        lines = append(lines, "")
        lines = append(lines, wGrayedStyle.Render("Install Bitcoin Core + LND"))
        lines = append(lines, wGrayedStyle.Render("to enable Lightning"))
    } else if !m.cfg.WalletExists() {
        lines = append(lines, wLabelStyle.Render("Wallet: ")+
            wWarningStyle.Render("not created"))
        lines = append(lines, "")
        lines = append(lines, wActionStyle.Render("Select to create wallet ▸"))
    } else {
        lines = append(lines, wLabelStyle.Render("Wallet: ")+
            wGoodStyle.Render("created"))
        if m.cfg.AutoUnlock {
            lines = append(lines, wLabelStyle.Render("Auto-unlock: ")+
                wGoodStyle.Render("enabled"))
        }
        lines = append(lines, "")
        lines = append(lines, wActionStyle.Render("Select for details ▸"))
    }

    content := padLines(lines, h)
    border := m.cardBorder(cardLightning, hasLND)
    return border.Width(w).Padding(0, 1).Render(content)
}

// ── Lightning detail screen (hidden tab) ─────────────────

func (m Model) viewLightning() string {
    bw := wMin(m.width-4, wContentWidth)

    var lines []string
    lines = append(lines, wLightningStyle.Render("⚡ Lightning Node Details"))
    lines = append(lines, "")

    // Wallet info
    lines = append(lines, wHeaderStyle.Render("Wallet"))
    lines = append(lines, "")

    if m.cfg.WalletExists() {
        lines = append(lines, "  "+wLabelStyle.Render("Status: ")+
            wGoodStyle.Render("created"))
        if m.cfg.AutoUnlock {
            lines = append(lines, "  "+wLabelStyle.Render("Auto-unlock: ")+
                wGoodStyle.Render("enabled"))
        }

        // Try to get balance from lncli
        balance := getLNDBalance(m.cfg)
        if balance != "" {
            lines = append(lines, "  "+wLabelStyle.Render("Balance: ")+
                wValueStyle.Render(balance+" sats"))
        }

        chanCount := getLNDChannelCount(m.cfg)
        if chanCount != "" {
            lines = append(lines, "  "+wLabelStyle.Render("Channels: ")+
                wValueStyle.Render(chanCount))
        }

        pubkey := getLNDPubkey(m.cfg)
        if pubkey != "" {
            lines = append(lines, "")
            lines = append(lines, "  "+wLabelStyle.Render("Pubkey:"))
            lines = append(lines, "  "+wMonoStyle.Render(pubkey))
        }
    } else {
        lines = append(lines, "  "+wWarningStyle.Render("Wallet not created"))
    }

    // LIT info
    lines = append(lines, "")
    lines = append(lines, wHeaderStyle.Render("Lightning Terminal"))
    lines = append(lines, "")

    if m.cfg.LITInstalled {
        litOnion := readOnion("/var/lib/tor/lnd-lit/hostname")
        lines = append(lines, "  "+wLabelStyle.Render("Status: ")+
            wGoodStyle.Render("installed"))
        if litOnion != "" {
            lines = append(lines, "")
            lines = append(lines, "  "+wLabelStyle.Render("Tor URL:"))
            lines = append(lines, "  "+wMonoStyle.Render(
                "https://"+litOnion+":8443"))
        }
        if m.cfg.LITPassword != "" {
            lines = append(lines, "")
            lines = append(lines, "  "+wLabelStyle.Render("UI Password:"))
            lines = append(lines, "  "+wMonoStyle.Render(m.cfg.LITPassword))
        }
        lines = append(lines, "")
        lines = append(lines, "  "+wDimStyle.Render(
            "Open the Tor URL in Tor Browser to access Terminal"))
    } else {
        lines = append(lines, "  "+wDimStyle.Render("Not installed"))
        lines = append(lines, "  "+wDimStyle.Render(
            "Install from the Software tab"))
    }

    content := strings.Join(lines, "\n")
    box := wOuterBox.Width(bw).Padding(1, 2).Render(content)

    title := wTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(" Lightning Details ")
    footer := wFooterStyle.Render(
        "  m macaroon • backspace back • q quit  ")

    full := lipgloss.JoinVertical(lipgloss.Center,
        "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top, full)
}

// ── Pairing tab — two cards side by side ─────────────────

func (m Model) viewPairing(bw int) string {
    halfW := (bw - 4) / 2
    cardH := wBoxHeight

    // Zeus card
    var zeusLines []string
    if m.cfg.HasLND() {
        restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
        status := wGreenDotStyle.Render("●") + " ready"
        if restOnion == "" {
            status = wRedDotStyle.Render("●") + " waiting"
        }
        zeusLines = []string{
            wLightningStyle.Render("⚡ Zeus Wallet"),
            "",
            wDimStyle.Render("LND REST over Tor"),
            "",
            status,
            "",
            wActionStyle.Render("Select for setup ▸"),
        }
    } else {
        zeusLines = []string{
            wGrayedStyle.Render("⚡ Zeus Wallet"),
            "",
            wGrayedStyle.Render("LND not installed"),
        }
    }

    zeusContent := padLines(zeusLines, cardH)
    zeusBorder := wNormalBorder
    if m.pairingFocus == 0 {
        if m.cfg.HasLND() {
            zeusBorder = wSelectedBorder
        } else {
            zeusBorder = wGrayedBorder
        }
    }
    zeusCard := zeusBorder.Width(halfW).Padding(1, 2).Render(zeusContent)

    // Sparrow card
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    sparrowStatus := wGreenDotStyle.Render("●") + " ready"
    if btcRPC == "" {
        sparrowStatus = wRedDotStyle.Render("●") + " waiting"
    }
    sparrowLines := []string{
        wHeaderStyle.Render("Sparrow Wallet"),
        "",
        wDimStyle.Render("Bitcoin Core RPC / Tor"),
        "",
        sparrowStatus,
        "",
        wActionStyle.Render("Select for setup ▸"),
    }

    sparrowContent := padLines(sparrowLines, cardH)
    sparrowBorder := wNormalBorder
    if m.pairingFocus == 1 {
        sparrowBorder = wSelectedBorder
    }
    sparrowCard := sparrowBorder.Width(halfW).Padding(1, 2).
        Render(sparrowContent)

    return lipgloss.JoinHorizontal(lipgloss.Top,
        zeusCard, "  ", sparrowCard)
}

// ── Zeus detail screen ───────────────────────────────────

func (m Model) viewZeus() string {
    bw := wMin(m.width-4, wContentWidth)

    var lines []string
    lines = append(lines, wLightningStyle.Render(
        "⚡ Zeus Wallet — LND REST over Tor"))
    lines = append(lines, "")

    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        lines = append(lines, wWarnStyle.Render(
            "LND REST onion not available. Wait for Tor."))
    } else {
        lines = append(lines, wHeaderStyle.Render("Connection Details"))
        lines = append(lines, "")
        lines = append(lines, "  "+wLabelStyle.Render("Type: ")+
            wMonoStyle.Render("LND (REST)"))
        lines = append(lines, "  "+wLabelStyle.Render("Port: ")+
            wMonoStyle.Render("8080"))
        lines = append(lines, "")
        lines = append(lines, "  "+wLabelStyle.Render("Host:"))
        lines = append(lines, "  "+wMonoStyle.Render(restOnion))
        lines = append(lines, "")

        mac := readMacaroonHex(m.cfg)
        if mac != "" {
            preview := mac
            if len(preview) > 40 {
                preview = preview[:40] + "..."
            }
            lines = append(lines, "  "+wLabelStyle.Render("Macaroon:"))
            lines = append(lines, "  "+wMonoStyle.Render(preview))
            lines = append(lines, "")
            lines = append(lines, "  "+wActionStyle.Render(
                "[m] full macaroon    [r] QR code"))
        } else {
            lines = append(lines, "  "+wWarningStyle.Render(
                "Macaroon not available. Create wallet first."))
        }
    }

    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render("Steps:"))
    lines = append(lines, wDimStyle.Render(
        "1. Install Zeus, enable Tor in settings"))
    lines = append(lines, wDimStyle.Render(
        "2. Scan QR or add node manually"))
    lines = append(lines, wDimStyle.Render(
        "3. Paste host, port, and macaroon"))

    content := strings.Join(lines, "\n")
    box := wOuterBox.Width(bw).Padding(1, 2).Render(content)

    title := wTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(" Zeus Wallet Setup ")
    footer := wFooterStyle.Render(
        "  m macaroon • r QR • backspace back • q quit  ")

    full := lipgloss.JoinVertical(lipgloss.Center,
        "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top, full)
}

// ── Sparrow detail screen ────────────────────────────────

func (m Model) viewSparrow() string {
    bw := wMin(m.width-4, wContentWidth)

    var lines []string
    lines = append(lines, wHeaderStyle.Render(
        "Sparrow Wallet — Bitcoin Core RPC over Tor"))
    lines = append(lines, "")
    lines = append(lines, wWarningStyle.Render(
        "WARNING: Cookie changes on restart. Reconnect after any restart."))
    lines = append(lines, "")

    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC == "" {
        lines = append(lines, wWarnStyle.Render(
            "Bitcoin RPC onion not available."))
    } else {
        port := "8332"
        if !m.cfg.IsMainnet() {
            port = "48332"
        }
        cookie := readCookieValue(m.cfg)

        lines = append(lines, wHeaderStyle.Render("Connection Details"))
        lines = append(lines, "")
        lines = append(lines, "  "+wLabelStyle.Render("Port: ")+
            wMonoStyle.Render(port))
        lines = append(lines, "  "+wLabelStyle.Render("User: ")+
            wMonoStyle.Render("__cookie__"))
        lines = append(lines, "")
        lines = append(lines, "  "+wLabelStyle.Render("URL:"))
        lines = append(lines, "  "+wMonoStyle.Render(btcRPC))
        lines = append(lines, "")
        if cookie != "" {
            lines = append(lines, "  "+wLabelStyle.Render("Password:"))
            lines = append(lines, "  "+wMonoStyle.Render(cookie))
        } else {
            lines = append(lines, "  "+wLabelStyle.Render("Password: ")+
                wWarnStyle.Render("not available"))
        }
    }

    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render("Steps:"))
    lines = append(lines, wDimStyle.Render(
        "1. In Sparrow: File → Preferences → Server"))
    lines = append(lines, wDimStyle.Render(
        "2. Select Bitcoin Core tab"))
    lines = append(lines, wDimStyle.Render(
        "3. Enter URL, port, user, and password"))
    lines = append(lines, wDimStyle.Render(
        "4. Test Connection"))
    lines = append(lines, wDimStyle.Render(
        "5. Sparrow needs Tor locally (SOCKS5 localhost:9050)"))

    content := strings.Join(lines, "\n")
    box := wOuterBox.Width(bw).Padding(1, 2).Render(content)

    title := wTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(" Sparrow Wallet Setup ")
    footer := wFooterStyle.Render("  backspace back • q quit  ")

    full := lipgloss.JoinVertical(lipgloss.Center,
        "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top, full)
}

// ── Macaroon full view ───────────────────────────────────

func (m Model) viewMacaroon() string {
    mac := readMacaroonHex(m.cfg)
    if mac == "" {
        mac = "Macaroon not available."
    }

    title := wLightningStyle.Render("⚡ Admin Macaroon (hex)")
    hint := wDimStyle.Render(
        "Select and copy the text below. Press backspace to go back.")

    content := lipgloss.JoinVertical(lipgloss.Left,
        "", title, "", hint, "", mac, "")

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center, content)
}

// ── QR code screen ───────────────────────────────────────

func (m Model) viewQR() string {
    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    mac := readMacaroonHex(m.cfg)

    if restOnion == "" || mac == "" {
        c := wWarnStyle.Render("QR not available.")
        return lipgloss.Place(m.width, m.height,
            lipgloss.Center, lipgloss.Center, c)
    }

    uri := fmt.Sprintf("lndconnect://%s:8080?macaroon=%s",
        restOnion, hexToBase64URL(mac))
    qr := renderQRCode(uri)

    var lines []string
    lines = append(lines, wLightningStyle.Render("⚡ Zeus QR Code"))
    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render(
        "You may need to zoom out to see the full QR code."))
    lines = append(lines, wDimStyle.Render(
        "macOS: Cmd+Minus  |  Linux: Ctrl+Minus"))
    lines = append(lines, "")
    if qr != "" {
        lines = append(lines, qr)
    } else {
        lines = append(lines, wWarnStyle.Render("Could not generate QR."))
    }
    lines = append(lines, "")
    lines = append(lines, wFooterStyle.Render(
        "backspace back • q quit"))

    content := lipgloss.JoinVertical(lipgloss.Left, lines...)
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top, content)
}

// ── Logs tab — selectable service list ───────────────────

func (m Model) viewLogs(bw int) string {
    var lines []string
    lines = append(lines, wHeaderStyle.Render(
        "Select a service to view logs"))
    lines = append(lines, "")

    services := []struct {
        name string
        sel  logSelection
    }{
        {"Tor", logSelTor},
        {"Bitcoin Core", logSelBitcoin},
    }
    if m.cfg.HasLND() {
        services = append(services, struct {
            name string
            sel  logSelection
        }{"LND", logSelLND})
    }
    if m.cfg.LITInstalled {
        services = append(services, struct {
            name string
            sel  logSelection
        }{"Lightning Terminal", logSelLIT})
    }

    for _, svc := range services {
        prefix := "  "
        style := wValueStyle
        if m.logSel == svc.sel {
            prefix = "▸ "
            style = wActionStyle
        }
        lines = append(lines, style.Render(prefix+svc.name))
    }

    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render(
        "Press Enter to view logs in terminal"))

    content := padLines(lines, wBoxHeight)
    return wOuterBox.Width(bw).Padding(1, 2).Render(content)
}

// ── Software tab ─────────────────────────────────────────

func (m Model) viewSoftware(bw int) string {
    var lines []string

    lines = append(lines, wHeaderStyle.Render("Lightning Terminal (LIT)"))
    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render(
        "Browser-based interface for managing"))
    lines = append(lines, wDimStyle.Render(
        "channel liquidity. Bundles Loop, Pool,"))
    lines = append(lines, wDimStyle.Render(
        "Faraday, and Terminal UI."))
    lines = append(lines, "")
    lines = append(lines, wLabelStyle.Render("Version: ")+
        wValueStyle.Render("v"+installer.LitVersionStr()))
    lines = append(lines, "")

    if m.cfg.LITInstalled {
        lines = append(lines, wGreenDotStyle.Render("●")+
            " "+wGoodStyle.Render("Installed"))
        lines = append(lines, "")
        lines = append(lines, wDimStyle.Render(
            "Access via Lightning tab on Dashboard"))
    } else if !m.cfg.HasLND() {
        lines = append(lines, wGrayedStyle.Render(
            "Requires LND installation"))
    } else if !m.cfg.WalletExists() {
        lines = append(lines, wGrayedStyle.Render(
            "Requires LND wallet — create from Dashboard"))
    } else {
        lines = append(lines, wRedDotStyle.Render("●")+
            " "+wDimStyle.Render("Not installed"))
        lines = append(lines, "")
        lines = append(lines, wActionStyle.Render(
            "Press Enter to install ▸"))
    }

    content := padLines(lines, wBoxHeight)
    return wOuterBox.Width(bw).Padding(1, 2).Render(content)
}

// ── Shell actions (run outside bubbletea) ────────────────

// runServiceManager shows an interactive service manager in the shell.
func runServiceManager() {
    fmt.Print("\033[2J\033[H")
    fmt.Println()
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println("    Service Manager")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println()

    services := []string{"tor", "bitcoind", "lnd", "litd"}
    reader := strings.NewReader("")
    _ = reader

    for {
        // Show status
        for i, svc := range services {
            status := wRedDotStyle.Render("●") + " stopped"
            cmd := exec.Command("systemctl", "is-active", "--quiet", svc)
            if cmd.Run() == nil {
                status = wGreenDotStyle.Render("●") + " running"
            }
            fmt.Printf("  %d. %-12s %s\n", i+1, svc, status)
        }

        fmt.Println()
        fmt.Print("  Service [1-4] or [b]ack: ")
        var choice string
        fmt.Scanln(&choice)

        if choice == "b" || choice == "B" || choice == "" {
            return
        }

        var idx int
        fmt.Sscanf(choice, "%d", &idx)
        if idx < 1 || idx > len(services) {
            fmt.Println("  Invalid selection")
            continue
        }

        svc := services[idx-1]
        fmt.Printf("\n  %s — [r]estart [s]top [a]start [b]ack: ", svc)
        var action string
        fmt.Scanln(&action)

        switch strings.ToLower(action) {
        case "r":
            fmt.Printf("  Restarting %s...\n", svc)
            cmd := exec.Command("systemctl", "restart", svc)
            if out, err := cmd.CombinedOutput(); err != nil {
                fmt.Printf("  Error: %s %s\n", err, out)
            } else {
                fmt.Printf("  ✓ %s restarted\n", svc)
            }
        case "s":
            fmt.Printf("  Stopping %s...\n", svc)
            cmd := exec.Command("systemctl", "stop", svc)
            if out, err := cmd.CombinedOutput(); err != nil {
                fmt.Printf("  Error: %s %s\n", err, out)
            } else {
                fmt.Printf("  ✓ %s stopped\n", svc)
            }
        case "a":
            fmt.Printf("  Starting %s...\n", svc)
            cmd := exec.Command("systemctl", "start", svc)
            if out, err := cmd.CombinedOutput(); err != nil {
                fmt.Printf("  Error: %s %s\n", err, out)
            } else {
                fmt.Printf("  ✓ %s started\n", svc)
            }
        }

        fmt.Println()
    }
}

// runLogViewer shows journal logs for the selected service.
func runLogViewer(sel logSelection, cfg *config.AppConfig) {
    svcMap := map[logSelection]string{
        logSelTor:     "tor",
        logSelBitcoin: "bitcoind",
        logSelLND:     "lnd",
        logSelLIT:     "litd",
    }

    svc := svcMap[sel]

    fmt.Print("\033[2J\033[H")
    fmt.Println()
    fmt.Printf("  ═══════════════════════════════════════════\n")
    fmt.Printf("    %s Logs\n", svc)
    fmt.Printf("  ═══════════════════════════════════════════\n")
    fmt.Println()
    fmt.Println("  Showing last 50 lines. Press Ctrl+C to stop live view.")
    fmt.Println("  Press Enter after Ctrl+C to return to dashboard.")
    fmt.Println()

    // Show last 50 lines then follow
    cmd := exec.Command("journalctl", "-u", svc,
        "-n", "50", "--no-pager", "-f")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    // Wait for Enter to go back
    fmt.Println()
    fmt.Print("  Press Enter to return to dashboard...")
    fmt.Scanln()
}

// ── QR rendering ─────────────────────────────────────────

func renderQRCode(data string) string {
    qr, err := qrcode.New(data, qrcode.Low)
    if err != nil {
        return ""
    }
    bitmap := qr.Bitmap()
    rows := len(bitmap)
    cols := len(bitmap[0])

    var b strings.Builder
    for y := 0; y < rows; y += 2 {
        for x := 0; x < cols; x++ {
            top := bitmap[y][x]
            bot := false
            if y+1 < rows {
                bot = bitmap[y+1][x]
            }
            switch {
            case top && bot:
                b.WriteString("█")
            case top && !bot:
                b.WriteString("▀")
            case !top && bot:
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

func hexToBase64URL(hexStr string) string {
    data, err := hex.DecodeString(hexStr)
    if err != nil {
        return ""
    }
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
    s := string(result)
    s = strings.ReplaceAll(s, "+", "-")
    s = strings.ReplaceAll(s, "/", "_")
    return s
}

// ── LND query helpers ────────────────────────────────────

func getLNDBalance(cfg *config.AppConfig) string {
    cmd := exec.Command("sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd",
        "--network="+cfg.Network,
        "walletbalance")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "total_balance")
}

func getLNDChannelCount(cfg *config.AppConfig) string {
    cmd := exec.Command("sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd",
        "--network="+cfg.Network,
        "getinfo")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "num_active_channels")
}

func getLNDPubkey(cfg *config.AppConfig) string {
    cmd := exec.Command("sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd",
        "--network="+cfg.Network,
        "getinfo")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "identity_pubkey")
}

// ── Generic helpers ──────────────────────────────────────

func svcDot(name string) string {
    cmd := exec.Command("systemctl", "is-active", "--quiet", name)
    if cmd.Run() == nil {
        return "  " + wGreenDotStyle.Render("●") + " " +
            wValueStyle.Render(name)
    }
    return "  " + wRedDotStyle.Render("●") + " " +
        wDimStyle.Render(name)
}

// padLines pads a slice of lines to exactly targetHeight by
// adding empty lines. Truncates if over.
func padLines(lines []string, targetHeight int) string {
    for len(lines) < targetHeight {
        lines = append(lines, "")
    }
    if len(lines) > targetHeight {
        lines = lines[:targetHeight]
    }
    return strings.Join(lines, "\n")
}

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
        "/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon", network)
    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return hex.EncodeToString(data)
}

func readCookieValue(cfg *config.AppConfig) string {
    p := "/var/lib/bitcoin/.cookie"
    if !cfg.IsMainnet() {
        p = fmt.Sprintf("/var/lib/bitcoin/%s/.cookie", cfg.Network)
    }
    data, err := os.ReadFile(p)
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
    out, err := cmd.CombinedOutput()
    if err != nil {
        return "N/A", "N/A", "N/A"
    }
    lines := strings.Split(strings.TrimSpace(string(out)), "\n")
    if len(lines) < 2 {
        return "N/A", "N/A", "N/A"
    }
    f := strings.Fields(lines[1])
    if len(f) < 3 {
        return "N/A", "N/A", "N/A"
    }
    return f[0], f[1], f[2]
}

func memUsage() (string, string, string) {
    data, err := os.ReadFile("/proc/meminfo")
    if err != nil {
        return "N/A", "N/A", "N/A"
    }
    var total, avail int
    for _, line := range strings.Split(string(data), "\n") {
        if strings.HasPrefix(line, "MemTotal:") {
            fmt.Sscanf(line, "MemTotal: %d kB", &total)
        }
        if strings.HasPrefix(line, "MemAvailable:") {
            fmt.Sscanf(line, "MemAvailable: %d kB", &avail)
        }
    }
    if total == 0 {
        return "N/A", "N/A", "N/A"
    }
    used := total - avail
    pct := float64(used) / float64(total) * 100
    return fmtKB(total), fmtKB(used), fmt.Sprintf("%.0f%%", pct)
}

func dirSize(path string) string {
    cmd := exec.Command("du", "-sh", path)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return "N/A"
    }
    f := strings.Fields(string(out))
    if len(f) < 1 {
        return "N/A"
    }
    return f[0]
}

func fmtKB(kb int) string {
    if kb >= 1048576 {
        return fmt.Sprintf("%.1f GB", float64(kb)/1048576.0)
    }
    return fmt.Sprintf("%.0f MB", float64(kb)/1024.0)
}

func extractJSON(j, key string) string {
    s := fmt.Sprintf(`"%s":`, key)
    idx := strings.Index(j, s)
    if idx == -1 {
        s = fmt.Sprintf(`"%s" :`, key)
        idx = strings.Index(j, s)
        if idx == -1 {
            return ""
        }
    }
    rest := strings.TrimSpace(j[idx+len(s):])
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

func wMin(a, b int) int {
    if a < b {
        return a
    }
    return b
}