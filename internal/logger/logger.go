// internal/logger/logger.go

// Package logger provides structured logging for the rlvpn application.
//
// Log file: /var/log/rlvpn.log (created during bootstrap, owned by ripsline)
//
// Log format:
//
//	[2026-02-21 14:30:05 UTC] [section] message
//
// Sections:
//
//	[install]  — installation steps and progress
//	[verify]   — GPG signature and checksum verification
//	[config]   — configuration load/save operations
//	[tui]      — TUI actions (installs, upgrades, service management)
//	[status]   — dashboard status polling warnings
//	[system]   — system-level operations (firewall, tor, services)
package logger

import (
	"fmt"
	"os"
	"time"
)

const LogPath = "/var/log/rlvpn.log"

func Log(section, format string, args ...interface{}) {
	entry := fmt.Sprintf("[%s] [%s] %s\n",
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		section,
		fmt.Sprintf(format, args...))
	f, err := os.OpenFile(LogPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
}

func Install(format string, args ...interface{}) {
	Log("install", format, args...)
}

func Verify(format string, args ...interface{}) {
	Log("verify", format, args...)
}

func Config(format string, args ...interface{}) {
	Log("config", format, args...)
}

func TUI(format string, args ...interface{}) {
	Log("tui", format, args...)
}

func Status(format string, args ...interface{}) {
	Log("status", format, args...)
}

func System(format string, args ...interface{}) {
	Log("system", format, args...)
}
