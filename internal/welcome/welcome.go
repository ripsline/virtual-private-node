package welcome

import (
    "context"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "time"

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
    svLightning
    svZeus
    svSparrow
    svMacaroon
    svQR
    svFullURL
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

type svcActionDoneMsg struct{}

// ── Model ────────────────────────────────────────────────

type Model struct {
    cfg          *config.AppConfig
    version      string
    activeTab    wTab
    subview      wSubview
    dashCard     cardPos
    cardActive   bool
    svcCursor    int
    svcConfirm   string
    logSel       logSelection
    pairingFocus int
    urlTarget    string
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

// Show launches the welcome TUI. Re-launches after shell actions.
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

func (m Model) svcCount() int {
    n := 2
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

// ── Update ───────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        return m.handleKey(msg)
    case svcActionDoneMsg:
        return m, nil
    }
    return m, nil
}

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
            case svFullURL:
                m.subview = svNone
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
        case "u":
            if m.subview == svLightning {
                litOnion := readOnion("/var/lib/tor/lnd-lit/hostname")
                if litOnion != "" {
                    m.urlTarget = "https://" + litOnion + ":8443"
                    m.subview = svFullURL
                    return m, nil
                }
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
        m.svcConfirm = ""
        return m, nil
    case "q":
        return m, tea.Quit
    }

    if m.dashCard == cardServices {
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
        case "s":
            m.svcConfirm = "stop"
        case "a":
            m.svcConfirm = "start"
        }
    }

    if m.dashCard == cardSystem {
        if key == "u" {
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
            if m.cfg.SyncthingInstalled {
                syncOnion := readOnion("/var/lib/tor/syncthing/hostname")
                if syncOnion != "" {
                    m.urlTarget = "http://" + syncOnion + ":8384"
                    m.subview = svFullURL
                }
            } else if m.cfg.HasLND() && m.cfg.WalletExists() {
                m.shellAction = svSyncthingInstall
                return m, tea.Quit
            }
        } else {
            if m.cfg.LITInstalled {
                litOnion := readOnion("/var/lib/tor/lnd-lit/hostname")
                if litOnion != "" {
                    m.urlTarget = "https://" + litOnion + ":8443"
                    m.subview = svFullURL
                }
            } else if m.cfg.HasLND() && m.cfg.WalletExists() {
                m.shellAction = svLITInstall
                return m, tea.Quit
            }
        }
    }
    return m, nil
}

// ── Main View ────────────────────────────────────────────

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
    case svFullURL:
        return m.viewFullURL()
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

func (m Model) viewTabs(tw int) string {
    tabs := []struct {
        n string
        t wTab
    }{
        {"Dashboard", tabDashboard},
        {"Wallet Pairing", tabPairing},
        {"Logs", tabLogs},
        {"Additional Software", tabSoftware},
    }
    w := tw / len(tabs)
    var out []string
    for _, t := range tabs {
        if t.t == m.activeTab {
            out = append(out, wActiveTabStyle.Width(w).
                Align(lipgloss.Center).Render(t.n))
        } else {
            out = append(out, wInactiveTabStyle.Width(w).
                Align(lipgloss.Center).Render(t.n))
        }
    }
    return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}

func (m Model) viewFooter() string {
    if m.cardActive {
        if m.dashCard == cardServices {
            return wFooterStyle.Render(
                "  ↑↓ select • [r]estart [s]top [a]start • backspace back • q quit  ")
        }
        if m.dashCard == cardSystem {
            return wFooterStyle.Render(
                "  [u]pdate system • backspace back • q quit  ")
        }
    }
    switch m.activeTab {
    case tabDashboard:
        return wFooterStyle.Render(
            "  ↑↓←→ navigate • enter select • tab switch • q quit  ")
    case tabPairing:
        return wFooterStyle.Render(
            "  ←→ select • enter open • tab switch • q quit  ")
    case tabLogs:
        return wFooterStyle.Render(
            "  ↑↓ select • enter view • tab switch • q quit  ")
    case tabSoftware:
        return wFooterStyle.Render(
            "  ←→ select • enter install/view • tab switch • q quit  ")
    }
    return ""
}

// ── Dashboard — four product cards ───────────────────────

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
    if m.cfg.SyncthingInstalled {
        names = append(names, "syncthing")
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

    return m.getBorder(cardServices, true).Width(w).Padding(0, 1).
        Render(padLines(lines, h))
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

    return m.getBorder(cardSystem, true).Width(w).Padding(0, 1).
        Render(padLines(lines, h))
}

