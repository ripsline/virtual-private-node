// Package installer — tui.go
//
// Interactive TUI for gathering installation configuration.
// Uses bubbletea for terminal UI with arrow key navigation
// and lipgloss for styling. Matches the welcome TUI's
// black/white brand aesthetic.
package installer

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// ── Styles (black and white brand, matching welcome TUI) ─

var (
    // Title bar — black text on white background
    tuiTitleStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    // Section headers
    tuiSectionStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    // Selected option — white bold
    tuiSelectedStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("15")).
                Bold(true)

    // Unselected option — light gray
    tuiUnselectedStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("250"))

    // De-emphasized text
    tuiDimStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    // Warning text for options (e.g. "needs larger SSD")
    tuiWarningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Italic(true)

    // Summary box border
    tuiBoxStyle = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245")).
            Padding(1, 2)

    // Summary key (left side, right-aligned)
    tuiSummaryKeyStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("245")).
                Width(16).
                Align(lipgloss.Right)

    // Summary value (right side, bold white)
    tuiSummaryValStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("15")).
                Bold(true)
)

// ── Questions ────────────────────────────────────────────
//
// Each question has a title and a list of options. Options
// have a label (shown to user), description, value (stored
// in config), and optional warning text.

type question struct {
    title   string
    options []option
}

type option struct {
    label string
    desc  string
    value string
    warn  string // shown when this option is highlighted
}

// buildQuestions returns the base set of questions shown to every user.
func buildQuestions() []question {
    return []question{
        {
            title: "Network",
            options: []option{
                {
                    label: "Mainnet",
                    desc:  "Real bitcoin — use with caution",
                    value: "mainnet",
                },
                {
                    label: "Testnet4",
                    desc:  "Test bitcoin — safe for experimenting",
                    value: "testnet4",
                },
            },
        },
        {
            title: "Components",
            options: []option{
                {
                    label: "Bitcoin Core only",
                    desc:  "Pruned node routed through Tor",
                    value: "bitcoin",
                },
                {
                    label: "Bitcoin Core + LND",
                    desc:  "Full Lightning node with Tor hidden services",
                    value: "bitcoin+lnd",
                },
            },
        },
        {
            title: "Blockchain Storage (Pruned)",
            options: []option{
                {
                    label: "10 GB",
                    desc:  "Minimum — works but tight",
                    value: "10",
                },
                {
                    label: "25 GB",
                    desc:  "Recommended",
                    value: "25",
                },
                {
                    label: "50 GB",
                    desc:  "More block history",
                    value: "50",
                    warn:  "Make sure your VPS has at least 60 GB of disk space",
                },
            },
        },
    }
}

// p2pQuestion is only shown when LND is selected as a component.
func p2pQuestion() question {
    return question{
        title: "LND P2P Mode",
        options: []option{
            {
                label: "Tor only",
                desc:  "Maximum privacy — all connections through Tor",
                value: "tor",
            },
            {
                label: "Hybrid",
                desc:  "Tor + clearnet — better routing performance",
                value: "hybrid",
            },
        },
    }
}

// sshQuestion asks which SSH port to use.
func sshQuestion() question {
    return question{
        title: "SSH Port",
        options: []option{
            {
                label: "22",
                desc:  "Default SSH port",
                value: "22",
            },
            {
                label: "Custom",
                desc:  "Enter a custom port after selection",
                value: "custom",
            },
        },
    }
}

// ── TUI Model ────────────────────────────────────────────

type tuiPhase int

const (
    phaseQuestions tuiPhase = iota
    phaseSummary
    phaseConfirmed
    phaseCancelled
)

type tuiModel struct {
    questions []question
    current   int
    cursors   []int    // selected option index per question
    answers   []string // stored value per question
    phase     tuiPhase
    width     int
    height    int
}

// tuiResult holds the parsed answers from the TUI.
type tuiResult struct {
    network    string
    components string
    pruneSize  string
    p2pMode    string
    sshPort    string
}

func newTuiModel() tuiModel {
    questions := buildQuestions()
    questions = append(questions, sshQuestion())

    return tuiModel{
        questions: questions,
        cursors:   make([]int, len(questions)),
        answers:   make([]string, len(questions)),
    }
}

