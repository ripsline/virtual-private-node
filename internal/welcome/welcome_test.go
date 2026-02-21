package welcome

import (
    "testing"

    tea "github.com/charmbracelet/bubbletea"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/installer"
)

func testModel() Model {
    cfg := config.Default()
    return NewModel(cfg, "0.0.0-test")
}

func testModelWithLND() Model {
    cfg := config.Default()
    cfg.LNDInstalled = true
    return NewModel(cfg, "0.0.0-test")
}

func testModelFullStack() Model {
    cfg := config.Default()
    cfg.LNDInstalled = true
    cfg.LITInstalled = true
    cfg.SyncthingInstalled = true
    return NewModel(cfg, "0.0.0-test")
}

// ── Tab Navigation ───────────────────────────────────────

func TestInitialState(t *testing.T) {
    m := testModel()

    if m.activeTab != tabDashboard {
        t.Errorf("initial tab: got %d, want %d (dashboard)", m.activeTab, tabDashboard)
    }
    if m.subview != svNone {
        t.Errorf("initial subview: got %d, want %d (none)", m.subview, svNone)
    }
    if m.dashCard != cardServices {
        t.Errorf("initial card: got %d, want %d (services)", m.dashCard, cardServices)
    }
    if m.cardActive {
        t.Error("card should not be active initially")
    }
}

func TestTabForward(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24

    // Tab through all 4 tabs
    expected := []wTab{tabPairing, tabAddons, tabSettings, tabDashboard}
    for _, want := range expected {
        newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
        m = newM.(Model)
        if m.activeTab != want {
            t.Errorf("after tab: got %d, want %d", m.activeTab, want)
        }
    }
}

func TestTabBackward(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
    m = newM.(Model)
    if m.activeTab != tabSettings {
        t.Errorf("after shift+tab: got %d, want %d (settings)", m.activeTab, tabSettings)
    }
}

func TestNumberKeySwitchesTab(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24

    tests := []struct {
        key  string
        want wTab
    }{
        {"1", tabDashboard},
        {"2", tabPairing},
        {"3", tabAddons},
        {"4", tabSettings},
    }
    for _, tt := range tests {
        newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
        result := newM.(Model)
        if result.activeTab != tt.want {
            t.Errorf("key %s: got tab %d, want %d", tt.key, result.activeTab, tt.want)
        }
    }
}

// ── Dashboard Navigation ─────────────────────────────────

func TestDashboardCardNavigation(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.activeTab = tabDashboard
    m.dashCard = cardServices

    // Right → System
    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
    m = newM.(Model)
    if m.dashCard != cardSystem {
        t.Errorf("right from services: got %d, want %d (system)", m.dashCard, cardSystem)
    }

    // Down → Lightning
    newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
    m = newM.(Model)
    if m.dashCard != cardLightning {
        t.Errorf("down from system: got %d, want %d (lightning)", m.dashCard, cardLightning)
    }

    // Left → Bitcoin
    newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
    m = newM.(Model)
    if m.dashCard != cardBitcoin {
        t.Errorf("left from lightning: got %d, want %d (bitcoin)", m.dashCard, cardBitcoin)
    }

    // Up → Services
    newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
    m = newM.(Model)
    if m.dashCard != cardServices {
        t.Errorf("up from bitcoin: got %d, want %d (services)", m.dashCard, cardServices)
    }
}

// ── Card Activation ──────────────────────────────────────

func TestEnterActivatesServicesCard(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.activeTab = tabDashboard
    m.dashCard = cardServices

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    m = newM.(Model)
    if !m.cardActive {
        t.Error("enter on services card should activate it")
    }
    if m.svcCursor != 0 {
        t.Errorf("service cursor should start at 0, got %d", m.svcCursor)
    }
}

func TestEnterActivatesSystemCard(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.activeTab = tabDashboard
    m.dashCard = cardSystem

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    m = newM.(Model)
    if !m.cardActive {
        t.Error("enter on system card should activate it")
    }
}

func TestBackspaceDeactivatesCard(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.activeTab = tabDashboard
    m.dashCard = cardServices
    m.cardActive = true

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
    m = newM.(Model)
    if m.cardActive {
        t.Error("backspace should deactivate card")
    }
}

// ── Lightning Card Actions ───────────────────────────────

func TestLightningCardInstallLND(t *testing.T) {
    m := testModel() // no LND installed
    m.width = 80
    m.height = 24
    m.activeTab = tabDashboard
    m.dashCard = cardLightning

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    m = newM.(Model)
    if m.shellAction != svLNDInstall {
        t.Errorf("enter on lightning without LND: got shellAction %d, want %d (svLNDInstall)",
            m.shellAction, svLNDInstall)
    }
}