func (m Model) cardBitcoinView(w, h int) string {
    var lines []string
    lines = append(lines, wBitcoinStyle.Render("₿ Bitcoin"))
    lines = append(lines, "")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, "sudo", "-u", "bitcoin", "bitcoin-cli",
        "-datadir=/var/lib/bitcoin", "-conf=/etc/bitcoin/bitcoin.conf",
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

    return m.getBorder(cardBitcoin, true).Width(w).Padding(0, 1).
        Render(padLines(lines, h))
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
    } else if !m.cfg.WalletExists() {
        lines = append(lines, wLabelStyle.Render("Wallet: ")+
            wWarningStyle.Render("not created"))
        lines = append(lines, "")
        lines = append(lines, wActionStyle.Render("Select to create ▸"))
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

    return m.getBorder(cardLightning, hasLND).Width(w).Padding(0, 1).
        Render(padLines(lines, h))
}

// ── Lightning detail ─────────────────────────────────────

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
        lines = append(lines, "  "+wGoodStyle.Render("Installed"))
        litOnion := readOnion("/var/lib/tor/lnd-lit/hostname")
        if litOnion != "" {
            lines = append(lines, "")
            lines = append(lines, "  "+wLabelStyle.Render("Address: ")+
                wMonoStyle.Render(litOnion))
            lines = append(lines, "  "+wLabelStyle.Render("Port: ")+
                wMonoStyle.Render("8443"))
            lines = append(lines, "")
            lines = append(lines, "  "+wActionStyle.Render("[u] view full URL to copy"))
            lines = append(lines, "")
            lines = append(lines, "  "+wDimStyle.Render("Open in Tor Browser. Accept security warning."))
        }
        if m.cfg.LITPassword != "" {
            lines = append(lines, "")
            lines = append(lines, "  "+wLabelStyle.Render("UI Password:"))
            lines = append(lines, "  "+wMonoStyle.Render(m.cfg.LITPassword))
        }
    } else {
        lines = append(lines, "  "+wDimStyle.Render("Not installed — use Additional Software tab"))
    }

    content := strings.Join(lines, "\n")
    box := wOuterBox.Width(bw).Padding(1, 2).Render(content)
    title := wTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(" Lightning Details ")
    footer := wFooterStyle.Render("  u full URL • backspace back • q quit  ")
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, full)
}

// ── Full URL view ────────────────────────────────────────

func (m Model) viewFullURL() string {
    title := wHeaderStyle.Render("Full URL — Copy and paste into Tor Browser")
    hint := wDimStyle.Render("Select and copy. Press backspace to go back.")
    content := lipgloss.JoinVertical(lipgloss.Left,
        "", title, "", hint, "", m.urlTarget, "")
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center, content)
}

// ── Pairing tab ──────────────────────────────────────────

func (m Model) viewPairing(bw int) string {
    halfW := (bw - 4) / 2
    cardH := wBoxHeight

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
            status, "", wActionStyle.Render("Select for setup ▸"),
        }
    } else {
        zeusLines = []string{
            wGrayedStyle.Render("⚡ Zeus Wallet"), "",
            wGrayedStyle.Render("LND not installed"),
        }
    }
    zBorder := wNormalBorder
    if m.pairingFocus == 0 {
        if m.cfg.HasLND() {
            zBorder = wSelectedBorder
        } else {
            zBorder = wGrayedBorder
        }
    }
    zeusCard := zBorder.Width(halfW).Padding(1, 2).Render(padLines(zeusLines, cardH))

    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    sStatus := wGreenDotStyle.Render("●") + " ready"
    if btcRPC == "" {
        sStatus = wRedDotStyle.Render("●") + " waiting"
    }
    sparrowLines := []string{
        wHeaderStyle.Render("Sparrow Wallet"), "",
        wDimStyle.Render("Bitcoin Core RPC / Tor"), "",
        sStatus, "", wActionStyle.Render("Select for setup ▸"),
    }
    sBorder := wNormalBorder
    if m.pairingFocus == 1 {
        sBorder = wSelectedBorder
    }
    sparrowCard := sBorder.Width(halfW).Padding(1, 2).Render(padLines(sparrowLines, cardH))

    return lipgloss.JoinHorizontal(lipgloss.Top, zeusCard, "  ", sparrowCard)
}

// ── Zeus / Sparrow / Macaroon / QR screens ───────────────

