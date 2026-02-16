package welcome

import (
    "strings"

    "github.com/charmbracelet/lipgloss"

    "github.com/ripsline/virtual-private-node/internal/lnd"
    "github.com/ripsline/virtual-private-node/internal/theme"
)

func (m Model) viewLightning() string {
    bw := min(m.width-4, theme.ContentWidth)
    var lines []string
    lines = append(lines, theme.Lightning.Render("⚡️ Lightning Node"))
    lines = append(lines, "")

    if m.cfg.WalletExists() {
        lines = append(lines, "  "+theme.Label.Render("Status: ")+
            theme.Good.Render("created"))
        if m.cfg.AutoUnlock {
            lines = append(lines, "  "+theme.Label.Render("Auto-unlock: ")+
                theme.Good.Render("enabled"))
        }

        bal, err := lnd.GetBalance(m.cfg.Network)
        if err == nil && bal.TotalBalance != "" {
            lines = append(lines, "  "+theme.Label.Render("Balance: ")+
                theme.Value.Render(bal.TotalBalance+" sats"))
        }

        info, err := lnd.GetInfo(m.cfg.Network)
        if err == nil {
            if info.Channels > 0 {
                lines = append(lines, "  "+theme.Label.Render("Channels: ")+
                    theme.Value.Render(strings.TrimSpace(
                        strings.Replace(string(rune(info.Channels+'0')), "", "", 0))))
            }
            if info.Pubkey != "" {
                lines = append(lines, "")
                lines = append(lines, "  "+theme.Label.Render("Pubkey:"))
                lines = append(lines, "  "+theme.Mono.Render(info.Pubkey))
            }
        }
    } else {
        lines = append(lines, "  "+theme.Warning.Render("Wallet not created"))
    }

    content := strings.Join(lines, "\n")
    box := theme.Box.Width(bw).Padding(1, 2).Render(content)
    title := theme.Title.Width(bw).Align(lipgloss.Center).
        Render(" ⚡️ Lightning Details ")
    footer := theme.Footer.Render("  backspace back • q quit  ")
    full := lipgloss.JoinVertical(lipgloss.Center,
        "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center, full)
}