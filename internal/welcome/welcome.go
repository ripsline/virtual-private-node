// internal/welcome/welcome.go

package welcome

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ripsline/virtual-private-node/internal/config"
	"github.com/ripsline/virtual-private-node/internal/installer"
	"github.com/ripsline/virtual-private-node/internal/logger"
)

// ── Enums ────────────────────────────────────────────────

type wTab int

const (
	tabDashboard wTab = iota
	tabPairing
	tabAddons
	tabSettings
)

type wSubview int

const (
	svNone wSubview = iota
	svLightning
	svZeus
	svSyncthingDetail
	svLITDetail
	svLndHubDetail
	svLndHubCreateAccount
	svQR
	svFullURL
	svWalletCreate
	svLNDInstall
	svLITInstall
	svSyncthingInstall
	svLndHubInstall
	svSystemUpdate
	svLogView
	svMacaroonShell
	svSelfUpdate
	svP2PUpgrade
)

type cardPos int

const (
	cardServices cardPos = iota
	cardSystem
	cardBitcoin
	cardLightning
)

// ── Messages ─────────────────────────────────────────────

type svcActionDoneMsg struct{}
type tickMsg time.Time
type latestVersionMsg string

type statusMsg struct {
	services                     map[string]bool
	diskTotal, diskUsed, diskPct string
	ramTotal, ramUsed, ramPct    string
	btcSize, lndSize             string
	btcBlocks, btcHeaders        int
	btcProgress                  float64
	btcSynced, btcResponding     bool
	rebootRequired               bool
	lndPubkey                    string
	lndChannels                  int
	lndBalance                   string
	lndSyncedChain               bool
	lndSyncedGraph               bool
	lndResponding                bool
	publicIP                     string
}

// ── Model ────────────────────────────────────────────────

type Model struct {
	cfg           *config.AppConfig
	version       string
	activeTab     wTab
	subview       wSubview
	dashCard      cardPos
	cardActive    bool
	svcCursor     int
	svcConfirm    string
	sysConfirm    string
	logSvcName    string
	addonFocus    int
	urlTarget     string
	qrMode        string
	width         int
	height        int
	shellAction   wSubview
	status        *statusMsg
	settingsFocus int
	latestVersion string
	updateConfirm bool
	fetchInFlight bool
	lastAccount   *installer.LndHubAccount
}

func NewModel(cfg *config.AppConfig, version string) Model {
	return Model{
		cfg: cfg, version: version,
		activeTab: tabDashboard, subview: svNone,
		dashCard:      cardServices,
		fetchInFlight: true,
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
		case svLndHubInstall:
			installer.RunLndHubInstall(cfg)
			if u, e := config.Load(); e == nil {
				cfg = u
			}
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
			if err := installer.AppendLNCLIToShell(cfg); err != nil {
				logger.TUI("Warning: failed to add lncli wrapper: %v", err)
			}
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
			runLogViewer(final.logSvcName, cfg)
			continue
		case svSelfUpdate:
			installer.RunSelfUpdate(cfg, final.latestVersion)
			continue
		case svP2PUpgrade:
			installer.RunP2PModeUpgrade(cfg)
			if u, e := config.Load(); e == nil {
				cfg = u
			}
			continue
		case svLndHubCreateAccount:
			account, err := installer.CreateLndHubAccount(cfg.LndHubAdminToken)
			if err != nil {
				logger.TUI("Warning: failed to create LndHub account: %v", err)
			}
			// Store account to display on next TUI launch
			if account != nil {
				m := NewModel(cfg, version)
				m.lastAccount = account
				m.subview = svLndHubDetail
				p := tea.NewProgram(m, tea.WithAltScreen())
				result, _ := p.Run()
				final = result.(Model)
				// If they quit from detail, fall through to check shellAction again
				if final.shellAction != svNone {
					continue
				}
			}
			continue
		default:
			return
		}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(m.cfg),
		fetchLatestVersion(),
		tickEvery(5*time.Second),
	)
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchLatestVersion() tea.Cmd {
	return func() tea.Msg {
		v := installer.CheckLatestVersion()
		return latestVersionMsg(v)
	}
}