func (m tuiModel) Init() tea.Cmd {
    return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c":
            m.phase = phaseCancelled
            return m, tea.Quit

        case "q":
            if m.phase == phaseSummary {
                m.phase = phaseCancelled
                return m, tea.Quit
            }

        // Navigate options up
        case "up", "k":
            if m.phase == phaseQuestions {
                if m.cursors[m.current] > 0 {
                    m.cursors[m.current]--
                }
            }

        // Navigate options down
        case "down", "j":
            if m.phase == phaseQuestions {
                maxIdx := len(m.questions[m.current].options) - 1
                if m.cursors[m.current] < maxIdx {
                    m.cursors[m.current]++
                }
            }

        // Select option or confirm summary
        case "enter":
            return m.handleEnter()

        // Go back to previous question
        case "backspace", "left", "h":
            if m.phase == phaseQuestions && m.current > 0 {
                m.current--
            } else if m.phase == phaseSummary {
                m.phase = phaseQuestions
                m.current = len(m.questions) - 1
            }
        }
    }

    return m, nil
}

// handleEnter processes the Enter key — saves the current answer
// and advances to the next question or summary.
func (m tuiModel) handleEnter() (tea.Model, tea.Cmd) {
    if m.phase == phaseSummary {
        m.phase = phaseConfirmed
        return m, tea.Quit
    }

    if m.phase != phaseQuestions {
        return m, nil
    }

    // Save the selected answer
    q := m.questions[m.current]
    selected := q.options[m.cursors[m.current]]
    m.answers[m.current] = selected.value

    // After components question, dynamically add/remove P2P question
    if m.current == 1 {
        m = m.handleComponentChoice()
    }

    // Advance to next question or show summary
    if m.current < len(m.questions)-1 {
        m.current++
    } else {
        m.phase = phaseSummary
    }

    return m, nil
}

// handleComponentChoice adds the P2P question when LND is selected
// and removes it when Bitcoin-only is selected.
func (m tuiModel) handleComponentChoice() tuiModel {
    hasP2P := false
    for _, q := range m.questions {
        if q.title == "LND P2P Mode" {
            hasP2P = true
            break
        }
    }

    if m.answers[1] == "bitcoin+lnd" && !hasP2P {
        // Insert P2P question after prune size (index 3)
        p2p := p2pQuestion()
        newQuestions := make([]question, 0, len(m.questions)+1)
        newQuestions = append(newQuestions, m.questions[:3]...)
        newQuestions = append(newQuestions, p2p)
        newQuestions = append(newQuestions, m.questions[3:]...)
        m.questions = newQuestions

        newCursors := make([]int, len(m.questions))
        copy(newCursors, m.cursors)
        m.cursors = newCursors

        newAnswers := make([]string, len(m.questions))
        copy(newAnswers, m.answers)
        m.answers = newAnswers
    } else if m.answers[1] == "bitcoin" && hasP2P {
        // Remove P2P question
        for i, q := range m.questions {
            if q.title == "LND P2P Mode" {
                m.questions = append(m.questions[:i], m.questions[i+1:]...)
                m.cursors = append(m.cursors[:i], m.cursors[i+1:]...)
                m.answers = append(m.answers[:i], m.answers[i+1:]...)
                break
            }
        }
    }

    return m
}

// View renders the install TUI. Returns a loading screen if
// the terminal size hasn't been reported yet.
func (m tuiModel) View() string {
    // Wait for terminal size before rendering
    if m.width == 0 || m.height == 0 {
        return "Loading..."
    }

    var b strings.Builder

    // Title
    b.WriteString(tuiTitleStyle.Render(" Virtual Private Node "))
    b.WriteString("\n\n")

    switch m.phase {
    case phaseQuestions:
        b.WriteString(m.renderQuestion())
    case phaseSummary:
        b.WriteString(m.renderSummary())
    }

    // Footer with keyboard hints
    b.WriteString("\n")
    if m.phase == phaseQuestions {
        b.WriteString(tuiDimStyle.Render("↑↓ navigate • enter select • backspace back • ctrl+c quit"))
    } else if m.phase == phaseSummary {
        b.WriteString(tuiDimStyle.Render("enter confirm • backspace edit • q cancel"))
    }

    content := b.String()

    // Center everything in the terminal
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        content,
    )
}

