package welcome

import (
    "github.com/charmbracelet/lipgloss"

    "github.com/ripsline/virtual-private-node/internal/installer"
    "github.com/ripsline/virtual-private-node/internal/theme"
)

func (m Model) viewAddons(bw int) string {
    thirdW := (bw - 8) / 3
    cardH := theme.BoxHeight

    lndCard := m.addonLNDCard(thirdW, cardH)
    syncCard := m.addonSyncthingCard(thirdW, cardH)
    litCard := m.addonLITCard(thirdW, cardH)

    return lipgloss.JoinHorizontal(lipgloss.Top,
        lndCard, "  ", syncCard, "  ", litCard)
}

func (m Model) addonLNDCard(w, h int) string {
    var lines []string
    lines = append(lines, theme.Lightning.Render("⚡️ LND"))
    lines = append(lines, "")
    lines = append(lines, theme.Dim.Render("Lightning Network Daemon"))
    lines = append(lines, theme.Dim.Render("v"+installer.LndVersionStr()))
    lines = append(lines, "")

    if m.cfg.HasLND() {
        lines = append(lines, theme.GreenDot.Render("●")+" "+theme.Good.Render("Installed"))
        if m.cfg.WalletExists() {
            lines = append(lines, theme.Label.Render("Wallet: ")+theme.Good.Render("created"))
        } else {
            lines = append(lines, theme.Label.Render("Wallet: ")+theme.Warning.Render("not created"))
        }
    } else {
        lines = append(lines, theme.RedDot.Render("●")+" "+theme.Dim.Render("Not installed"))
        lines = append(lines, "")
        lines = append(lines, theme.Action.Render("Select to install ▸"))
    }

    border := theme.NormalBorder
    if m.addonFocus == 0 {
        border = theme.SelectedBorder
    }
    return border.Width(w).Padding(1, 2).Render(padLines(lines, h))
}

func (m Model) addonSyncthingCard(w, h int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("🔄 Syncthing"))
    lines = append(lines, "")
    lines = append(lines, theme.Dim.Render("File sync & auto-backup"))
    lines = append(lines, theme.Dim.Render("LND channel state."))
    lines = append(lines, "")

    if m.cfg.SyncthingInstalled {
        lines = append(lines, theme.GreenDot.Render("●")+" "+theme.Good.Render("Installed"))
        if m.cfg.SyncthingPassword != "" {
            lines = append(lines, theme.Label.Render("User: ")+theme.Mono.Render("admin"))
            lines = append(lines, theme.Label.Render("Pass: ")+theme.Mono.Render(m.cfg.SyncthingPassword))
        }
        lines = append(lines, "")
        lines = append(lines, theme.Dim.Render("Open in Tor Browser"))
        lines = append(lines, theme.Action.Render("Select for full URL ▸"))
    } else if !m.cfg.HasLND() || !m.cfg.WalletExists() {
        lines = append(lines, theme.Grayed.Render("Requires LND + wallet"))
    } else {
        lines = append(lines, theme.RedDot.Render("●")+" "+theme.Dim.Render("Not installed"))
        lines = append(lines, "")
        lines = append(lines, theme.Action.Render("Select to install ▸"))
    }

    border := theme.NormalBorder
    if m.addonFocus == 1 {
        if m.cfg.HasLND() && m.cfg.WalletExists() {
            border = theme.SelectedBorder
        } else if !m.cfg.SyncthingInstalled {
            border = theme.GrayedBorder
        } else {
            border = theme.SelectedBorder
        }
    }
    return border.Width(w).Padding(1, 2).Render(padLines(lines, h))
}

func (m Model) addonLITCard(w, h int) string {
    var lines []string
    lines = append(lines, theme.Lightning.Render("⚡️ Lightning Terminal"))
    lines = append(lines, "")
    lines = append(lines, theme.Dim.Render("Browser UI for channel"))
    lines = append(lines, theme.Dim.Render("liquidity management."))
    lines = append(lines, "")

    if m.cfg.LITInstalled {
        lines = append(lines, theme.GreenDot.Render("●")+" "+theme.Good.Render("Installed"))
        lines = append(lines, theme.Label.Render("Version: ")+
            theme.Value.Render("v"+installer.LitVersionStr()))
        if m.cfg.LITPassword != "" {
            lines = append(lines, "")
            lines = append(lines, theme.Label.Render("Password:"))
            lines = append(lines, theme.Mono.Render(m.cfg.LITPassword))
        }
        lines = append(lines, "")
        lines = append(lines, theme.Dim.Render("Open in Tor Browser."))
        lines = append(lines, theme.Dim.Render("Ignore security warning:"))
        lines = append(lines, theme.Dim.Render("Advanced → Accept Risk."))
        lines = append(lines, "")
        lines = append(lines, theme.Action.Render("Select for full URL ▸"))
    } else if !m.cfg.HasLND() || !m.cfg.WalletExists() {
        lines = append(lines, theme.Grayed.Render("Requires LND + wallet"))
    } else {
        lines = append(lines, theme.RedDot.Render("●")+" "+theme.Dim.Render("Not installed"))
        lines = append(lines, "")
        lines = append(lines, theme.Action.Render("Select to install ▸"))
    }

    border := theme.NormalBorder
    if m.addonFocus == 2 {
        if m.cfg.HasLND() && m.cfg.WalletExists() {
            border = theme.SelectedBorder
        } else if !m.cfg.LITInstalled {
            border = theme.GrayedBorder
        } else {
            border = theme.SelectedBorder
        }
    }
    return border.Width(w).Padding(1, 2).Render(padLines(lines, h))
}