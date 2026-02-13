// Package installer — syncthing.go
//
// Installs Syncthing via apt, configures it as a systemd service,
// sets up Tor hidden service, auto-configures LND channel backup
// sync via systemd path watcher.
package installer

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
)

// installSyncthingRepo adds the Syncthing apt repository and signing key.
func installSyncthingRepo() error {
    // Create keyrings directory
    os.MkdirAll("/etc/apt/keyrings", 0755)

    // Download release key
    cmd := exec.Command("curl", "-L", "-o",
        "/etc/apt/keyrings/syncthing-archive-keyring.gpg",
        "https://syncthing.net/release-key.gpg")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("download syncthing key: %s: %s", err, output)
    }

    // Add stable repository
    repoLine := `deb [signed-by=/etc/apt/keyrings/syncthing-archive-keyring.gpg] https://apt.syncthing.net/ syncthing stable-v2`
    if err := os.WriteFile("/etc/apt/sources.list.d/syncthing.list",
        []byte(repoLine+"\n"), 0644); err != nil {
        return err
    }

    return nil
}

// installSyncthingPackage runs apt update and installs syncthing.
func installSyncthingPackage() error {
    cmd := exec.Command("apt-get", "update", "-qq")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("apt update: %s: %s", err, output)
    }

    cmd = exec.Command("apt-get", "install", "-y", "-qq", "syncthing")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install syncthing: %s: %s", err, output)
    }

    return nil
}

// createSyncthingDirs creates data and config directories.
func createSyncthingDirs() error {
    dirs := []struct {
        path  string
        owner string
        mode  os.FileMode
    }{
        {"/var/lib/syncthing", systemUser + ":" + systemUser, 0750},
        {"/var/lib/syncthing/lnd-backup", systemUser + ":" + systemUser, 0750},
        {"/etc/syncthing", systemUser + ":" + systemUser, 0750},
    }

    for _, d := range dirs {
        if err := os.MkdirAll(d.path, d.mode); err != nil {
            return err
        }
        exec.Command("chown", d.owner, d.path).Run()
        os.Chmod(d.path, d.mode)
    }

    return nil
}