// renderQuestion shows the current question with selectable options.
func (m tuiModel) renderQuestion() string {
    var b strings.Builder

    // Progress indicator
    progress := tuiDimStyle.Render(fmt.Sprintf(
        "Question %d of %d", m.current+1, len(m.questions),
    ))
    b.WriteString(progress)
    b.WriteString("\n\n")

    q := m.questions[m.current]

    // Question title
    b.WriteString(tuiSectionStyle.Render(q.title))
    b.WriteString("\n\n")

    // Options with cursor indicator
    for i, opt := range q.options {
        cursor := "  "
        style := tuiUnselectedStyle
        if i == m.cursors[m.current] {
            cursor = "▸ "
            style = tuiSelectedStyle
        }

        label := style.Render(cursor + opt.label)
        desc := tuiDimStyle.Render(" — " + opt.desc)
        b.WriteString(label + desc)
        b.WriteString("\n")

        // Show warning if this option is currently highlighted
        if i == m.cursors[m.current] && opt.warn != "" {
            b.WriteString("  " + tuiWarningStyle.Render("⚠️  "+opt.warn))
            b.WriteString("\n")
        }
    }

    // Show previously answered questions below the current one
    if m.current > 0 {
        b.WriteString("\n")
        b.WriteString(tuiDimStyle.Render("─────────────────────────────"))
        b.WriteString("\n")
        for i := 0; i < m.current; i++ {
            key := tuiDimStyle.Render(fmt.Sprintf("%s:", m.questions[i].title))
            val := m.answers[i]
            b.WriteString(fmt.Sprintf("%s %s\n", key, val))
        }
    }

    return b.String()
}

// renderSummary shows all selected options in a bordered box.
func (m tuiModel) renderSummary() string {
    var b strings.Builder

    b.WriteString(tuiSectionStyle.Render("Installation Summary"))
    b.WriteString("\n\n")

    result := m.getResult()

    rows := []struct {
        key string
        val string
    }{
        {"Network", result.network},
        {"Components", result.components},
        {"Prune", result.pruneSize + " GB"},
        {"SSH Port", result.sshPort},
    }

    // Insert P2P mode row if LND is selected
    if result.components == "bitcoin+lnd" {
        mode := "Tor only"
        if result.p2pMode == "hybrid" {
            mode = "Hybrid (Tor + clearnet)"
        }
        rows = append(rows[:3],
            append([]struct {
                key string
                val string
            }{{"P2P Mode", mode}}, rows[3:]...)...,
        )
    }

    // Build the summary content inside a bordered box
    var content strings.Builder
    for _, row := range rows {
        key := tuiSummaryKeyStyle.Render(row.key + ":")
        val := tuiSummaryValStyle.Render(" " + row.val)
        content.WriteString(key + val + "\n")
    }

    b.WriteString(tuiBoxStyle.Render(content.String()))
    b.WriteString("\n\n")

    b.WriteString(tuiSelectedStyle.Render("Press Enter to install"))
    b.WriteString("\n")

    return b.String()
}

// getResult extracts the final configuration from answered questions.
func (m tuiModel) getResult() tuiResult {
    result := tuiResult{
        network:    "testnet4",
        components: "bitcoin+lnd",
        pruneSize:  "25",
        p2pMode:    "tor",
        sshPort:    "22",
    }

    for i, q := range m.questions {
        if i >= len(m.answers) || m.answers[i] == "" {
            continue
        }
        switch q.title {
        case "Network":
            result.network = m.answers[i]
        case "Components":
            result.components = m.answers[i]
        case "Blockchain Storage (Pruned)":
            result.pruneSize = m.answers[i]
        case "LND P2P Mode":
            result.p2pMode = m.answers[i]
        case "SSH Port":
            result.sshPort = m.answers[i]
        }
    }

    return result
}

// RunTUI launches the interactive setup TUI and returns the
// user's choices as an installConfig. Returns nil if cancelled.
func RunTUI() (*installConfig, error) {
    m := newTuiModel()

    p := tea.NewProgram(m, tea.WithAltScreen())
    result, err := p.Run()
    if err != nil {
        return nil, fmt.Errorf("TUI error: %w", err)
    }

    final := result.(tuiModel)
    if final.phase == phaseCancelled {
        return nil, nil
    }

    r := final.getResult()

    cfg := &installConfig{
        network:    NetworkConfigFromName(r.network),
        components: r.components,
        p2pMode:    r.p2pMode,
        sshPort:    22,
    }

    fmt.Sscanf(r.pruneSize, "%d", &cfg.pruneSize)

    if r.sshPort != "custom" {
        fmt.Sscanf(r.sshPort, "%d", &cfg.sshPort)
    }

    // If hybrid mode, auto-detect public IP silently
    if cfg.p2pMode == "hybrid" {
        cfg.publicIPv4 = detectPublicIP()
        if cfg.publicIPv4 == "" {
            // Can't detect IP — fall back to Tor only
            cfg.p2pMode = "tor"
        }
    }

    return cfg, nil
}