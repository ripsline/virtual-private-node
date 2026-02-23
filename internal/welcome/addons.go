// internal/welcome/addons.go

package welcome

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ripsline/virtual-private-node/internal/installer"
	"github.com/ripsline/virtual-private-node/internal/paths"
	"github.com/ripsline/virtual-private-node/internal/theme"
)

func (m Model) viewAddons(bw int) string {
	halfW := (bw - 2) / 2
	cardH := theme.BoxHeight

	syncCard := m.addonSyncthingCard(halfW, cardH)
	litCard := m.addonLITCard(halfW, cardH)

	return lipgloss.JoinHorizontal(lipgloss.Top, syncCard, "  ", litCard)
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
		lines = append(lines, theme.Label.Render("Version: ")+
			theme.Value.Render(getSyncthingVersion()))
		lines = append(lines, "")
		lines = append(lines, theme.Action.Render("Select for details ▸"))
	} else if !m.cfg.HasLND() || !m.cfg.WalletExists() {
		lines = append(lines, theme.Grayed.Render("Requires LND + wallet"))
	} else {
		lines = append(lines, theme.RedDot.Render("●")+" "+theme.Dim.Render("Not installed"))
		lines = append(lines, "")
		lines = append(lines, theme.Action.Render("Select to install ▸"))
	}

	border := theme.NormalBorder
	if m.addonFocus == 0 {
		if (m.cfg.HasLND() && m.cfg.WalletExists()) || m.cfg.SyncthingInstalled {
			border = theme.SelectedBorder
		} else {
			border = theme.GrayedBorder
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
		lines = append(lines, "")
		lines = append(lines, theme.Action.Render("Select for details ▸"))
	} else if !m.cfg.HasLND() || !m.cfg.WalletExists() {
		lines = append(lines, theme.Grayed.Render("Requires LND + wallet"))
	} else {
		lines = append(lines, theme.RedDot.Render("●")+" "+theme.Dim.Render("Not installed"))
		lines = append(lines, "")
		lines = append(lines, theme.Action.Render("Select to install ▸"))
	}

	border := theme.NormalBorder
	if m.addonFocus == 1 {
		if (m.cfg.HasLND() && m.cfg.WalletExists()) || m.cfg.LITInstalled {
			border = theme.SelectedBorder
		} else {
			border = theme.GrayedBorder
		}
	}
	return border.Width(w).Padding(1, 2).Render(padLines(lines, h))
}

func (m Model) viewSyncthingDetail() string {
	bw := min(m.width-4, theme.ContentWidth)
	var lines []string
	lines = append(lines, theme.Header.Render("🔄 Syncthing — File Sync over Tor"))
	lines = append(lines, "")

	syncOnion := readOnion(paths.TorSyncthingHostname)
	if syncOnion == "" {
		lines = append(lines, theme.Warn.Render("Tor address not available yet."))
	} else {
		addr := syncOnion + ":8384"
		lines = append(lines, "  "+theme.Label.Render("URL:"))
		if len(addr) > 55 {
			lines = append(lines, "  "+theme.Mono.Render(addr[:55]+"..."))
		} else {
			lines = append(lines, "  "+theme.Mono.Render(addr))
		}
		lines = append(lines, "")
		lines = append(lines, "  "+theme.Label.Render("User: ")+theme.Mono.Render("admin"))
		if m.cfg.SyncthingPassword != "" {
			lines = append(lines, "  "+theme.Label.Render("Pass: ")+theme.Mono.Render(m.cfg.SyncthingPassword))
		}
		lines = append(lines, "")
		lines = append(lines, "  "+theme.Action.Render("[u] full Tor Browser URL"))
	}

	lines = append(lines, "")
	lines = append(lines, theme.Dim.Render("  1. Open Tor Browser"))
	lines = append(lines, theme.Dim.Render("  2. Paste URL above"))
	lines = append(lines, theme.Dim.Render("  3. Login with user and password above"))
	lines = append(lines, theme.Dim.Render("  4. Pair your local Syncthing instance"))

	box := theme.Box.Width(bw).Padding(1, 2).Render(strings.Join(lines, "\n"))
	title := theme.Title.Width(bw).Align(lipgloss.Center).Render(" 🔄 Syncthing Details ")
	footer := theme.Footer.Render("  u full URL • backspace back • q quit  ")
	full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}

func (m Model) viewLITDetail() string {
	bw := min(m.width-4, theme.ContentWidth)
	var lines []string
	lines = append(lines, theme.Lightning.Render("⚡️ Lightning Terminal — Web UI over Tor"))
	lines = append(lines, "")

	litOnion := readOnion(paths.TorLNDLITHostname)
	if litOnion == "" {
		lines = append(lines, theme.Warn.Render("Tor address not available yet."))
	} else {
		addr := litOnion + ":8443"
		lines = append(lines, "  "+theme.Label.Render("URL:"))
		if len(addr) > 55 {
			lines = append(lines, "  "+theme.Mono.Render(addr[:55]+"..."))
		} else {
			lines = append(lines, "  "+theme.Mono.Render(addr))
		}
		lines = append(lines, "")
		if m.cfg.LITPassword != "" {
			lines = append(lines, "  "+theme.Label.Render("Password: ")+theme.Mono.Render(m.cfg.LITPassword))
		}
		lines = append(lines, "")
		lines = append(lines, "  "+theme.Action.Render("[u] full Tor Browser URL"))
	}

	lines = append(lines, "")
	lines = append(lines, theme.Dim.Render("  1. Open Tor Browser"))
	lines = append(lines, theme.Dim.Render("  2. Paste URL above"))
	lines = append(lines, theme.Dim.Render("  3. Ignore security warning"))
	lines = append(lines, theme.Dim.Render("     Advanced → Accept Risk & Continue"))
	lines = append(lines, theme.Dim.Render("     Connection is encrypted by Tor."))
	lines = append(lines, theme.Dim.Render("  4. Login with password above"))

	box := theme.Box.Width(bw).Padding(1, 2).Render(strings.Join(lines, "\n"))
	title := theme.Title.Width(bw).Align(lipgloss.Center).Render(" ⚡️ Lightning Terminal Details ")
	footer := theme.Footer.Render("  u full URL • backspace back • q quit  ")
	full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}
