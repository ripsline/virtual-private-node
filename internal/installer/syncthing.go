package installer

import (
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
)

func installSyncthingRepo() error {
    os.MkdirAll("/etc/apt/keyrings", 0755)
    cmd := exec.Command("curl", "-L", "-o",
        "/etc/apt/keyrings/syncthing-archive-keyring.gpg",
        "https://syncthing.net/release-key.gpg")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("download key: %s: %s", err, output)
    }
    repoLine := `deb [signed-by=/etc/apt/keyrings/syncthing-archive-keyring.gpg] https://apt.syncthing.net/ syncthing stable-v2`
    return os.WriteFile("/etc/apt/sources.list.d/syncthing.list",
        []byte(repoLine+"\n"), 0644)
}

func installSyncthingPackage() error {
    cmd := exec.Command("apt-get", "update", "-qq")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("apt update: %s: %s", err, output)
    }
    cmd = exec.Command("apt-get", "install", "-y", "-qq", "syncthing")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install: %s: %s", err, output)
    }
    return nil
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
        if err := os.MkdirAll(d.path, d.mode); err != nil {
            return err
        }
        if output, err := exec.Command("chown", d.owner, d.path).CombinedOutput(); err != nil {
            return fmt.Errorf("chown %s: %s: %s", d.path, err, output)
        }
        os.Chmod(d.path, d.mode)
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
    return os.WriteFile("/etc/systemd/system/syncthing.service",
        []byte(content), 0644)
}

func configureSyncthingAuth(password string) error {
    // Ensure config directory is writable
    exec.Command("chown", systemUser+":"+systemUser, "/etc/syncthing").Run()

    // Generate default config
    cmd := exec.Command("sudo", "-u", systemUser, "syncthing",
        "generate", "--home=/etc/syncthing")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("syncthing generate: %s: %s", err, output)
    }

    configPath := "/etc/syncthing/config.xml"
    data, err := os.ReadFile(configPath)
    if err != nil {
        return fmt.Errorf("read config: %w", err)
    }

    content := string(data)
    content = strings.Replace(content,
        "<address>0.0.0.0:8384</address>",
        "<address>127.0.0.1:8384</address>", 1)
    // Allow access via Tor onion address (skip host header check)
    content = strings.Replace(content,
        "<insecureSkipHostcheck>false</insecureSkipHostcheck>",
        "<insecureSkipHostcheck>true</insecureSkipHostcheck>", 1)
    content = strings.Replace(content,
        "<user></user>",
        "<user>admin</user>", 1)
    if strings.Contains(content, "<password></password>") {
        content = strings.Replace(content,
            "<password></password>",
            fmt.Sprintf("<password>%s</password>", password), 1)
    }

    if err := os.WriteFile(configPath, []byte(content), 0640); err != nil {
        return err
    }
    exec.Command("chown", systemUser+":"+systemUser, configPath).Run()
    return nil
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
    if err := os.WriteFile("/etc/systemd/system/lnd-backup-watch.path",
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
    if err := os.WriteFile("/etc/systemd/system/lnd-backup-copy.service",
        []byte(copyService), 0644); err != nil {
        return err
    }

    for _, args := range [][]string{
        {"systemctl", "daemon-reload"},
        {"systemctl", "enable", "lnd-backup-watch.path"},
        {"systemctl", "start", "lnd-backup-watch.path"},
    } {
        cmd := exec.Command(args[0], args[1:]...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("%v: %s: %s", args, err, output)
        }
    }

    if _, err := os.Stat(backupSource); err == nil {
        exec.Command("cp", backupSource, backupDest).Run()
        exec.Command("chown", systemUser+":"+systemUser, backupDest).Run()
    }
    return nil
}

func addSyncthingTorService() error {
    data, err := os.ReadFile("/etc/tor/torrc")
    if err != nil {
        return err
    }
    if strings.Contains(string(data), "syncthing") {
        return nil
    }
    addition := `
# Syncthing web UI (Tor only, HTTP)
HiddenServiceDir /var/lib/tor/syncthing/
HiddenServicePort 8384 127.0.0.1:8384
`
    return os.WriteFile("/etc/tor/torrc", append(data, []byte(addition)...), 0644)
}

func startSyncthing() error {
    for _, args := range [][]string{
        {"systemctl", "daemon-reload"},
        {"systemctl", "enable", "syncthing"},
        {"systemctl", "start", "syncthing"},
    } {
        cmd := exec.Command(args[0], args[1:]...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("%v: %s: %s", args, err, output)
        }
    }
    return nil
}