func TestLightningCardWithLNDShowsDetail(t *testing.T) {
    m := testModelWithLND()
    m.width = 80
    m.height = 24
    m.activeTab = tabDashboard
    m.dashCard = cardLightning

    // LND installed but no wallet — should trigger wallet creation
    // WalletExists() checks for a file that won't exist in tests
    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    m = newM.(Model)
    if m.shellAction != svWalletCreate {
        t.Errorf("enter on lightning with LND no wallet: got shellAction %d, want %d (svWalletCreate)",
            m.shellAction, svWalletCreate)
    }
}

// ── Subview Navigation ───────────────────────────────────

func TestSubviewBackspace(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.subview = svLightning

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
    m = newM.(Model)
    if m.subview != svNone {
        t.Errorf("backspace from subview: got %d, want %d (none)", m.subview, svNone)
    }
}

func TestQRBackspacGoesToZeus(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.subview = svQR

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
    m = newM.(Model)
    if m.subview != svZeus {
        t.Errorf("backspace from QR: got %d, want %d (zeus)", m.subview, svZeus)
    }
}

// ── Tab Switching Resets State ───────────────────────────

func TestTabSwitchResetsCardActive(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.activeTab = tabDashboard
    m.cardActive = true
    m.svcConfirm = "restart"

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
    m = newM.(Model)
    if m.cardActive {
        t.Error("tab switch should deactivate card")
    }
    if m.svcConfirm != "" {
        t.Error("tab switch should clear svcConfirm")
    }
}

// ── Service Count ────────────────────────────────────────

func TestServiceCountBase(t *testing.T) {
    m := testModel()
    if m.svcCount() != 2 {
        t.Errorf("base service count: got %d, want 2 (tor, bitcoind)", m.svcCount())
    }
}

func TestServiceCountWithLND(t *testing.T) {
    m := testModelWithLND()
    if m.svcCount() != 3 {
        t.Errorf("LND service count: got %d, want 3", m.svcCount())
    }
}

func TestServiceCountFullStack(t *testing.T) {
    m := testModelFullStack()
    if m.svcCount() != 5 {
        t.Errorf("full stack service count: got %d, want 5", m.svcCount())
    }
}

// ── Service Names ────────────────────────────────────────

func TestServiceNames(t *testing.T) {
    m := testModelFullStack()
    expected := []string{"tor", "bitcoind", "lnd", "litd", "syncthing"}
    for i, want := range expected {
        got := m.svcName(i)
        if got != want {
            t.Errorf("svcName(%d): got %q, want %q", i, got, want)
        }
    }
}

func TestServiceNameOutOfBounds(t *testing.T) {
    m := testModel()
    got := m.svcName(99)
    if got != "" {
        t.Errorf("svcName(99): got %q, want empty", got)
    }
}

// ── Addons Navigation ────────────────────────────────────

func TestAddonsSyncthingRequiresLND(t *testing.T) {
    m := testModel() // no LND
    m.width = 80
    m.height = 24
    m.activeTab = tabAddons
    m.addonFocus = 0

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    m = newM.(Model)
    // Should not trigger install without LND
    if m.shellAction == svSyncthingInstall {
        t.Error("syncthing install should not trigger without LND")
    }
}

func TestAddonsLITRequiresLND(t *testing.T) {
    m := testModel() // no LND
    m.width = 80
    m.height = 24
    m.activeTab = tabAddons
    m.addonFocus = 1

    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    m = newM.(Model)
    if m.shellAction == svLITInstall {
        t.Error("LIT install should not trigger without LND")
    }
}

// ── Settings ─────────────────────────────────────────────

func TestSettingsUpdateConfirm(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.activeTab = tabSettings
    m.latestVersion = "9.9.9"

    m = handleSettingsKey(m, "enter")
    if !m.updateConfirm {
        t.Error("enter with new version available should set updateConfirm")
    }

    m = handleSettingsKey(m, "n")
    if m.updateConfirm {
        t.Error("pressing n should cancel updateConfirm")
    }
}

func TestSettingsNoUpdateWhenCurrent(t *testing.T) {
    m := testModel()
    m.width = 80
    m.height = 24
    m.activeTab = tabSettings
    m.latestVersion = installer.GetVersion() // matches current version exactly

    m = handleSettingsKey(m, "enter")
    if m.updateConfirm {
        t.Error("should not confirm update when already on latest")
    }
}