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
    wTitleStyle = lipgloss.NewStyle().Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).Padding(0, 2)
    wActiveTabStyle = lipgloss.NewStyle().Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).Padding(0, 2)
    wInactiveTabStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("250")).
                Background(lipgloss.Color("236")).Padding(0, 2)
    wHeaderStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
    wLabelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
    wValueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
    wGoodStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
    wWarnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
    wSelectedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("220"))
    wNormalBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("245"))
    wGrayedBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
    wGreenDotStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
    wRedDotStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
    wLightningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("135")).Bold(true)
    wBitcoinStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
    wDimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
    wGrayedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
    wFooterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
    wMonoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
    wWarningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
    wActionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
    wOuterBox       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("245"))
)

const (
    wContentWidth = 76
    wBoxHeight    = 20
)

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
    svLightning
    svZeus
    svSparrow
    svMacaroon
    svQR
    svWalletCreate
    svLITInstall
    svSyncthingInstall
    svSystemUpdate
    svLogView
)

type cardPos int

const (
    cardServices  cardPos = iota
    cardSystem
    cardBitcoin
    cardLightning
)

type logSelection int

const (
    logSelTor logSelection = iota
    logSelBitcoin
    logSelLND
    logSelLIT
    logSelSyncthing
)

// svcAction is used when acting on services from within the card
type svcAction int

const (
    svcActNone svcAction = iota
)

type Model struct {
    cfg          *config.AppConfig
    version      string
    activeTab    wTab
    subview      wSubview
    dashCard     cardPos
    cardActive   bool         // true when "inside" a card
    svcCursor    int          // cursor within services card
    svcConfirm   string       // "r", "s", or "a" when confirming an action
    logSel       logSelection
    pairingFocus int
    width        int
    height       int
    shellAction  wSubview
}

func NewModel(cfg *config.AppConfig, version string) Model {
    return Model{
        cfg: cfg, version: version,
        activeTab: tabDashboard, subview: svNone,
        dashCard: cardServices,
    }
}

func Show(cfg *config.AppConfig, version string) {
    for {
        m := NewModel(cfg, version)
        p := tea.NewProgram(m, tea.WithAltScreen())
        result, _ := p.Run()
        final := result.(Model)

        switch final.shellAction {
        case svWalletCreate:
            installer.RunWalletCreation(cfg.Network)
            if u, e := config.Load(); e == nil {
                cfg = u
            }
            continue
        case svLITInstall:
            installer.RunLITInstall(cfg)
            if u, e := config.Load(); e == nil {
                cfg = u
            }
            continue
        case svSyncthingInstall:
            installer.RunSyncthingInstall(cfg)
            if u, e := config.Load(); e == nil {
                cfg = u
            }
            continue
        case svSystemUpdate:
            runSystemUpdate()
            continue
        case svLogView:
            runLogViewer(final.logSel, cfg)
            continue
        default:
            return
        }
    }
}

func (m Model) Init() tea.Cmd { return nil }

// svcCount returns how many services are shown
func (m Model) svcCount() int {
    n := 2 // tor + bitcoind
    if m.cfg.HasLND() {
        n++
    }
    if m.cfg.LITInstalled {
        n++
    }
    if m.cfg.SyncthingInstalled {
        n++
    }
    return n
}

func (m Model) svcName(i int) string {
    names := []string{"tor", "bitcoind"}
    if m.cfg.HasLND() {
        names = append(names, "lnd")
    }
    if m.cfg.LITInstalled {
        names = append(names, "litd")
    }
    if m.cfg.SyncthingInstalled {
        names = append(names, "syncthing")
    }
    if i < len(names) {
        return names[i]
    }
    return ""
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        return m.handleKey(msg)
    case svcActionDoneMsg:
        // Service action completed, just re-render
        return m, nil
    }
    return m, nil
}

