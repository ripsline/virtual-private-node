package welcome

import (
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"
)

func readOnion(path string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(data))
}

func readMacaroonHex(cfg *config.AppConfig) string {
    network := cfg.Network
    if cfg.IsMainnet() {
        network = "mainnet"
    }
    path := fmt.Sprintf(
        "/var/lib/lnd/data/chain/bitcoin/%s/admin.macaroon", network)
    data, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return hex.EncodeToString(data)
}

func printMacaroon(cfg *config.AppConfig) {
    mac := readMacaroonHex(cfg)
    fmt.Print("\033[2J\033[H")
    fmt.Println()
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println("    Admin Macaroon (hex)")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println()
    if mac == "" {
        fmt.Println("  Not available.")
    } else {
        fmt.Println(mac)
    }
    fmt.Println()
    fmt.Print("  Press Enter to return...")
    fmt.Scanln()
    fmt.Print("\033[2J\033[H")
}

func getSyncthingVersion() string {
    output, err := system.RunContext(3*time.Second, "syncthing", "--version")
    if err != nil {
        return "unknown"
    }
    fields := strings.Fields(output)
    if len(fields) >= 2 {
        return fields[1]
    }
    return "unknown"
}

func runSystemUpdate() {
    fmt.Print("\033[2J\033[H")
    fmt.Println("\n  ═══════════════════════════════════════════")
    fmt.Println("    System Update")
    fmt.Println("  ═══════════════════════════════════════════")
    fmt.Println()
    fmt.Println("  Running apt update && apt upgrade...")
    fmt.Println()

    updateCmd := exec.Command("sudo", "apt-get", "update")
    updateCmd.Stdout = os.Stdout
    updateCmd.Stderr = os.Stderr
    updateCmd.Run()

    cmd := exec.Command("sudo", "apt-get", "upgrade", "-y")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    fmt.Println("\n  ✅ Update complete")
    if system.RebootRequired() {
        fmt.Println("\n  ⚠️ Reboot required.")
        fmt.Print("  Reboot now? [y/N]: ")
        var ans string
        fmt.Scanln(&ans)
        if strings.ToLower(ans) == "y" {
            system.SudoRun("reboot")
        }
    }
    fmt.Print("\n  Press Enter to return...")
    fmt.Scanln()
    fmt.Print("\033[2J\033[H")
}

func runLogViewer(svcName string, cfg *config.AppConfig) {
    fmt.Print("\033[2J\033[H")
    fmt.Printf("\n  ═══════════════════════════════════════════\n")
    fmt.Printf("    %s Logs (last 100 lines)\n", svcName)
    fmt.Printf("  ═══════════════════════════════════════════\n\n")

    cmd := exec.Command("sudo", "journalctl", "-u", svcName, "-n", "100", "--no-pager")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    fmt.Print("\n  Press Enter to return...")
    fmt.Scanln()
    fmt.Print("\033[2J\033[H")
}