// writeSyncthingService creates the systemd service for Syncthing.
func writeSyncthingService() error {
    content := fmt.Sprintf(`[Unit]
Description=Syncthing File Synchronization
After=network-online.target tor.service
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=/usr/bin/syncthing serve --no-browser --no-restart --home=/etc/syncthing --data=/var/lib/syncthing
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

// configureSyncthingAuth generates and sets the web UI password.
// Syncthing needs to start once to generate its config, then we
// modify the config to set the password.
func configureSyncthingAuth(password string) error {
    // Ensure syncthing can write to its config directory
    exec.Command("chown", systemUser+":"+systemUser, "/etc/syncthing").Run()
    exec.Command("chmod", "750", "/etc/syncthing").Run()

    // Generate default config as the bitcoin user
    cmd := exec.Command("sudo", "-u", systemUser, "syncthing",
        "generate", "--home=/etc/syncthing")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("syncthing generate failed: %s: %s", err, output)
    }

    // Read the generated config
    configPath := "/etc/syncthing/config.xml"
    data, err := os.ReadFile(configPath)
    if err != nil {
        return fmt.Errorf("read syncthing config: %w", err)
    }

    content := string(data)

    // Set the GUI to listen on localhost only
    content = strings.Replace(content,
        "<address>127.0.0.1:8384</address>",
        "<address>127.0.0.1:8384</address>", 1)

    // If default address is 0.0.0.0, change to localhost
    content = strings.Replace(content,
        "<address>0.0.0.0:8384</address>",
        "<address>127.0.0.1:8384</address>", 1)

    // Generate bcrypt hash for the password
    // Syncthing accepts plaintext passwords in the config too
    // with the format: <password>plaintext</password>
    // but we should set user as well
    content = strings.Replace(content,
        "<user></user>",
        "<user>admin</user>", 1)

    // Set plaintext password (Syncthing will hash it on first start)
    if strings.Contains(content, "<password></password>") {
        content = strings.Replace(content,
            "<password></password>",
            fmt.Sprintf("<password>%s</password>", password), 1)
    }

    if err := os.WriteFile(configPath, []byte(content), 0640); err != nil {
        return err
    }

    exec.Command("chown", "root:"+systemUser, configPath).Run()
    return nil
}

// setupChannelBackupWatcher creates a systemd path unit that
// watches the LND channel.backup file and copies it to the
// Syncthing sync folder whenever it changes.
func setupChannelBackupWatcher(cfg *config.AppConfig) error {
    network := cfg.Network
    if cfg.IsMainnet() {
        network = "mainnet"
    }

    backupSource := fmt.Sprintf(
        "/var/lib/lnd/data/chain/bitcoin/%s/channel.backup", network)
    backupDest := "/var/lib/syncthing/lnd-backup/channel.backup"

    // Create the path watcher unit
    pathUnit := fmt.Sprintf(`[Unit]
Description=Watch LND channel backup for changes

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

    // Create the copy service
    copyService := fmt.Sprintf(`[Unit]
Description=Copy LND channel backup to Syncthing folder

[Service]
Type=oneshot
User=%s
ExecStart=/bin/cp %s %s
`, systemUser, backupSource, backupDest)

    if err := os.WriteFile("/etc/systemd/system/lnd-backup-copy.service",
        []byte(copyService), 0644); err != nil {
        return err
    }

    // Enable and start the path watcher
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

    // Do an initial copy if the backup file exists
    if _, err := os.Stat(backupSource); err == nil {
        exec.Command("cp", backupSource, backupDest).Run()
        exec.Command("chown", systemUser+":"+systemUser, backupDest).Run()
    }

    return nil
}

// addSyncthingTorService adds a Tor hidden service for the
// Syncthing web UI. Port 8384, HTTP (no self-signed cert issue).
func addSyncthingTorService() error {
    data, err := os.ReadFile("/etc/tor/torrc")
    if err != nil {
        return err
    }

    if strings.Contains(string(data), "syncthing") {
        return nil // already configured
    }

    addition := `
# Syncthing web UI (Tor only, HTTP)
HiddenServiceDir /var/lib/tor/syncthing/
HiddenServicePort 8384 127.0.0.1:8384
`
    return os.WriteFile("/etc/tor/torrc",
        append(data, []byte(addition)...), 0644)
}

// startSyncthing enables and starts the Syncthing service.
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

// RunSyncthingInstall is called from the Software tab.
func RunSyncthingInstall(cfg *config.AppConfig) error {
    // Generate credentials
    passBytes := make([]byte, 12)
    rand.Read(passBytes)
    syncPassword := hex.EncodeToString(passBytes)

    // Confirmation box
    confirmMsg := setupTitleStyle.Render("Install Syncthing") + "\n\n" +
        setupTextStyle.Render("This will:") + "\n\n" +
        setupTextStyle.Render("  • Install Syncthing from official repository") + "\n" +
        setupTextStyle.Render("  • Create Tor hidden service for web UI") + "\n" +
        setupTextStyle.Render("  • Auto-configure LND channel backup sync") + "\n" +
        setupTextStyle.Render("  • Restart Tor") + "\n\n" +
        setupDimStyle.Render("Press Enter to proceed...")
    showInfoBox(confirmMsg)

    steps := []installStep{
        {name: "Adding Syncthing repository",
            fn: installSyncthingRepo},
        {name: "Installing Syncthing",
            fn: installSyncthingPackage},
        {name: "Creating Syncthing directories",
            fn: createSyncthingDirs},
        {name: "Creating Syncthing service",
            fn: writeSyncthingService},
        {name: "Configuring Syncthing authentication",
            fn: func() error { return configureSyncthingAuth(syncPassword) }},
        {name: "Configuring Tor for Syncthing",
            fn: addSyncthingTorService},
        {name: "Restarting Tor",
            fn: restartTor},
        {name: "Starting Syncthing",
            fn: startSyncthing},
        {name: "Setting up channel backup watcher",
            fn: func() error { return setupChannelBackupWatcher(cfg) }},
    }

    if err := runInstallTUI(steps, appVersion); err != nil {
        return err
    }

    cfg.SyncthingInstalled = true
    cfg.SyncthingPassword = syncPassword
    return config.Save(cfg)
}