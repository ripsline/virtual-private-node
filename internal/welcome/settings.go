package welcome

import (
    "fmt"

    "github.com/ripsline/virtual-private-node/internal/theme"
)

func (m Model) viewSettings(bw int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("Settings"))
    lines = append(lines, "")
    lines = append(lines, theme.Label.Render("Network: ")+
        theme.Value.Render(m.cfg.Network))
    lines = append(lines, "")
    lines = append(lines, theme.Header.Render("Blockchain Storage (Pruned)"))
    lines = append(lines, "")
    lines = append(lines, theme.Label.Render("Current: ")+
        theme.Value.Render(fmt.Sprintf("%d GB", m.cfg.PruneSize)))
    lines = append(lines, "")

    sizes := []int{25, 50, 75, 100}
    for _, size := range sizes {
        prefix := "  "
        style := theme.Value
        if size == m.cfg.PruneSize {
            prefix = "▸ "
            style = theme.Action
        }
        label := fmt.Sprintf("%d GB", size)
        if size == 25 {
            label += " — Minimum"
        }
        lines = append(lines, style.Render(prefix+label))
    }
    lines = append(lines, "")
    lines = append(lines, theme.Dim.Render("Select and press Enter to change."))
    lines = append(lines, theme.Dim.Render("No resync required."))

    return theme.Box.Width(bw).Padding(1, 2).Render(padLines(lines, theme.BoxHeight))
}