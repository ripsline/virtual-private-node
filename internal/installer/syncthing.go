package installer

import (
    "encoding/xml"
    "fmt"
    "os"

    "golang.org/x/crypto/bcrypt"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"
)

// ── Syncthing XML config types ───────────────────────────

type syncthingConfig struct {
    XMLName xml.Name      `xml:"configuration"`
    GUI     syncthingGUI  `xml:"gui"`
    Options syncthingOpts `xml:"options"`
    Rest    []byte        `xml:",innerxml"`
}

type syncthingGUI struct {
    Enabled               string `xml:"enabled,attr"`
    TLS                   string `xml:"tls,attr"`
    Address               string `xml:"address"`
    User                  string `xml:"user,omitempty"`
    Password              string `xml:"password,omitempty"`
    InsecureSkipHostcheck bool   `xml:"insecureSkipHostcheck"`
    APIKey                string `xml:"apikey,omitempty"`
    Theme                 string `xml:"theme,omitempty"`
}

type syncthingOpts struct {
    ListenAddresses       []string `xml:"listenAddress"`
    GlobalAnnounceEnabled bool     `xml:"globalAnnounceEnabled"`
    LocalAnnounceEnabled  bool     `xml:"localAnnounceEnabled"`
    RelaysEnabled         bool     `xml:"relaysEnabled"`
    NATEnabled            bool     `xml:"natEnabled"`
    Rest                  []byte   `xml:",innerxml"`
}

func installSyncthingRepo() error {
    system.SudoRun("mkdir", "-p", "/etc/apt/keyrings")
    if err := system.SudoRun("curl", "-L", "-o",
        "/etc/apt/keyrings/syncthing-archive-keyring.gpg",
        "https://syncthing.net/release-key.gpg"); err != nil {
        return err
    }
    repoLine := `deb [signed-by=/etc/apt/keyrings/syncthing-archive-keyring.gpg] https://apt.syncthing.net/ syncthing stable-v2`
    return system.SudoWriteFile("/etc/apt/sources.list.d/syncthing.list",
        []byte(repoLine+"\n"), 0644)
}

func installSyncthingPackage() error {
    if err := system.SudoRun("apt-get", "update", "-qq"); err != nil {
        return err
    }
    return system.SudoRun("apt-get", "install", "-y", "-qq", "syncthing")
}

func createSyncthingDirs() error {
    dirs := []struct {
        path  string
        owner string
        mode  os.FileMode
    }{
        {"/etc/syncthing", systemUser + ":" + systemUser, 0750},
        {"/var/lib/syncthing", systemUser + ":" + systemUser, 0750},
        {"/var/lib/syncthing/lnd-backup", systemUser + ":" + systemUser, 0750},
    }
    for _, d := range dirs {
        if err := system.SudoRun("mkdir", "-p", d.path); err != nil {
            return err
        }
        if err := system.SudoRun("chown", d.owner, d.path); err != nil {
            return err
        }
        if err := system.SudoRun("chmod", fmt.Sprintf("%o", d.mode), d.path); err != nil {
            return err
        }
    }
    return nil
}

func writeSyncthingService() error {
    content := fmt.Sprintf(`[Unit]
Description=Syncthing File Synchronization
After=network-online.target tor.service
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=/usr/bin/syncthing serve --no-browser --no-restart --config=/etc/syncthing --data=/var/lib/syncthing
Restart=on-failure
RestartSec=10
SuccessExitStatus=3 4
RestartForceExitStatus=3 4

[Install]
WantedBy=multi-user.target
`, systemUser, systemUser)
    return system.SudoWriteFile("/etc/systemd/system/syncthing.service",
        []byte(content), 0644)
}

func configureSyncthingAuth(password string) error {
    system.SudoRunSilent("chown", systemUser+":"+systemUser, "/etc/syncthing")

    if err := system.SudoRun("sudo", "-u", systemUser, "syncthing",
        "generate", "--home=/etc/syncthing"); err != nil {
        return fmt.Errorf("syncthing generate: %w", err)
    }

    configPath := "/etc/syncthing/config.xml"
    output, err := system.SudoRunOutput("cat", configPath)
    if err != nil {
        return fmt.Errorf("read config: %w", err)
    }

    var cfg syncthingConfig
    if err := xml.Unmarshal([]byte(output), &cfg); err != nil {
        return fmt.Errorf("parse config: %w", err)
    }

    // Hash password
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return fmt.Errorf("hash password: %w", err)
    }

    // Configure GUI
    cfg.GUI.Address = "127.0.0.1:8384"
    cfg.GUI.User = "admin"
    cfg.GUI.Password = string(hash)
    cfg.GUI.InsecureSkipHostcheck = true

    // Disable discovery and relays
    cfg.Options.GlobalAnnounceEnabled = false
    cfg.Options.LocalAnnounceEnabled = false
    cfg.Options.RelaysEnabled = false
    cfg.Options.NATEnabled = false
    cfg.Options.ListenAddresses = []string{"tcp://127.0.0.1:22000"}

    // Marshal back
    xmlOutput, err := xml.MarshalIndent(cfg, "", "    ")
    if err != nil {
        return fmt.Errorf("marshal config: %w", err)
    }

    xmlHeader := []byte(xml.Header)
    xmlOutput = append(xmlHeader, xmlOutput...)

    if err := system.SudoWriteFile(configPath, xmlOutput, 0640); err != nil {
        return err
    }
    return system.SudoRun("chown", systemUser+":"+systemUser, configPath)
}

func setupChannelBackupWatcher(cfg *config.AppConfig) error {
    network := cfg.Network
    if cfg.IsMainnet() {
        network = "mainnet"
    }
    backupSource := fmt.Sprintf(
        "/var/lib/lnd/data/chain/bitcoin/%s/channel.backup", network)
    backupDest := "/var/lib/syncthing/lnd-backup/channel.backup"

    pathUnit := fmt.Sprintf(`[Unit]
Description=Watch LND channel backup

[Path]
PathChanged=%s
Unit=lnd-backup-copy.service

[Install]
WantedBy=multi-user.target
`, backupSource)
    if err := system.SudoWriteFile("/etc/systemd/system/lnd-backup-watch.path",
        []byte(pathUnit), 0644); err != nil {
        return err
    }

    copyService := fmt.Sprintf(`[Unit]
Description=Copy LND channel backup

[Service]
Type=oneshot
User=%s
ExecStart=/bin/cp %s %s
`, systemUser, backupSource, backupDest)
    if err := system.SudoWriteFile("/etc/systemd/system/lnd-backup-copy.service",
        []byte(copyService), 0644); err != nil {
        return err
    }

    if err := system.SudoRun("systemctl", "daemon-reload"); err != nil {
        return err
    }
    if err := system.SudoRun("systemctl", "enable", "lnd-backup-watch.path"); err != nil {
        return err
    }
    if err := system.SudoRun("systemctl", "start", "lnd-backup-watch.path"); err != nil {
        return err
    }

    // Copy existing backup if present
    system.SudoRunSilent("cp", backupSource, backupDest)
    system.SudoRunSilent("chown", systemUser+":"+systemUser, backupDest)
    return nil
}

func startSyncthing() error {
    if err := system.SudoRun("systemctl", "daemon-reload"); err != nil {
        return err
    }
    if err := system.SudoRun("systemctl", "enable", "syncthing"); err != nil {
        return err
    }
    return system.SudoRun("systemctl", "start", "syncthing")
}