func (m Model) viewZeus() string {
    bw := wMin(m.width-4, wContentWidth)
    var lines []string
    lines = append(lines, wLightningStyle.Render("⚡ Zeus Wallet — LND REST over Tor"))
    lines = append(lines, "")
    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        lines = append(lines, wWarnStyle.Render("Not available yet."))
    } else {
        lines = append(lines, "  "+wLabelStyle.Render("Type: ")+wMonoStyle.Render("LND (REST)"))
        lines = append(lines, "  "+wLabelStyle.Render("Port: ")+wMonoStyle.Render("8080"))
        lines = append(lines, "")
        lines = append(lines, "  "+wLabelStyle.Render("Host:"))
        lines = append(lines, "  "+wMonoStyle.Render(restOnion))
        lines = append(lines, "")
        mac := readMacaroonHex(m.cfg)
        if mac != "" {
            preview := mac[:wMin(40, len(mac))] + "..."
            lines = append(lines, "  "+wLabelStyle.Render("Macaroon:"))
            lines = append(lines, "  "+wMonoStyle.Render(preview))
            lines = append(lines, "")
            lines = append(lines, "  "+wActionStyle.Render("[m] full macaroon    [r] QR code"))
        }
    }
    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render("1. Install Zeus, enable Tor"))
    lines = append(lines, wDimStyle.Render("2. Scan QR or add manually"))
    lines = append(lines, wDimStyle.Render("3. Paste host, port, macaroon"))

    box := wOuterBox.Width(bw).Padding(1, 2).Render(strings.Join(lines, "\n"))
    title := wTitleStyle.Width(bw).Align(lipgloss.Center).Render(" Zeus Wallet Setup ")
    footer := wFooterStyle.Render("  m macaroon • r QR • backspace back • q quit  ")
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, full)
}

func (m Model) viewSparrow() string {
    bw := wMin(m.width-4, wContentWidth)
    var lines []string
    lines = append(lines, wHeaderStyle.Render("Sparrow — Bitcoin Core RPC over Tor"))
    lines = append(lines, "")
    lines = append(lines, wWarningStyle.Render("WARNING: Cookie changes on restart."))
    lines = append(lines, "")
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC != "" {
        port := "8332"
        if !m.cfg.IsMainnet() {
            port = "48332"
        }
        cookie := readCookieValue(m.cfg)
        lines = append(lines, "  "+wLabelStyle.Render("Port: ")+wMonoStyle.Render(port))
        lines = append(lines, "  "+wLabelStyle.Render("User: ")+wMonoStyle.Render("__cookie__"))
        lines = append(lines, "")
        lines = append(lines, "  "+wLabelStyle.Render("URL:"))
        lines = append(lines, "  "+wMonoStyle.Render(btcRPC))
        lines = append(lines, "")
        if cookie != "" {
            lines = append(lines, "  "+wLabelStyle.Render("Password:"))
            lines = append(lines, "  "+wMonoStyle.Render(cookie))
        }
    }
    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render("1. Sparrow → Preferences → Server"))
    lines = append(lines, wDimStyle.Render("2. Bitcoin Core tab, enter details"))
    lines = append(lines, wDimStyle.Render("3. SOCKS5 proxy: localhost:9050"))

    box := wOuterBox.Width(bw).Padding(1, 2).Render(strings.Join(lines, "\n"))
    title := wTitleStyle.Width(bw).Align(lipgloss.Center).Render(" Sparrow Wallet Setup ")
    footer := wFooterStyle.Render("  backspace back • q quit  ")
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, full)
}

func (m Model) viewMacaroon() string {
    mac := readMacaroonHex(m.cfg)
    if mac == "" {
        mac = "Not available."
    }
    title := wLightningStyle.Render("⚡ Admin Macaroon (hex)")
    hint := wDimStyle.Render("Select and copy. Backspace to go back.")
    content := lipgloss.JoinVertical(lipgloss.Left, "", title, "", hint, "", mac, "")
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) viewQR() string {
    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    mac := readMacaroonHex(m.cfg)
    if restOnion == "" || mac == "" {
        return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
            wWarnStyle.Render("QR not available."))
    }
    uri := fmt.Sprintf("lndconnect://%s:8080?macaroon=%s",
        restOnion, hexToBase64URL(mac))
    qr := renderQRCode(uri)
    var lines []string
    lines = append(lines, wDimStyle.Render("Zoom out: Cmd+Minus / Ctrl+Minus"))
    if qr != "" {
        lines = append(lines, qr)
    }
    lines = append(lines, wFooterStyle.Render("backspace back • q quit"))
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top,
        lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// ── Logs tab ─────────────────────────────────────────────

