// internal/welcome/helpers.go

package welcome

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ripsline/virtual-private-node/internal/config"
	"github.com/ripsline/virtual-private-node/internal/logger"
	"github.com/ripsline/virtual-private-node/internal/paths"
	"github.com/ripsline/virtual-private-node/internal/system"
)

func readOnion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		output, err := system.SudoRunOutput("cat", path)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(output)
	}
	return strings.TrimSpace(string(data))
}

func readMacaroonHex(cfg *config.AppConfig) string {
	network := cfg.Network
	if cfg.IsMainnet() {
		network = "mainnet"
	}
	path := paths.LNDMacaroon(network)

	// Try direct read first
	data, err := os.ReadFile(path)
	if err != nil {
		// Fallback: sudo read (safe for binary files, no shell)
		data, err = system.SudoReadFile(path)
		if err != nil {
			logger.Status("Warning: failed to read macaroon: %v", err)
			return ""
		}
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
