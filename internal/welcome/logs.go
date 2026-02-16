package welcome

import (
    "fmt"
    "os"
    "os/exec"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/theme"
)

func (m Model) viewLogs(bw int) string {
    var lines []string
    lines = append(lines, theme.Header.Render("Select a service to view logs"))
    lines = append(lines, "")

    type svc struct {
        name string
        sel  logSelection
    }
    services := []svc{{"Tor", logSelTor}, {"Bitcoin Core", logSelBitcoin}}
    if m.cfg.HasLND() {
        services = append(services, svc{"LND", logSelLND})
    }
    if m.cfg.LITInstalled {
        services = append(services, svc{"Lightning Terminal", logSelLIT})
    }
    if m.cfg.SyncthingInstalled {
        services = append(services, svc{"Syncthing", logSelSyncthing})
    }

    for _, s := range services {
        prefix, style := "  ", theme.Value
        if m.logSel == s.sel {
            prefix, style = "▸ ", theme.Action
        }
        lines = append(lines, style.Render(prefix+s.name))
    }
    lines = append(lines, "")
    lines = append(lines, theme.Dim.Render("Enter to view"))

    return theme.Box.Width(bw).Padding(1, 2).Render(padLines(lines, theme.BoxHeight))
}

func runLogViewer(sel logSelection, cfg *config.AppConfig) {
    svcMap := map[logSelection]string{
        logSelTor: "tor", logSelBitcoin: "bitcoind",
        logSelLND: "lnd", logSelLIT: "litd",
        logSelSyncthing: "syncthing",
    }
    svc := svcMap[sel]
    fmt.Print("\033[2J\033[H")
    fmt.Printf("\n  ═══════════════════════════════════════════\n")
    fmt.Printf("    %s Logs (last 100 lines)\n", svc)
    fmt.Printf("  ═══════════════════════════════════════════\n\n")

    cmd := exec.Command("journalctl", "-u", svc, "-n", "100", "--no-pager")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    fmt.Print("\n  Press Enter to return...")
    fmt.Scanln()
    fmt.Print("\033[2J\033[H")
}