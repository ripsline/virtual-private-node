package welcome

import (
    "time"

    tea "github.com/charmbracelet/bubbletea"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/installer"
)

// ── Enums ────────────────────────────────────────────────

type wTab int

const (
    tabDashboard wTab = iota
    tabPairing
    tabLogs
    tabAddons
    tabSettings
)

type wSubview int

const (
    svNone wSubview = iota
    svLightning
    svZeus
    svSparrow
    svQR
    svFullURL
    svWalletCreate
    svLNDInstall
    svLITInstall
    svSyncthingInstall
    svSystemUpdate
    svLogView
    svMacaroonShell
    svPruneChange
)

type cardPos int

const (
    cardServices cardPos = iota
    cardSystem
    cardBitcoin
    cardLightning
    cardSyncthing
    cardLIT
)

type logSelection int

const (
    logSelTor logSelection = iota
    logSelBitcoin
    logSelLND
    logSelLIT
    logSelSyncthing
)

// ── Messages ─────────────────────────────────────────────

type svcActionDoneMsg struct{}
type tickMsg time.Time

type statusMsg struct {
    services                            map[string]bool
    diskTotal, diskUsed, diskPct        string
    ramTotal, ramUsed, ramPct           string
    btcSize, lndSize                    string
    btcBlocks, btcHeaders               int
    btcProgress                         float64
    btcSynced, btcResponding            bool
    rebootRequired                      bool
}

// ── Model ────────────────────────────────────────────────

type Model struct {
    cfg          *config.AppConfig
    version      string
    activeTab    wTab
    subview      wSubview
    dashCard     cardPos
    cardActive   bool
    svcCursor    int
    svcConfirm   string
    sysConfirm   string
    logSel       logSelection
    addonFocus   int
    pairingFocus int
    urlTarget    string
    width        int
    height       int
    shellAction  wSubview
    status       *statusMsg
}

func NewModel(cfg *config.AppConfig, version string) Model {
    return Model{
        cfg: cfg, version: version,
        activeTab: tabDashboard, subview: svNone,
        dashCard: cardServices,
    }
}

// Show launches the welcome TUI. Re-launches after shell actions.
func Show(cfg *config.AppConfig, version string) {
    for {
        m := NewModel(cfg, version)
        p := tea.NewProgram(m, tea.WithAltScreen())
        result, _ := p.Run()
        final := result.(Model)

        switch final.shellAction {
        case svMacaroonShell:
            printMacaroon(cfg)
            continue
        case svWalletCreate:
            installer.RunWalletCreation(cfg)
            if u, e := config.Load(); e == nil {
                cfg = u
            }
            continue
        case svLNDInstall:
            installer.RunLNDInstall(cfg)
            if u, e := config.Load(); e == nil {
                cfg = u
            }
            installer.AppendLNCLIToShell(cfg)
            continue
        case svLITInstall:
            installer.RunLITInstall(cfg)
            if u, e := config.Load(); e == nil {
                cfg = u
            }
            continue
        case svSyncthingInstall:
            installer.RunSyncthingInstall(cfg)
            if u, e := config.Load(); e == nil {
                cfg = u
            }
            continue
        case svSystemUpdate:
            runSystemUpdate()
            continue
        case svLogView:
            runLogViewer(final.logSel, cfg)
            continue
        default:
            return
        }
    }
}

func (m Model) Init() tea.Cmd {
    return tea.Batch(
        fetchStatus(m.cfg),
        tickEvery(5*time.Second),
    )
}

func tickEvery(d time.Duration) tea.Cmd {
    return tea.Tick(d, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}