func (m Model) viewLogs(bw int) string {
    var lines []string
    lines = append(lines, wHeaderStyle.Render("Select a service to view logs"))
    lines = append(lines, "")

    type svc struct {
        name string
        sel  logSelection
    }
    services := []svc{{"Tor", logSelTor}, {"Bitcoin Core", logSelBitcoin}}
    if m.cfg.HasLND() {
        services = append(services, svc{"LND", logSelLND})
    }
    if m.cfg.LITInstalled {
        services = append(services, svc{"Lightning Terminal", logSelLIT})
    }
    if m.cfg.SyncthingInstalled {
        services = append(services, svc{"Syncthing", logSelSyncthing})
    }

    for _, s := range services {
        prefix, style := "  ", wValueStyle
        if m.logSel == s.sel {
            prefix, style = "▸ ", wActionStyle
        }
        lines = append(lines, style.Render(prefix+s.name))
    }
    lines = append(lines, "")
    lines = append(lines, wDimStyle.Render("Enter to view • b to return"))

    return wOuterBox.Width(bw).Padding(1, 2).Render(padLines(lines, wBoxHeight))
}

// ── Software tab ─────────────────────────────────────────

func (m Model) viewSoftware(bw int) string {
    halfW := (bw - 4) / 2
    cardH := wBoxHeight

    // Syncthing card
    var syncLines []string
    syncLines = append(syncLines, wHeaderStyle.Render("Syncthing"))
    syncLines = append(syncLines, "")
    syncLines = append(syncLines, wDimStyle.Render("File sync & auto-backup"))
    syncLines = append(syncLines, wDimStyle.Render("LND channel state."))
    syncLines = append(syncLines, "")

    if m.cfg.SyncthingInstalled {
        syncLines = append(syncLines, wGreenDotStyle.Render("●")+" "+wGoodStyle.Render("Installed"))
        syncLines = append(syncLines, "")
        syncOnion := readOnion("/var/lib/tor/syncthing/hostname")
        if syncOnion != "" {
            syncLines = append(syncLines, wLabelStyle.Render("Address:"))
            syncLines = append(syncLines, "  "+wMonoStyle.Render(syncOnion))
            syncLines = append(syncLines, wLabelStyle.Render("Port: ")+wMonoStyle.Render("8384"))
        }
        if m.cfg.SyncthingPassword != "" {
            syncLines = append(syncLines, "")
            syncLines = append(syncLines, wLabelStyle.Render("User: ")+wMonoStyle.Render("admin"))
            syncLines = append(syncLines, wLabelStyle.Render("Pass: ")+wMonoStyle.Render(m.cfg.SyncthingPassword))
        }
        syncLines = append(syncLines, "")
        syncLines = append(syncLines, wActionStyle.Render("Select for full URL ▸"))
    } else if !m.cfg.HasLND() || !m.cfg.WalletExists() {
        syncLines = append(syncLines, wGrayedStyle.Render("Requires LND + wallet"))
    } else {
        syncLines = append(syncLines, wRedDotStyle.Render("●")+" "+wDimStyle.Render("Not installed"))
        syncLines = append(syncLines, "")
        syncLines = append(syncLines, wActionStyle.Render("Select to install ▸"))
    }

    sBorder := wNormalBorder
    if m.pairingFocus == 0 {
        if m.cfg.HasLND() && m.cfg.WalletExists() {
            sBorder = wSelectedBorder
        } else {
            sBorder = wGrayedBorder
        }
    }
    syncCard := sBorder.Width(halfW).Padding(1, 2).Render(padLines(syncLines, cardH))

    // LIT card
    var litLines []string
    litLines = append(litLines, wHeaderStyle.Render("Lightning Terminal"))
    litLines = append(litLines, "")
    litLines = append(litLines, wDimStyle.Render("Browser UI for channel"))
    litLines = append(litLines, wDimStyle.Render("liquidity management."))
    litLines = append(litLines, "")
    litLines = append(litLines, wLabelStyle.Render("Version: ")+
        wValueStyle.Render("v"+installer.LitVersionStr()))
    litLines = append(litLines, "")

    if m.cfg.LITInstalled {
        litLines = append(litLines, wGreenDotStyle.Render("●")+" "+wGoodStyle.Render("Installed"))
        litLines = append(litLines, "")
        litLines = append(litLines, wDimStyle.Render("See Lightning card"))
        litLines = append(litLines, "")
        litLines = append(litLines, wActionStyle.Render("Select for full URL ▸"))
    } else if !m.cfg.HasLND() || !m.cfg.WalletExists() {
        litLines = append(litLines, wGrayedStyle.Render("Requires LND + wallet"))
    } else {
        litLines = append(litLines, wRedDotStyle.Render("●")+" "+wDimStyle.Render("Not installed"))
        litLines = append(litLines, "")
        litLines = append(litLines, wActionStyle.Render("Select to install ▸"))
    }

    lBorder := wNormalBorder
    if m.pairingFocus == 1 {
        if m.cfg.HasLND() && m.cfg.WalletExists() {
            lBorder = wSelectedBorder
        } else {
            lBorder = wGrayedBorder
        }
    }
    litCard := lBorder.Width(halfW).Padding(1, 2).Render(padLines(litLines, cardH))

    return lipgloss.JoinHorizontal(lipgloss.Top, syncCard, "  ", litCard)
}

