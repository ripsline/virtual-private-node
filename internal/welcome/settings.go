package welcome

import (
    "fmt"
    "strconv"

    "github.com/charmbracelet/lipgloss"

    "github.com/ripsline/virtual-private-node/internal/installer"
    "github.com/ripsline/virtual-private-node/internal/theme"
)

var pruneSizes = []int{25, 50, 75, 100}

func (m Model) viewSettings(bw int) string {
    halfW := (bw - 4) / 2
    cardH := theme.BoxHeight / 2

    pruneCard := m.settingsPruneCard(halfW, cardH)
    updateCard := m.settingsUpdateCard(halfW, cardH)

    return lipgloss.JoinHorizontal(lipgloss.Top, pruneCard, "  ", updateCard)
}

func (m Model) settingsPruneCard(w, h int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("Blockchain Storage"))
    lines = append(lines, "")
    lines = append(lines, theme.Label.Render("Current: ")+
        theme.Value.Render(fmt.Sprintf("%d GB", m.cfg.PruneSize)))
    lines = append(lines, "")

    if m.settingsFocus == 0 {
        for i, size := range pruneSizes {
            prefix := "  "
            style := theme.Value
            if i == m.settingsCursor {
                prefix = "▸ "
                style = theme.Action
            }
            label := fmt.Sprintf("%d GB", size)
            if size == 25 {
                label += " — Minimum"
            }
            if size == m.cfg.PruneSize {
                label += " (current)"
            }
            lines = append(lines, style.Render(prefix+label))
        }

        lines = append(lines, "")

        if m.settingsCustom {
            lines = append(lines, theme.Action.Render("▸ Custom: "+m.settingsInput+" GB"))
        } else {
            prefix := "  "
            style := theme.Value
            if m.settingsCursor == len(pruneSizes) {
                prefix = "▸ "
                style = theme.Action
            }
            lines = append(lines, style.Render(prefix+"Custom..."))
        }
    } else {
        for _, size := range pruneSizes {
            label := fmt.Sprintf("  %d GB", size)
            if size == m.cfg.PruneSize {
                label += " (current)"
            }
            lines = append(lines, theme.Dim.Render(label))
        }
    }

    if m.settingsConfirm != "" {
        lines = append(lines, "")
        lines = append(lines, theme.Warning.Render(
            fmt.Sprintf("Change to %s? [y/n]", m.settingsConfirm)))
    }

    border := theme.NormalBorder
    if m.settingsFocus == 0 {
        border = theme.SelectedBorder
    }
    return border.Width(w).Padding(1, 2).Render(padLines(lines, h))
}

func (m Model) settingsUpdateCard(w, h int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("Update"))
    lines = append(lines, "")
    lines = append(lines, theme.Label.Render("Current: ")+
        theme.Value.Render("v"+installer.GetVersion()))
    lines = append(lines, "")

    if m.latestVersion == "" {
        lines = append(lines, theme.Dim.Render("Checking for updates..."))
    } else if m.latestVersion == installer.GetVersion() {
        lines = append(lines, theme.GreenDot.Render("●")+" "+
            theme.Good.Render("Up to date"))
    } else {
        lines = append(lines, theme.Label.Render("Latest:  ")+
            theme.Action.Render("v"+m.latestVersion))
        lines = append(lines, "")
        if m.updateConfirm {
            lines = append(lines, theme.Warning.Render(
                "Update to v"+m.latestVersion+"? [y/n]"))
        } else if m.settingsFocus == 1 {
            lines = append(lines, theme.Action.Render("Select to update ▸"))
        }
    }

    border := theme.NormalBorder
    if m.settingsFocus == 1 {
        border = theme.SelectedBorder
    }
    return border.Width(w).Padding(1, 2).Render(padLines(lines, h))
}

func handleSettingsKey(m Model, key string) Model {
    // Handle custom input mode
    if m.settingsCustom {
        switch key {
        case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
            if len(m.settingsInput) < 4 {
                m.settingsInput += key
            }
        case "backspace":
            if len(m.settingsInput) > 0 {
                m.settingsInput = m.settingsInput[:len(m.settingsInput)-1]
            } else {
                m.settingsCustom = false
            }
        case "enter":
            val, err := strconv.Atoi(m.settingsInput)
            if err == nil && val >= 25 {
                m.settingsConfirm = fmt.Sprintf("%d GB", val)
                m.settingsPendingSize = val
            }
            m.settingsCustom = false
            m.settingsInput = ""
        }
        return m
    }

    // Handle confirmations
    if m.settingsConfirm != "" {
        switch key {
        case "y":
            size := m.settingsPendingSize
            m.settingsConfirm = ""
            installer.RunPruneSizeChange(m.cfg, size)
        default:
            m.settingsConfirm = ""
            m.settingsPendingSize = 0
        }
        return m
    }

    if m.updateConfirm {
        switch key {
        case "y":
            m.updateConfirm = false
            m.shellAction = svSelfUpdate
        default:
            m.updateConfirm = false
        }
        return m
    }

    // Navigation
    switch key {
    case "left", "h":
        m.settingsFocus = 0
    case "right", "l":
        m.settingsFocus = 1
    case "up", "k":
        if m.settingsFocus == 0 && m.settingsCursor > 0 {
            m.settingsCursor--
        }
    case "down", "j":
        if m.settingsFocus == 0 && m.settingsCursor < len(pruneSizes) {
            m.settingsCursor++
        }
    case "enter":
        if m.settingsFocus == 0 {
            // Prune card
            if m.settingsCursor == len(pruneSizes) {
                m.settingsCustom = true
                m.settingsInput = ""
            } else {
                size := pruneSizes[m.settingsCursor]
                if size != m.cfg.PruneSize {
                    m.settingsConfirm = fmt.Sprintf("%d GB", size)
                    m.settingsPendingSize = size
                }
            }
        } else {
            // Update card
            if m.latestVersion != "" && m.latestVersion != installer.GetVersion() {
                m.updateConfirm = true
            }
        }
    }
    return m
}