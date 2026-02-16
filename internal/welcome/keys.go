package welcome

import (
    "os/exec"

    tea "github.com/charmbracelet/bubbletea"

    "github.com/ripsline/virtual-private-node/internal/system"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        return m.handleKey(msg)
    case svcActionDoneMsg:
        return m, fetchStatus(m.cfg)
    case statusMsg:
        m.status = &msg
        return m, nil
    case tickMsg:
        return m, tea.Batch(
            fetchStatus(m.cfg),
            tickEvery(5e9),
        )
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
            case svQR:
                m.subview = svZeus
            case svFullURL:
                m.subview = svNone
            default:
                m.subview = svNone
            }
            return m, nil
        case "m":
            if m.subview == svZeus {
                m.shellAction = svMacaroonShell
                return m, tea.Quit
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
        m.activeTab = (m.activeTab + 1) % 5
        m.cardActive = false
        m.svcConfirm = ""
        return m, nil
    case "shift+tab":
        m.activeTab = (m.activeTab + 4) % 5
        m.cardActive = false
        m.svcConfirm = ""
        return m, nil
    case "1":
        m.activeTab = tabDashboard
    case "2":
        m.activeTab = tabPairing
    case "3":
        m.activeTab = tabLogs
    case "4":
        m.activeTab = tabAddons
    case "5":
        m.activeTab = tabSettings
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
        m.sysConfirm = ""
        return m, nil
    case "q":
        return m, tea.Quit
    case "tab":
        m.activeTab = (m.activeTab + 1) % 5
        m.cardActive = false
        m.svcConfirm = ""
        m.sysConfirm = ""
        return m, nil
    case "shift+tab":
        m.activeTab = (m.activeTab + 4) % 5
        m.cardActive = false
        m.svcConfirm = ""
        m.sysConfirm = ""
        return m, nil
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
        if m.sysConfirm != "" {
            switch key {
            case "y":
                action := m.sysConfirm
                m.sysConfirm = ""
                if action == "update" {
                    m.shellAction = svSystemUpdate
                    return m, tea.Quit
                }
                if action == "reboot" {
                    return m, func() tea.Msg {
                        system.Run("reboot")
                        return svcActionDoneMsg{}
                    }
                }
            default:
                m.sysConfirm = ""
                return m, nil
            }
            return m, nil
        }
        switch key {
        case "u":
            m.sysConfirm = "update"
        case "r":
            if m.status != nil && m.status.rebootRequired {
                m.sysConfirm = "reboot"
            }
        }
    }

    return m, nil
}

func (m Model) navUp() Model {
    switch m.activeTab {
    case tabDashboard:
        cards := m.visibleCards()
        m.dashCard = gridUp(cards, m.dashCard)
    case tabLogs:
        avail := m.availableLogs()
        for i, sel := range avail {
            if sel == m.logSel && i > 0 {
                m.logSel = avail[i-1]
                break
            }
        }
    }
    return m
}

func (m Model) navDown() Model {
    switch m.activeTab {
    case tabDashboard:
        cards := m.visibleCards()
        m.dashCard = gridDown(cards, m.dashCard)
    case tabLogs:
        avail := m.availableLogs()
        for i, sel := range avail {
            if sel == m.logSel && i+1 < len(avail) {
                m.logSel = avail[i+1]
                break
            }
        }
    }
    return m
}

func (m Model) navLeft() Model {
    switch m.activeTab {
    case tabDashboard:
        cards := m.visibleCards()
        m.dashCard = gridLeft(cards, m.dashCard)
    case tabPairing:
        m.pairingFocus = 0
    case tabAddons:
        m.addonFocus = 0
    }
    return m
}