// ── Shell actions ────────────────────────────────────────

func runSystemUpdate() {
    fmt.Print("\033[2J\033[H")
    fmt.Println("\n  ═══════════════════════════════════════════")
    fmt.Println("    System Update")
    fmt.Println("  ═══════════════════════════════════════════\n")
    fmt.Println("  Running apt update && apt upgrade...")
    fmt.Println()

    exec.Command("apt-get", "update").Run()
    cmd := exec.Command("apt-get", "upgrade", "-y")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    fmt.Println("\n  ✓ Update complete")
    if _, err := os.Stat("/var/run/reboot-required"); err == nil {
        fmt.Println("\n  ⚠ Reboot required.")
        fmt.Print("  Reboot now? [y/N]: ")
        var ans string
        fmt.Scanln(&ans)
        if strings.ToLower(ans) == "y" {
            exec.Command("reboot").Run()
        }
    }
    fmt.Print("\n  Press Enter to return...")
    fmt.Scanln()
}

func runLogViewer(sel logSelection, cfg *config.AppConfig) {
    svcMap := map[logSelection]string{
        logSelTor: "tor", logSelBitcoin: "bitcoind",
        logSelLND: "lnd", logSelLIT: "litd",
        logSelSyncthing: "syncthing",
    }
    svc := svcMap[sel]
    fmt.Print("\033[2J\033[H")
    fmt.Printf("\n  ═══════════════════════════════════════════\n")
    fmt.Printf("    %s Logs (last 100 lines)\n", svc)
    fmt.Printf("  ═══════════════════════════════════════════\n\n")

    cmd := exec.Command("journalctl", "-u", svc, "-n", "100", "--no-pager")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    fmt.Print("\n  Press Enter to return...")
    fmt.Scanln()
}

// ── QR / Base64 ──────────────────────────────────────────

func renderQRCode(data string) string {
    qr, err := qrcode.New(data, qrcode.Low)
    if err != nil {
        return ""
    }
    bm := qr.Bitmap()
    rows, cols := len(bm), len(bm[0])
    var b strings.Builder
    for y := 0; y < rows; y += 2 {
        for x := 0; x < cols; x++ {
            top := bm[y][x]
            bot := y+1 < rows && bm[y+1][x]
            switch {
            case top && bot:
                b.WriteString("█")
            case top:
                b.WriteString("▀")
            case bot:
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
    return base64.RawURLEncoding.EncodeToString(data)
}

// ── LND queries ──────────────────────────────────────────

func getLNDBalance(cfg *config.AppConfig) string {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, "sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd", "--network="+cfg.Network, "walletbalance")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "total_balance")
}

func getLNDChannelCount(cfg *config.AppConfig) string {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, "sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd", "--network="+cfg.Network, "getinfo")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "num_active_channels")
}

func getLNDPubkey(cfg *config.AppConfig) string {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, "sudo", "-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd", "--network="+cfg.Network, "getinfo")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return extractJSON(string(out), "identity_pubkey")
}

// ── Helpers ──────────────────────────────────────────────

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
    data, err := os.ReadFile(fmt.Sprintf(
        "/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon", network))
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
    out, _ := cmd.CombinedOutput()
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
    data, _ := os.ReadFile("/proc/meminfo")
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
    return fmtKB(total), fmtKB(used), fmt.Sprintf("%.0f%%",
        float64(used)/float64(total)*100)
}

func dirSize(path string) string {
    out, err := exec.Command("du", "-sh", path).CombinedOutput()
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