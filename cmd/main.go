package main

import (
    "fmt"
    "os"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/installer"
    "github.com/ripsline/virtual-private-node/internal/welcome"
)

const version = "0.1.0"

func main() {
    if !installer.NeedsInstall() {
        cfg, err := config.Load()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
            cfg = config.Default()
        }
        // Launch the TUI welcome screen
        welcome.Show(cfg, version)
        return
    }

    if os.Geteuid() != 0 {
        fmt.Println("ERROR: Installer must run as root.")
        fmt.Println("Run with: sudo rlvpn")
        os.Exit(1)
    }

    fmt.Printf("\n  Virtual Private Node v%s\n\n", version)

    if err := installer.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "\n  Installation failed: %v\n", err)
        os.Exit(1)
    }
}