type svcActionDoneMsg struct{}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    key := msg.String()

    // Subviews
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
            if m.subview == svZeus {
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

    // Inside a card
    if m.cardActive {
        return m.handleCardKey(key)
    }

    // Main navigation
    switch key {
    case "q", "ctrl+c":
        return m, tea.Quit
    case "tab":
        m.activeTab = (m.activeTab + 1) % 4
        return m, nil
    case "shift+tab":
        m.activeTab = (m.activeTab + 3) % 4
        return m, nil
    case "1":
        m.activeTab = tabDashboard
    case "2":
        m.activeTab = tabPairing
    case "3":
        m.activeTab = tabLogs
    case "4":
        m.activeTab = tabSoftware

    case "up", "k":
        m = m.navUp()
    case "down", "j":
        m = m.navDown()
    case "left", "h":
        m = m.navLeft()
    case "right", "l":
        m = m.navRight()

    case "enter":
        return m.handleEnter()
    }
    return m, nil
}

func (m Model) handleCardKey(key string) (tea.Model, tea.Cmd) {
    switch key {
    case "backspace":
        m.cardActive = false
        return m, nil
    case "q":
        return m, tea.Quit
    }

    if m.dashCard == cardServices {
        // If confirming an action, handle y/n
        if m.svcConfirm != "" {
            switch key {
            case "y":
                svc := m.svcName(m.svcCursor)
                action := m.svcConfirm
                m.svcConfirm = ""
                return m, func() tea.Msg {
                    exec.Command("systemctl", action, svc).Run()
                    return svcActionDoneMsg{}
                }
            default:
                m.svcConfirm = ""
                return m, nil
            }
        }

        switch key {
        case "up", "k":
            if m.svcCursor > 0 {
                m.svcCursor--
            }
        case "down", "j":
            if m.svcCursor < m.svcCount()-1 {
                m.svcCursor++
            }
        case "r":
            m.svcConfirm = "restart"
            return m, nil
        case "s":
            m.svcConfirm = "stop"
            return m, nil
        case "a":
            m.svcConfirm = "start"
            return m, nil
        }
    }

    if m.dashCard == cardSystem {
        switch key {
        case "u":
            m.shellAction = svSystemUpdate
            return m, tea.Quit
        }
    }

    return m, nil
}

func (m Model) navUp() Model {
    switch m.activeTab {
    case tabDashboard:
        if m.dashCard == cardBitcoin {
            m.dashCard = cardServices
        } else if m.dashCard == cardLightning {
            m.dashCard = cardSystem
        }
    case tabLogs:
        if m.logSel > 0 {
            m.logSel--
        }
    }
    return m
}

func (m Model) navDown() Model {
    switch m.activeTab {
    case tabDashboard:
        if m.dashCard == cardServices {
            m.dashCard = cardBitcoin
        } else if m.dashCard == cardSystem {
            m.dashCard = cardLightning
        }
    case tabLogs:
        mx := logSelLND
        if m.cfg.LITInstalled {
            mx = logSelLIT
        }
        if m.cfg.SyncthingInstalled {
            mx = logSelSyncthing
        }
        if m.logSel < mx {
            m.logSel++
        }
    }
    return m
}

func (m Model) navLeft() Model {
    switch m.activeTab {
    case tabDashboard:
        if m.dashCard == cardSystem {
            m.dashCard = cardServices
        } else if m.dashCard == cardLightning {
            m.dashCard = cardBitcoin
        }
    case tabPairing, tabSoftware:
        m.pairingFocus = 0
    }
    return m
}

