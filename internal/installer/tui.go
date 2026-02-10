package installer

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// ── Styles ───────────────────────────────────────────────

var (
    titleStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).
            Padding(0, 2)

    sectionStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    selectedStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)

    unselectedStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("250"))

    dimStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243"))

    warningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Italic(true)

    boxStyle = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("245")).
            Padding(1, 2)

    summaryKeyStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Width(16).
            Align(lipgloss.Right)

    summaryValStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")).
            Bold(true)
)

// ── Questions ────────────────────────────────────────────

type question struct {
    title   string
    options []option
}

type option struct {
    label string
    desc  string
    value string
    warn  string // optional warning shown when selected
}

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
                    desc:  "Minimum",
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
                    warn:  "Make sure your SSD is at least 100 GB",
                },
            },
        },
    }
}

// P2P question is only shown if LND is selected.
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

// SSH question
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
    cursors   []int // selected option index per question
    answers   []string
    phase     tuiPhase
    width     int
    height    int
}

type tuiResult struct {
    network    string
    components string
    pruneSize  string
    p2pMode    string
    sshPort    string
}

func newTuiModel() tuiModel {
    questions := buildQuestions()
    // Add SSH question
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

        case "up", "k":
            if m.phase == phaseQuestions {
                if m.cursors[m.current] > 0 {
                    m.cursors[m.current]--
                }
            }

        case "down", "j":
            if m.phase == phaseQuestions {
                max := len(m.questions[m.current].options) - 1
                if m.cursors[m.current] < max {
                    m.cursors[m.current]++
                }
            }

        case "enter":
            return m.handleEnter()

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

func (m tuiModel) handleEnter() (tea.Model, tea.Cmd) {
    if m.phase == phaseSummary {
        m.phase = phaseConfirmed
        return m, tea.Quit
    }

    if m.phase != phaseQuestions {
        return m, nil
    }

    // Save the answer
    q := m.questions[m.current]
    selected := q.options[m.cursors[m.current]]
    m.answers[m.current] = selected.value

    // After components question, inject or remove P2P question
    if m.current == 1 {
        m = m.handleComponentChoice()
    }

    // Move to next question or summary
    if m.current < len(m.questions)-1 {
        m.current++
    } else {
        m.phase = phaseSummary
    }

    return m, nil
}

// handleComponentChoice adds or removes the P2P question based
// on whether LND was selected.
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

func (m tuiModel) View() string {
    var b strings.Builder

    // Title
    b.WriteString("\n")
    b.WriteString("  " + titleStyle.Render(" Virtual Private Node "))
    b.WriteString("\n\n")

    switch m.phase {
    case phaseQuestions:
        b.WriteString(m.renderQuestion())
    case phaseSummary:
        b.WriteString(m.renderSummary())
    }

    // Footer
    b.WriteString("\n")
    if m.phase == phaseQuestions {
        footer := dimStyle.Render("  ↑↓ navigate • enter select • backspace back • ctrl+c quit")
        b.WriteString(footer)
    } else if m.phase == phaseSummary {
        footer := dimStyle.Render("  enter confirm • backspace edit • q cancel")
        b.WriteString(footer)
    }
    b.WriteString("\n")

    return b.String()
}

func (m tuiModel) renderQuestion() string {
    var b strings.Builder

    // Progress indicator
    progress := dimStyle.Render(fmt.Sprintf(
        "  Question %d of %d", m.current+1, len(m.questions),
    ))
    b.WriteString(progress)
    b.WriteString("\n\n")

    q := m.questions[m.current]

    // Question title
    b.WriteString("  " + sectionStyle.Render(q.title))
    b.WriteString("\n\n")

    // Options
    for i, opt := range q.options {
        cursor := "  "
        style := unselectedStyle
        if i == m.cursors[m.current] {
            cursor = "▸ "
            style = selectedStyle
        }

        label := style.Render(cursor + opt.label)
        desc := dimStyle.Render(" — " + opt.desc)
        b.WriteString("  " + label + desc)
        b.WriteString("\n")

        // Show warning if this option is selected
        if i == m.cursors[m.current] && opt.warn != "" {
            b.WriteString("    " + warningStyle.Render("⚠️  "+opt.warn))
            b.WriteString("\n")
        }
    }

    // Show previous answers
    if m.current > 0 {
        b.WriteString("\n")
        b.WriteString("  " + dimStyle.Render("─────────────────────────────"))
        b.WriteString("\n")
        for i := 0; i < m.current; i++ {
            key := dimStyle.Render(fmt.Sprintf("  %s:", m.questions[i].title))
            val := m.answers[i]
            b.WriteString(fmt.Sprintf("  %s %s\n", key, val))
        }
    }

    return b.String()
}

func (m tuiModel) renderSummary() string {
    var b strings.Builder

    b.WriteString("  " + sectionStyle.Render("Installation Summary"))
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

    // Build the summary box content
    var content strings.Builder
    for _, row := range rows {
        key := summaryKeyStyle.Render(row.key + ":")
        val := summaryValStyle.Render(" " + row.val)
        content.WriteString(key + val + "\n")
    }

    b.WriteString(boxStyle.Render(content.String()))
    b.WriteString("\n\n")

    b.WriteString("  " + selectedStyle.Render("Press Enter to install"))
    b.WriteString("\n")

    return b.String()
}

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
// user's choices. Returns nil if the user cancelled.
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

    // Parse prune size
    fmt.Sscanf(r.pruneSize, "%d", &cfg.pruneSize)

    // Parse SSH port
    if r.sshPort != "custom" {
        fmt.Sscanf(r.sshPort, "%d", &cfg.sshPort)
    }

    return cfg, nil
}