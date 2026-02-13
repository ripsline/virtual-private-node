package installer

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

var (
    tuiTitleStyle = lipgloss.NewStyle().Bold(true).
            Foreground(lipgloss.Color("0")).
            Background(lipgloss.Color("15")).Padding(0, 2)
    tuiSectionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
    tuiSelectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
    tuiUnselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
    tuiDimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
    tuiWarningStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
    tuiBoxStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
                BorderForeground(lipgloss.Color("245")).Padding(1, 2)
    tuiSummaryKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
                Width(16).Align(lipgloss.Right)
    tuiSummaryValStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
    tuiValueStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)

const tuiContentWidth = 76

type question struct {
    title   string
    options []option
}

type option struct {
    label, desc, value, warn string
}

func buildQuestions() []question {
    return []question{
        {title: "Network", options: []option{
            {label: "Mainnet", desc: "Real bitcoin — use with caution", value: "mainnet"},
            {label: "Testnet4", desc: "Test bitcoin — safe for experimenting", value: "testnet4"},
        }},
        {title: "Components", options: []option{
            {label: "Bitcoin Core only", desc: "Pruned node routed through Tor", value: "bitcoin"},
            {label: "Bitcoin Core + LND", desc: "Full Lightning node with Tor hidden services", value: "bitcoin+lnd"},
        }},
        {title: "Blockchain Storage (Pruned)", options: []option{
            {label: "10 GB", desc: "Minimum — works but tight", value: "10"},
            {label: "25 GB", desc: "Recommended", value: "25"},
            {label: "50 GB", desc: "More block history", value: "50",
                warn: "Make sure your VPS has at least 60 GB of disk space"},
        }},
    }
}

func p2pQuestion() question {
    return question{title: "LND P2P Mode", options: []option{
        {label: "Tor only", desc: "Maximum privacy", value: "tor"},
        {label: "Hybrid", desc: "Tor + clearnet — better routing", value: "hybrid"},
    }}
}

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
    cursors   []int
    answers   []string
    phase     tuiPhase
    version   string
    width, height int
}

type tuiResult struct {
    network, components, pruneSize, p2pMode string
}

func newTuiModel(version string) tuiModel {
    q := buildQuestions()
    return tuiModel{
        questions: q, cursors: make([]int, len(q)),
        answers: make([]string, len(q)), version: version,
    }
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            m.phase = phaseCancelled
            return m, tea.Quit
        case "up", "k":
            if m.phase == phaseQuestions && m.cursors[m.current] > 0 {
                m.cursors[m.current]--
            }
        case "down", "j":
            if m.phase == phaseQuestions {
                mx := len(m.questions[m.current].options) - 1
                if m.cursors[m.current] < mx {
                    m.cursors[m.current]++
                }
            }
        case "enter":
            return m.handleEnter()
        case "backspace":
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
    m.answers[m.current] = m.questions[m.current].options[m.cursors[m.current]].value
    if m.current == 1 {
        m = m.handleComponentChoice()
    }
    if m.current < len(m.questions)-1 {
        m.current++
    } else {
        m.phase = phaseSummary
    }
    return m, nil
}