func (m Model) navRight() Model {
    switch m.activeTab {
    case tabDashboard:
        if m.dashCard == cardServices {
            m.dashCard = cardSystem
        } else if m.dashCard == cardBitcoin {
            m.dashCard = cardLightning
        }
    case tabPairing, tabSoftware:
        m.pairingFocus = 1
    }
    return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
    switch m.activeTab {
    case tabDashboard:
        switch m.dashCard {
        case cardServices:
            m.cardActive = true
            m.svcCursor = 0
            return m, nil
        case cardSystem:
            m.cardActive = true
            return m, nil
        case cardLightning:
            if !m.cfg.HasLND() {
                return m, nil
            }
            if !m.cfg.WalletExists() {
                m.shellAction = svWalletCreate
                return m, tea.Quit
            }
            m.subview = svLightning
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
        if m.pairingFocus == 0 {
            // Syncthing (left card)
            if m.cfg.HasLND() && m.cfg.WalletExists() && !m.cfg.SyncthingInstalled {
                m.shellAction = svSyncthingInstall
                return m, tea.Quit
            }
        } else {
            // LIT (right card)
            if m.cfg.HasLND() && m.cfg.WalletExists() && !m.cfg.LITInstalled {
                m.shellAction = svLITInstall
                return m, tea.Quit
            }
        }
    }
    return m, nil
}

func (m Model) View() string {
    if m.width == 0 {
        return "Loading..."
    }
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
    body := lipgloss.JoinVertical(lipgloss.Center, "", title, "", tabs, "", content)
    gap := m.height - lipgloss.Height(body) - 2
    if gap < 0 {
        gap = 0
    }
    full := lipgloss.JoinVertical(lipgloss.Center, body, strings.Repeat("\n", gap), footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, full)
}

func (m Model) viewTabs(tw int) string {
    tabs := []struct {
        n string
        t wTab
    }{{"Dashboard", tabDashboard}, {"Pairing", tabPairing}, {"Logs", tabLogs}, {"Software", tabSoftware}}
    w := tw / len(tabs)
    var out []string
    for _, t := range tabs {
        if t.t == m.activeTab {
            out = append(out, wActiveTabStyle.Width(w).Align(lipgloss.Center).Render(t.n))
        } else {
            out = append(out, wInactiveTabStyle.Width(w).Align(lipgloss.Center).Render(t.n))
        }
    }
    return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}

func (m Model) viewFooter() string {
    if m.cardActive {
        if m.dashCard == cardServices {
            return wFooterStyle.Render("  ↑↓ select • [r]estart [s]top [a]start • backspace back • q quit  ")
        }
        if m.dashCard == cardSystem {
            return wFooterStyle.Render("  [u]pdate system • backspace back • q quit  ")
        }
    }
    switch m.activeTab {
    case tabDashboard:
        return wFooterStyle.Render("  ↑↓←→ navigate • enter select • tab switch • q quit  ")
    case tabPairing:
        return wFooterStyle.Render("  ←→ select • enter open • tab switch • q quit  ")
    case tabLogs:
        return wFooterStyle.Render("  ↑↓ select • enter view • tab switch • q quit  ")
    case tabSoftware:
        return wFooterStyle.Render("  enter install • tab switch • q quit  ")
    }
    return ""
}

// ── Dashboard — four product cards in 2x2 grid ──────────

func (m Model) viewDashboard(bw int) string {
    halfW := (bw - 4) / 2
    cardH := wBoxHeight / 2

    svc := m.cardServicesView(halfW, cardH)
    sys := m.cardSystemView(halfW, cardH)
    btc := m.cardBitcoinView(halfW, cardH)
    ln := m.cardLightningView(halfW, cardH)

    top := lipgloss.JoinHorizontal(lipgloss.Top, svc, "  ", sys)
    bot := lipgloss.JoinHorizontal(lipgloss.Top, btc, "  ", ln)

    return lipgloss.JoinVertical(lipgloss.Left, top, "", bot)
}

func (m Model) getBorder(pos cardPos, enabled bool) lipgloss.Style {
    if !enabled {
        return wGrayedBorder
    }
    if m.activeTab == tabDashboard && m.dashCard == pos {
        if m.cardActive {
            return wSelectedBorder
        }
        return wSelectedBorder
    }
    return wNormalBorder
}

func (m Model) cardServicesView(w, h int) string {
    var lines []string
    lines = append(lines, wHeaderStyle.Render("Services"))
    lines = append(lines, "")

    names := []string{"tor", "bitcoind"}
    if m.cfg.HasLND() {
        names = append(names, "lnd")
    }
    if m.cfg.LITInstalled {
        names = append(names, "litd")
    }

    for i, name := range names {
        dot := wRedDotStyle.Render("●")
        cmd := exec.Command("systemctl", "is-active", "--quiet", name)
        if cmd.Run() == nil {
            dot = wGreenDotStyle.Render("●")
        }

        prefix := "  "
        style := wValueStyle
        if m.cardActive && m.dashCard == cardServices && m.svcCursor == i {
            prefix = "▸ "
            style = wActionStyle
        }
        lines = append(lines, prefix+dot+" "+style.Render(name))
    }

    if m.cardActive && m.dashCard == cardServices {
        lines = append(lines, "")
        if m.svcConfirm != "" {
            svc := m.svcName(m.svcCursor)
            lines = append(lines, wWarningStyle.Render(
                fmt.Sprintf("%s %s? [y/n]", m.svcConfirm, svc)))
        } else {
            lines = append(lines, wDimStyle.Render("[r]estart [s]top [a]start"))
        }
    }

    content := padLines(lines, h)
    border := m.getBorder(cardServices, true)
    return border.Width(w).Padding(0, 1).Render(content)
}

func (m Model) cardSystemView(w, h int) string {
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
    lines = append(lines, wLabelStyle.Render("Bitcoin: ")+wValueStyle.Render(btcSize))

    if m.cfg.HasLND() {
        lndSize := dirSize("/var/lib/lnd")
        lines = append(lines, wLabelStyle.Render("LND: ")+wValueStyle.Render(lndSize))
    }

    if m.cardActive && m.dashCard == cardSystem {
        lines = append(lines, "")
        lines = append(lines, wActionStyle.Render("[u]pdate packages"))
    }

    content := padLines(lines, h)
    border := m.getBorder(cardSystem, true)
    return border.Width(w).Padding(0, 1).Render(content)
}

func (m Model) cardBitcoinView(w, h int) string {
    var lines []string
    lines = append(lines, wBitcoinStyle.Render("₿ Bitcoin"))
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
    border := m.getBorder(cardBitcoin, true)
    return border.Width(w).Padding(0, 1).Render(content)
}

func (m Model) cardLightningView(w, h int) string {
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
    border := m.getBorder(cardLightning, hasLND)
    return border.Width(w).Padding(0, 1).Render(content)
}

// ── Lightning detail screen ──────────────────────────────

func (m Model) viewLightning() string {
    bw := wMin(m.width-4, wContentWidth)
    var lines []string

    lines = append(lines, wLightningStyle.Render("⚡ Lightning Node Details"))
    lines = append(lines, "")

    lines = append(lines, wHeaderStyle.Render("Wallet"))
    lines = append(lines, "")

    if m.cfg.WalletExists() {
        lines = append(lines, "  "+wLabelStyle.Render("Status: ")+
            wGoodStyle.Render("created"))
        if m.cfg.AutoUnlock {
            lines = append(lines, "  "+wLabelStyle.Render("Auto-unlock: ")+
                wGoodStyle.Render("enabled"))
        }

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

    lines = append(lines, "")
    lines = append(lines, wHeaderStyle.Render("Lightning Terminal"))
    lines = append(lines, "")

    if m.cfg.LITInstalled {
        lines = append(lines, "  "+wLabelStyle.Render("Status: ")+
            wGoodStyle.Render("installed"))

        litOnion := readOnion("/var/lib/tor/lnd-lit/hostname")
        if litOnion != "" {
            lines = append(lines, "")
            lines = append(lines, "  "+wLabelStyle.Render("Address: ")+
                wMonoStyle.Render(litOnion))
            lines = append(lines, "  "+wLabelStyle.Render("Port: ")+
                wMonoStyle.Render("8443"))
            lines = append(lines, "")
            lines = append(lines, "  "+wDimStyle.Render(
                "In Tor Browser: https://ADDRESS:PORT"))
            lines = append(lines, "  "+wDimStyle.Render(
                "Your browser will show a security warning."))
            lines = append(lines, "  "+wDimStyle.Render(
                "This is expected — click Advanced → Accept Risk and Continue."))
            lines = append(lines, "  "+wDimStyle.Render(
                "The connection is encrypted by Tor."))
        }
        if m.cfg.LITPassword != "" {
            lines = append(lines, "")
            lines = append(lines, "  "+wLabelStyle.Render("UI Password:"))
            lines = append(lines, "  "+wMonoStyle.Render(m.cfg.LITPassword))
        }
    } else {
        lines = append(lines, "  "+wDimStyle.Render("Not installed"))
        lines = append(lines, "  "+wDimStyle.Render("Install from the Software tab"))
    }

    content := strings.Join(lines, "\n")
    box := wOuterBox.Width(bw).Padding(1, 2).Render(content)
    title := wTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(" Lightning Details ")
    footer := wFooterStyle.Render("  backspace back • q quit  ")

    full := lipgloss.JoinVertical(lipgloss.Center,
        "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top, full)
}

// ── Pairing tab — side by side cards ─────────────────────

func (m Model) viewPairing(bw int) string {
    halfW := (bw - 4) / 2
    cardH := wBoxHeight

    // Zeus
    var zeusLines []string
    if m.cfg.HasLND() {
        restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
        status := wGreenDotStyle.Render("●") + " ready"
        if restOnion == "" {
            status = wRedDotStyle.Render("●") + " waiting"
        }
        zeusLines = []string{
            wLightningStyle.Render("⚡ Zeus Wallet"), "",
            wDimStyle.Render("LND REST over Tor"), "",
            status, "",
            wActionStyle.Render("Select for setup ▸"),
        }
    } else {
        zeusLines = []string{
            wGrayedStyle.Render("⚡ Zeus Wallet"), "",
            wGrayedStyle.Render("LND not installed"),
        }
    }

    zeusContent := padLines(zeusLines, cardH)
    zBorder := wNormalBorder
    if m.pairingFocus == 0 {
        zBorder = wSelectedBorder
        if !m.cfg.HasLND() {
            zBorder = wGrayedBorder
        }
    }
    zeusCard := zBorder.Width(halfW).Padding(1, 2).Render(zeusContent)

    // Sparrow
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    sStatus := wGreenDotStyle.Render("●") + " ready"
    if btcRPC == "" {
        sStatus = wRedDotStyle.Render("●") + " waiting"
    }
    sparrowLines := []string{
        wHeaderStyle.Render("Sparrow Wallet"), "",
        wDimStyle.Render("Bitcoin Core RPC / Tor"), "",
        sStatus, "",
        wActionStyle.Render("Select for setup ▸"),
    }
    sparrowContent := padLines(sparrowLines, cardH)
    sBorder := wNormalBorder
    if m.pairingFocus == 1 {
        sBorder = wSelectedBorder
    }
    sparrowCard := sBorder.Width(halfW).Padding(1, 2).Render(sparrowContent)

    return lipgloss.JoinHorizontal(lipgloss.Top, zeusCard, "  ", sparrowCard)
}

// ── Zeus detail screen ───────────────────────────────────

func (m Model) viewZeus() string {
    bw := wMin(m.width-4, wContentWidth)
    var lines []string

    lines = append(lines, wLightningStyle.Render("⚡ Zeus Wallet — LND REST over Tor"))
    lines = append(lines, "")

    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        lines = append(lines, wWarnStyle.Render("LND REST onion not available."))
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
    lines = append(lines, wDimStyle.Render("1. Install Zeus, enable Tor"))
    lines = append(lines, wDimStyle.Render("2. Scan QR or add manually"))
    lines = append(lines, wDimStyle.Render("3. Paste host, port, macaroon"))

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
        "WARNING: Cookie changes on restart."))
    lines = append(lines, wWarningStyle.Render(
        "Reconnect Sparrow after any restart."))
    lines = append(lines, "")

    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC == "" {
        lines = append(lines, wWarnStyle.Render("Bitcoin RPC onion not available."))
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
    lines = append(lines, wDimStyle.Render("1. Sparrow → Preferences → Server"))
    lines = append(lines, wDimStyle.Render("2. Bitcoin Core tab"))
    lines = append(lines, wDimStyle.Render("3. Enter URL, port, user, password"))
    lines = append(lines, wDimStyle.Render("4. Test Connection"))
    lines = append(lines, wDimStyle.Render("5. Tor locally: SOCKS5 localhost:9050"))

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

// ── Macaroon view ────────────────────────────────────────

func (m Model) viewMacaroon() string {
    mac := readMacaroonHex(m.cfg)
    if mac == "" {
        mac = "Macaroon not available."
    }
    title := wLightningStyle.Render("⚡ Admin Macaroon (hex)")
    hint := wDimStyle.Render("Select and copy. Press backspace to go back.")
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
    lines = append(lines, wDimStyle.Render("Zoom out: Cmd+Minus / Ctrl+Minus"))
    if qr != "" {
        lines = append(lines, qr)
    } else {
        lines = append(lines, wWarnStyle.Render("Could not generate QR."))
    }
    lines = append(lines, wFooterStyle.Render("backspace back • q quit"))

    content := lipgloss.JoinVertical(lipgloss.Left, lines...)
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Top, content)
}

// ── Logs tab — TUI-based log viewer ──────────────────────

func (m Model) viewLogs(bw int) string {
    var lines []string
    lines = append(lines, wHeaderStyle.Render("Select a service to view logs"))
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

    if m.cfg.SyncthingInstalled {
        services = append(services, struct {
            name string
            sel  logSelection
        }{"Syncthing", logSelSyncthing})
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
    lines = append(lines, wDimStyle.Render("Press Enter to view logs"))
    lines = append(lines, wDimStyle.Render("Press b in log viewer to return"))

    content := padLines(lines, wBoxHeight)
    return wOuterBox.Width(bw).Padding(1, 2).Render(content)
}

// ── Software tab ─────────────────────────────────────────

func (m Model) viewSoftware(bw int) string {
    halfW := (bw - 4) / 2
    cardH := wBoxHeight

    // Syncthing card (left)
    var syncLines []string
    syncLines = append(syncLines, wHeaderStyle.Render("Syncthing"))
    syncLines = append(syncLines, "")
    syncLines = append(syncLines, wDimStyle.Render("File sync between your"))
    syncLines = append(syncLines, wDimStyle.Render("node and local devices."))
    syncLines = append(syncLines, wDimStyle.Render("Auto-backup LND channels."))
    syncLines = append(syncLines, "")

    if m.cfg.SyncthingInstalled {
        syncLines = append(syncLines, wGreenDotStyle.Render("●")+" "+
            wGoodStyle.Render("Installed"))
        syncLines = append(syncLines, "")

        syncOnion := readOnion("/var/lib/tor/syncthing/hostname")
        if syncOnion != "" {
            syncLines = append(syncLines, wLabelStyle.Render("Address:"))
            syncLines = append(syncLines, "  "+wMonoStyle.Render(syncOnion))
            syncLines = append(syncLines, wLabelStyle.Render("Port: ")+
                wMonoStyle.Render("8384"))
        }
        if m.cfg.SyncthingPassword != "" {
            syncLines = append(syncLines, "")
            syncLines = append(syncLines, wLabelStyle.Render("User: ")+
                wMonoStyle.Render("admin"))
            syncLines = append(syncLines, wLabelStyle.Render("Pass: ")+
                wMonoStyle.Render(m.cfg.SyncthingPassword))
        }
        syncLines = append(syncLines, "")
        syncLines = append(syncLines, wDimStyle.Render("Open in Tor Browser"))
        syncLines = append(syncLines, wDimStyle.Render("http://ADDRESS:8384"))
    } else if !m.cfg.HasLND() {
        syncLines = append(syncLines, wGrayedStyle.Render("Requires LND"))
    } else if !m.cfg.WalletExists() {
        syncLines = append(syncLines, wGrayedStyle.Render("Requires LND wallet"))
    } else {
        syncLines = append(syncLines, wRedDotStyle.Render("●")+" "+
            wDimStyle.Render("Not installed"))
        syncLines = append(syncLines, "")
        syncLines = append(syncLines, wActionStyle.Render("Select to install ▸"))
    }

    syncContent := padLines(syncLines, cardH)
    sBorder := wNormalBorder
    if m.pairingFocus == 0 {
        if m.cfg.HasLND() && m.cfg.WalletExists() {
            sBorder = wSelectedBorder
        } else {
            sBorder = wGrayedBorder
        }
    }
    syncCard := sBorder.Width(halfW).Padding(1, 2).Render(syncContent)

    // LIT card (right)
    var litLines []string
    litLines = append(litLines, wHeaderStyle.Render("Lightning Terminal"))
    litLines = append(litLines, "")
    litLines = append(litLines, wDimStyle.Render("Browser UI for managing"))
    litLines = append(litLines, wDimStyle.Render("channel liquidity. Loop,"))
    litLines = append(litLines, wDimStyle.Render("Pool, Faraday, Terminal."))
    litLines = append(litLines, "")
    litLines = append(litLines, wLabelStyle.Render("Version: ")+
        wValueStyle.Render("v"+installer.LitVersionStr()))
    litLines = append(litLines, "")

    if m.cfg.LITInstalled {
        litLines = append(litLines, wGreenDotStyle.Render("●")+" "+
            wGoodStyle.Render("Installed"))
        litLines = append(litLines, "")
        litLines = append(litLines, wDimStyle.Render("See Lightning tab"))
    } else if !m.cfg.HasLND() {
        litLines = append(litLines, wGrayedStyle.Render("Requires LND"))
    } else if !m.cfg.WalletExists() {
        litLines = append(litLines, wGrayedStyle.Render("Requires LND wallet"))
    } else {
        litLines = append(litLines, wRedDotStyle.Render("●")+" "+
            wDimStyle.Render("Not installed"))
        litLines = append(litLines, "")
        litLines = append(litLines, wActionStyle.Render("Select to install ▸"))
    }

    litContent := padLines(litLines, cardH)
    lBorder := wNormalBorder
    if m.pairingFocus == 1 {
        if m.cfg.HasLND() && m.cfg.WalletExists() {
            lBorder = wSelectedBorder
        } else {
            lBorder = wGrayedBorder
        }
    }
    litCard := lBorder.Width(halfW).Padding(1, 2).Render(litContent)

    return lipgloss.JoinHorizontal(lipgloss.Top, syncCard, "  ", litCard)
}

// ── Shell actions ────────────────────────────────────────

func runSystemUpdate() {
    fmt.Print("\033[2J\033[H")
    fmt.Println()
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println("    System Update")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println()
    fmt.Println("  Running apt update && apt upgrade...")
    fmt.Println("  This may take a few minutes.")
    fmt.Println()

    cmd := exec.Command("apt-get", "update")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    cmd = exec.Command("apt-get", "upgrade", "-y")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    fmt.Println()
    fmt.Println("  ✓ Update complete")

    // Check if reboot needed
    if _, err := os.Stat("/var/run/reboot-required"); err == nil {
        fmt.Println()
        fmt.Println("  ⚠ Reboot required for kernel update.")
        fmt.Print("  Reboot now? [y/N]: ")
        var answer string
        fmt.Scanln(&answer)
        if strings.ToLower(answer) == "y" {
            exec.Command("reboot").Run()
        }
    }

    fmt.Println()
    fmt.Print("  Press Enter to return to dashboard...")
    fmt.Scanln()
}

func runLogViewer(sel logSelection, cfg *config.AppConfig) {
    svcMap := map[logSelection]string{
        logSelTor:     "tor",
        logSelBitcoin: "bitcoind",
        logSelLND:     "lnd",
        logSelLIT:     "litd",
        logSelSyncthing: "syncthing",
    }
    svc := svcMap[sel]

    fmt.Print("\033[2J\033[H")
    fmt.Println()
    fmt.Printf("  ═══════════════════════════════════════════\n")
    fmt.Printf("    %s Logs (last 100 lines)\n", svc)
    fmt.Printf("  ═══════════════════════════════════════════\n")
    fmt.Println()

    // Show last 100 lines statically (no -f follow mode)
    cmd := exec.Command("journalctl", "-u", svc,
        "-n", "100", "--no-pager")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

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
        "--lnddir=/var/lib/lnd", "--network="+cfg.Network,
        "walletbalance")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "total_balance")
}

func getLNDChannelCount(cfg *config.AppConfig) string {
    cmd := exec.Command("sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd", "--network="+cfg.Network,
        "getinfo")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "num_active_channels")
}

func getLNDPubkey(cfg *config.AppConfig) string {
    cmd := exec.Command("sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd", "--network="+cfg.Network,
        "getinfo")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "identity_pubkey")
}

// ── Generic helpers ──────────────────────────────────────

func padLines(lines []string, target int) string {
    for len(lines) < target {
        lines = append(lines, "")
    }
    if len(lines) > target {
        lines = lines[:target]
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
    path := fmt.Sprintf("/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon", network)
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