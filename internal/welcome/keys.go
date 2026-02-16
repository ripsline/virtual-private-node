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
    case latestVersionMsg:
        m.latestVersion = string(msg)
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
                if m.urlTarget != "" {
                    // Return to whichever detail screen launched the URL
                    m.subview = svNone
                }
                m.subview = svNone
            case svSyncthingDetail:
                m.subview = svNone
            case svLITDetail:
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
        case "u":
            if m.subview == svSyncthingDetail {
                syncOnion := readOnion("/var/lib/tor/syncthing/hostname")
                if syncOnion != "" {
                    m.urlTarget = "http://" + syncOnion + ":8384"
                    m.subview = svFullURL
                }
                return m, nil
            }
            if m.subview == svLITDetail {
                litOnion := readOnion("/var/lib/tor/lnd-lit/hostname")
                if litOnion != "" {
                    m.urlTarget = "https://" + litOnion + ":8443"
                    m.subview = svFullURL
                }
                return m, nil
            }
        }
        return m, nil
    }

    // Inside a card
    if m.cardActive {
        return m.handleCardKey(key)
    }

    // Settings tab handles its own keys
    if m.activeTab == tabSettings {
        switch key {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "tab":
            m.activeTab = (m.activeTab + 1) % 4
            m.settingsCustom = false
            m.settingsConfirm = ""
            m.updateConfirm = false
            return m, nil
        case "shift+tab":
            m.activeTab = (m.activeTab + 3) % 4
            m.settingsCustom = false
            m.settingsConfirm = ""
            m.updateConfirm = false
            return m, nil
        default:
            m = handleSettingsKey(m, key)
            return m, nil
        }
    }

    // Main navigation
    switch key {
    case "q", "ctrl+c":
        return m, tea.Quit
    case "tab":
        m.activeTab = (m.activeTab + 1) % 4
        m.cardActive = false
        m.svcConfirm = ""
        return m, nil
    case "shift+tab":
        m.activeTab = (m.activeTab + 3) % 4
        m.cardActive = false
        m.svcConfirm = ""
        return m, nil
    case "1":
        m.activeTab = tabDashboard
    case "2":
        m.activeTab = tabPairing
    case "3":
        m.activeTab = tabAddons
    case "4":
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
        m.activeTab = (m.activeTab + 1) % 4
        m.cardActive = false
        m.svcConfirm = ""
        m.sysConfirm = ""
        return m, nil
    case "shift+tab":
        m.activeTab = (m.activeTab + 3) % 4
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
        case "l":
            m.logSvcName = m.svcName(m.svcCursor)
            m.shellAction = svLogView
            m.cardActive = false
            return m, tea.Quit
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
        if m.dashCard == cardBitcoin {
            m.dashCard = cardServices
        } else if m.dashCard == cardLightning {
            m.dashCard = cardSystem
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
    case tabPairing:
        m.pairingFocus = 0
    case tabAddons:
        m.addonFocus = 0
    case tabSettings:
        m.settingsFocus = 0
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
    case tabPairing:
        m.pairingFocus = 1
    case tabAddons:
        m.addonFocus = 1
    case tabSettings:
        m.settingsFocus = 1
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
                m.shellAction = svLNDInstall
                return m, tea.Quit
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
    case tabAddons:
        return m.handleAddonEnter()
    case tabSettings:
        // Handled by handleSettingsKey
    }
    return m, nil
}

func (m Model) handleAddonEnter() (tea.Model, tea.Cmd) {
    switch m.addonFocus {
    case 0: // Syncthing
        if m.cfg.SyncthingInstalled {
            m.subview = svSyncthingDetail
            return m, nil
        }
        if !m.cfg.HasLND() || !m.cfg.WalletExists() {
            return m, nil
        }
        m.shellAction = svSyncthingInstall
        return m, tea.Quit
    case 1: // LIT
        if m.cfg.LITInstalled {
            m.subview = svLITDetail
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