func (m Model) navRight() Model {
    switch m.activeTab {
    case tabDashboard:
        cards := m.visibleCards()
        m.dashCard = gridRight(cards, m.dashCard)
    case tabPairing:
        m.pairingFocus = 1
    case tabAddons:
        maxFocus := m.addonMaxFocus()
        if m.addonFocus < maxFocus {
            m.addonFocus++
        }
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
        if m.pairingFocus == 0 && m.cfg.HasLND() && m.cfg.WalletExists() {
            m.subview = svZeus
        } else if m.pairingFocus == 1 {
            m.subview = svSparrow
        }
    case tabLogs:
        m.shellAction = svLogView
        return m, tea.Quit
    case tabAddons:
        return m.handleAddonEnter()
    case tabSettings:
        m.shellAction = svPruneChange
        return m, tea.Quit
    }
    return m, nil
}

func (m Model) handleAddonEnter() (tea.Model, tea.Cmd) {
    switch m.addonFocus {
    case 0: // LND
        if m.cfg.HasLND() {
            return m, nil // already installed
        }
        m.shellAction = svLNDInstall
        return m, tea.Quit
    case 1: // Syncthing
        if m.cfg.SyncthingInstalled {
            syncOnion := readOnion("/var/lib/tor/syncthing/hostname")
            if syncOnion != "" {
                m.urlTarget = "http://" + syncOnion + ":8384"
                m.subview = svFullURL
            }
            return m, nil
        }
        if !m.cfg.HasLND() || !m.cfg.WalletExists() {
            return m, nil
        }
        m.shellAction = svSyncthingInstall
        return m, tea.Quit
    case 2: // LIT
        if m.cfg.LITInstalled {
            litOnion := readOnion("/var/lib/tor/lnd-lit/hostname")
            if litOnion != "" {
                m.urlTarget = "https://" + litOnion + ":8443"
                m.subview = svFullURL
            }
            return m, nil
        }
        if !m.cfg.HasLND() || !m.cfg.WalletExists() {
            return m, nil
        }
        m.shellAction = svLITInstall
        return m, tea.Quit
    }
    return m, nil
}

func (m Model) addonMaxFocus() int {
    // LND, Syncthing, LIT
    return 2
}

// ── Grid navigation helpers ──────────────────────────────

func (m Model) visibleCards() []cardPos {
    cards := []cardPos{cardServices, cardSystem, cardBitcoin}
    if m.cfg.HasLND() {
        cards = append(cards, cardLightning)
    }
    if m.cfg.SyncthingInstalled {
        cards = append(cards, cardSyncthing)
    }
    if m.cfg.LITInstalled {
        cards = append(cards, cardLIT)
    }
    return cards
}

func gridIndex(cards []cardPos, pos cardPos) int {
    for i, c := range cards {
        if c == pos {
            return i
        }
    }
    return 0
}

func gridUp(cards []cardPos, pos cardPos) cardPos {
    idx := gridIndex(cards, pos)
    // 2-column grid: up means -2
    if idx >= 2 {
        return cards[idx-2]
    }
    return pos
}

func gridDown(cards []cardPos, pos cardPos) cardPos {
    idx := gridIndex(cards, pos)
    if idx+2 < len(cards) {
        return cards[idx+2]
    }
    return pos
}

func gridLeft(cards []cardPos, pos cardPos) cardPos {
    idx := gridIndex(cards, pos)
    // Left from even column (right side) to odd (left side)
    if idx%2 == 1 {
        return cards[idx-1]
    }
    return pos
}

func gridRight(cards []cardPos, pos cardPos) cardPos {
    idx := gridIndex(cards, pos)
    if idx%2 == 0 && idx+1 < len(cards) {
        return cards[idx+1]
    }
    return pos
}

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

func (m Model) availableLogs() []logSelection {
    avail := []logSelection{logSelTor, logSelBitcoin}
    if m.cfg.HasLND() {
        avail = append(avail, logSelLND)
    }
    if m.cfg.LITInstalled {
        avail = append(avail, logSelLIT)
    }
    if m.cfg.SyncthingInstalled {
        avail = append(avail, logSelSyncthing)
    }
    return avail
}