func (m tuiModel) handleComponentChoice() tuiModel {
    hasP2P := false
    for _, q := range m.questions {
        if q.title == "LND P2P Mode" {
            hasP2P = true
            break
        }
    }
    if m.answers[1] == "bitcoin+lnd" && !hasP2P {
        p := p2pQuestion()
        nq := make([]question, 0, len(m.questions)+1)
        nq = append(nq, m.questions[:3]...)
        nq = append(nq, p)
        nq = append(nq, m.questions[3:]...)
        m.questions = nq
        nc := make([]int, len(m.questions))
        copy(nc, m.cursors)
        m.cursors = nc
        na := make([]string, len(m.questions))
        copy(na, m.answers)
        m.answers = na
    } else if m.answers[1] == "bitcoin" && hasP2P {
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
    if m.width == 0 {
        return "Loading..."
    }
    bw := minInt(m.width-4, tuiContentWidth)
    title := tuiTitleStyle.Width(bw).Align(lipgloss.Center).
        Render(fmt.Sprintf(" Virtual Private Node v%s ", m.version))
    var content string
    switch m.phase {
    case phaseQuestions:
        content = m.renderQuestion()
    case phaseSummary:
        content = m.renderSummary()
    }
    var footer string
    if m.phase == phaseQuestions {
        footer = tuiDimStyle.Render("  ↑↓ navigate • enter select • backspace back • q quit  ")
    } else {
        footer = tuiDimStyle.Render("  enter confirm • backspace edit • q cancel  ")
    }
    box := tuiBoxStyle.Width(bw).Render(content)
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}

func (m tuiModel) renderQuestion() string {
    var b strings.Builder
    b.WriteString(tuiDimStyle.Render(fmt.Sprintf("Question %d of %d",
        m.current+1, len(m.questions))) + "\n\n")
    b.WriteString(tuiSectionStyle.Render(m.questions[m.current].title) + "\n\n")
    for i, opt := range m.questions[m.current].options {
        cur, sty := "  ", tuiUnselectedStyle
        if i == m.cursors[m.current] {
            cur, sty = "▸ ", tuiSelectedStyle
        }
        b.WriteString(sty.Render(cur+opt.label) + tuiDimStyle.Render(" — "+opt.desc) + "\n")
        if i == m.cursors[m.current] && opt.warn != "" {
            b.WriteString("  " + tuiWarningStyle.Render("WARNING: "+opt.warn) + "\n")
        }
    }
    if m.current > 0 {
        b.WriteString("\n" + tuiDimStyle.Render("─────────────────────────────") + "\n")
        for i := 0; i < m.current; i++ {
            b.WriteString(tuiDimStyle.Render(m.questions[i].title+": ") +
                tuiValueStyle.Render(m.answers[i]) + "\n")
        }
    }
    return b.String()
}

func (m tuiModel) renderSummary() string {
    var b strings.Builder
    b.WriteString(tuiSectionStyle.Render("Installation Summary") + "\n\n")
    r := m.getResult()
    rows := []struct{ k, v string }{
        {"Network", r.network}, {"Components", r.components},
        {"Prune", r.pruneSize + " GB"},
    }
    if r.components == "bitcoin+lnd" {
        mode := "Tor only"
        if r.p2pMode == "hybrid" {
            mode = "Hybrid (Tor + clearnet)"
        }
        rows = append(rows, struct{ k, v string }{"P2P Mode", mode})
    }
    var c strings.Builder
    for _, row := range rows {
        c.WriteString(tuiSummaryKeyStyle.Render(row.k+":") +
            tuiSummaryValStyle.Render(" "+row.v) + "\n")
    }
    b.WriteString(tuiBoxStyle.Render(c.String()) + "\n\n")
    b.WriteString(tuiSelectedStyle.Render("Press Enter to install"))
    return b.String()
}

func (m tuiModel) getResult() tuiResult {
    r := tuiResult{network: "testnet4", components: "bitcoin+lnd",
        pruneSize: "25", p2pMode: "tor"}
    for i, q := range m.questions {
        if i >= len(m.answers) || m.answers[i] == "" {
            continue
        }
        switch q.title {
        case "Network":
            r.network = m.answers[i]
        case "Components":
            r.components = m.answers[i]
        case "Blockchain Storage (Pruned)":
            r.pruneSize = m.answers[i]
        case "LND P2P Mode":
            r.p2pMode = m.answers[i]
        }
    }
    return r
}

func RunTUI(version string) (*installConfig, error) {
    m := newTuiModel(version)
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
        network: NetworkConfigFromName(r.network), components: r.components,
        p2pMode: r.p2pMode, sshPort: 22,
    }
    fmt.Sscanf(r.pruneSize, "%d", &cfg.pruneSize)
    if cfg.p2pMode == "hybrid" {
        cfg.publicIPv4 = detectPublicIP()
        if cfg.publicIPv4 == "" {
            cfg.p2pMode = "tor"
        }
    }
    return cfg, nil
}