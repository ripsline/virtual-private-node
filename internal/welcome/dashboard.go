package welcome

import (
    "fmt"

    "github.com/charmbracelet/lipgloss"

    "github.com/ripsline/virtual-private-node/internal/bitcoin"
    "github.com/ripsline/virtual-private-node/internal/theme"
)

func (m Model) viewDashboard(bw int) string {
    cards := m.visibleCards()
    halfW := (bw - 4) / 2
    cardH := theme.BoxHeight / 2

    var rows []string
    for i := 0; i < len(cards); i += 2 {
        left := m.renderCard(cards[i], halfW, cardH)
        if i+1 < len(cards) {
            right := m.renderCard(cards[i+1], halfW, cardH)
            rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
        } else {
            rows = append(rows, left)
        }
    }
    return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderCard(pos cardPos, w, h int) string {
    switch pos {
    case cardServices:
        return m.cardServicesView(w, h)
    case cardSystem:
        return m.cardSystemView(w, h)
    case cardBitcoin:
        return m.cardBitcoinView(w, h)
    case cardLightning:
        return m.cardLightningView(w, h)
    case cardSyncthing:
        return m.cardSyncthingView(w, h)
    case cardLIT:
        return m.cardLITView(w, h)
    }
    return ""
}

func (m Model) getBorder(pos cardPos, enabled bool) lipgloss.Style {
    if !enabled {
        return theme.GrayedBorder
    }
    if m.activeTab == tabDashboard && m.dashCard == pos {
        return theme.SelectedBorder
    }
    return theme.NormalBorder
}

func (m Model) cardServicesView(w, h int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("Services"))
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
        dot := theme.RedDot.Render("●")
        if m.status != nil {
            if active, ok := m.status.services[name]; ok && active {
                dot = theme.GreenDot.Render("●")
            }
        }
        prefix := "  "
        style := theme.Value
        if m.cardActive && m.dashCard == cardServices && m.svcCursor == i {
            prefix = "▸ "
            style = theme.Action
        }
        lines = append(lines, prefix+dot+" "+style.Render(name))
    }

    if m.cardActive && m.dashCard == cardServices {
        lines = append(lines, "")
        if m.svcConfirm != "" {
            svc := m.svcName(m.svcCursor)
            lines = append(lines, theme.Warning.Render(
                fmt.Sprintf("%s %s? [y/n]", m.svcConfirm, svc)))
        } else {
            lines = append(lines, theme.Dim.Render("[r]estart [s]top [a]start"))
        }
    }

    return m.getBorder(cardServices, true).Width(w).
        Padding(0, 1).Render(padLines(lines, h))
}

func (m Model) cardSystemView(w, h int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("System"))
    lines = append(lines, "")

    if m.status != nil {
        lines = append(lines, theme.Label.Render("Disk: ")+
            theme.Value.Render(fmt.Sprintf("%s / %s (%s)",
                m.status.diskUsed, m.status.diskTotal, m.status.diskPct)))
        lines = append(lines, theme.Label.Render("RAM:  ")+
            theme.Value.Render(fmt.Sprintf("%s / %s (%s)",
                m.status.ramUsed, m.status.ramTotal, m.status.ramPct)))
        lines = append(lines, theme.Label.Render("Bitcoin: ")+
            theme.Value.Render(m.status.btcSize))
        if m.cfg.HasLND() {
            lines = append(lines, theme.Label.Render("LND: ")+
                theme.Value.Render(m.status.lndSize))
        }
    } else {
        lines = append(lines, theme.Dim.Render("Loading..."))
    }

    if m.cardActive && m.dashCard == cardSystem {
        lines = append(lines, "")
        if m.sysConfirm != "" {
            lines = append(lines, theme.Warning.Render(
                fmt.Sprintf("%s system? [y/n]", m.sysConfirm)))
        } else {
            lines = append(lines, theme.Action.Render("[u]pdate packages"))
            if m.status != nil && m.status.rebootRequired {
                lines = append(lines, theme.Warning.Render("⚠️ Reboot required"))
                lines = append(lines, theme.Action.Render("[r]eboot"))
            }
        }
    } else if m.status != nil && m.status.rebootRequired {
        lines = append(lines, "")
        lines = append(lines, theme.Warning.Render("⚠️ Reboot required"))
    }

    return m.getBorder(cardSystem, true).Width(w).
        Padding(0, 1).Render(padLines(lines, h))
}

func (m Model) cardBitcoinView(w, h int) string {
    var lines []string
    lines = append(lines, theme.Bitcoin.Render("₿ Bitcoin"))
    lines = append(lines, "")

    if m.status == nil {
        lines = append(lines, theme.Dim.Render("Loading..."))
    } else if !m.status.btcResponding {
        lines = append(lines, theme.Warn.Render("Not responding"))
    } else {
        if m.status.btcSynced {
            lines = append(lines, theme.Label.Render("Sync: ")+
                theme.Good.Render("✅ synced"))
        } else {
            lines = append(lines, theme.Label.Render("Sync: ")+
                theme.Warn.Render("🔄 syncing"))
        }
        lines = append(lines, theme.Label.Render("Height: ")+
            theme.Value.Render(fmt.Sprintf("%d / %d",
                m.status.btcBlocks, m.status.btcHeaders)))
        if m.status.btcProgress > 0 {
            lines = append(lines, theme.Label.Render("Progress: ")+
                theme.Value.Render(bitcoin.FormatProgress(m.status.btcProgress)))
        }
        lines = append(lines, theme.Label.Render("Network: ")+
            theme.Value.Render(m.cfg.Network))
    }

    return m.getBorder(cardBitcoin, true).Width(w).
        Padding(0, 1).Render(padLines(lines, h))
}

func (m Model) cardLightningView(w, h int) string {
    var lines []string
    lines = append(lines, theme.Lightning.Render("⚡️ Lightning"))
    lines = append(lines, "")

    if !m.cfg.WalletExists() {
        lines = append(lines, theme.Label.Render("Wallet: ")+
            theme.Warning.Render("not created"))
        lines = append(lines, "")
        lines = append(lines, theme.Action.Render("Select to create ▸"))
    } else {
        lines = append(lines, theme.Label.Render("Wallet: ")+
            theme.Good.Render("created"))
        if m.cfg.AutoUnlock {
            lines = append(lines, theme.Label.Render("Auto-unlock: ")+
                theme.Good.Render("enabled"))
        }
        lines = append(lines, "")
        lines = append(lines, theme.Action.Render("Select for details ▸"))
    }

    return m.getBorder(cardLightning, true).Width(w).
        Padding(0, 1).Render(padLines(lines, h))
}

func (m Model) cardSyncthingView(w, h int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("🔄 Syncthing"))
    lines = append(lines, "")
    lines = append(lines, theme.GreenDot.Render("●")+" "+theme.Good.Render("Running"))
    if m.cfg.SyncthingPassword != "" {
        lines = append(lines, theme.Label.Render("User: ")+theme.Mono.Render("admin"))
    }
    return m.getBorder(cardSyncthing, true).Width(w).
        Padding(0, 1).Render(padLines(lines, h))
}

func (m Model) cardLITView(w, h int) string {
    var lines []string
    lines = append(lines, theme.Lightning.Render("⚡️ Lightning Terminal"))
    lines = append(lines, "")
    lines = append(lines, theme.GreenDot.Render("●")+" "+theme.Good.Render("Running"))
    return m.getBorder(cardLIT, true).Width(w).
        Padding(0, 1).Render(padLines(lines, h))
}