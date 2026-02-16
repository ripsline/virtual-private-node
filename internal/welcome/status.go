package welcome

import (
    "github.com/ripsline/virtual-private-node/internal/bitcoin"
    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"

    tea "github.com/charmbracelet/bubbletea"
)

func fetchStatus(cfg *config.AppConfig) tea.Cmd {
    return func() tea.Msg {
        s := statusMsg{services: make(map[string]bool)}

        names := []string{"tor", "bitcoind"}
        if cfg.HasLND() {
            names = append(names, "lnd")
        }
        if cfg.LITInstalled {
            names = append(names, "litd")
        }
        if cfg.SyncthingInstalled {
            names = append(names, "syncthing")
        }
        for _, name := range names {
            s.services[name] = system.IsServiceActive(name)
        }

        disk := system.Disk("/")
        s.diskTotal = disk.Total
        s.diskUsed = disk.Used
        s.diskPct = disk.Percent

        mem := system.Memory()
        s.ramTotal = mem.Total
        s.ramUsed = mem.Used
        s.ramPct = mem.Percent

        s.btcSize = system.DirSize("/var/lib/bitcoin")
        if cfg.HasLND() {
            s.lndSize = system.DirSize("/var/lib/lnd")
        }

        s.rebootRequired = system.RebootRequired()

        info := bitcoin.GetBlockchainInfo("/var/lib/bitcoin", "/etc/bitcoin/bitcoin.conf")
        s.btcResponding = info.Responding
        s.btcBlocks = info.Blocks
        s.btcHeaders = info.Headers
        s.btcProgress = info.Progress
        s.btcSynced = info.Synced

        return s
    }
}