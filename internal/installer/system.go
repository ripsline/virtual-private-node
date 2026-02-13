package installer

import (
    "fmt"
    "os"
    "os/exec"
    "os/user"
    "strings"
)

// checkOS verifies we're running on Debian.
func checkOS() error {
    data, err := os.ReadFile("/etc/os-release")
    if err != nil {
        return fmt.Errorf("cannot read /etc/os-release")
    }
    if !strings.Contains(string(data), "ID=debian") {
        return fmt.Errorf("requires Debian 12+")
    }
    return nil
}

// createSystemUser creates the non-login system user that runs
// bitcoind, lnd, and litd services.
func createSystemUser(username string) error {
    if _, err := user.Lookup(username); err == nil {
        return nil
    }
    cmd := exec.Command("adduser",
        "--system", "--group",
        "--home", "/var/lib/bitcoin",
        "--shell", "/usr/sbin/nologin",
        username)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("%s: %s", err, output)
    }
    return nil
}

// createDirs creates FHS-compliant directory structure.
func createDirs(username string, cfg *installConfig) error {
    dirs := []struct {
        path  string
        owner string
        mode  os.FileMode
    }{
        {"/etc/bitcoin", "root:" + username, 0750},
        {"/var/lib/bitcoin", username + ":" + username, 0750},
    }

    if cfg.components == "bitcoin+lnd" {
        dirs = append(dirs,
            struct {
                path  string
                owner string
                mode  os.FileMode
            }{"/etc/lnd", "root:" + username, 0750},
            struct {
                path  string
                owner string
                mode  os.FileMode
            }{"/var/lib/lnd", username + ":" + username, 0750},
        )
    }

    for _, d := range dirs {
        if err := os.MkdirAll(d.path, d.mode); err != nil {
            return fmt.Errorf("mkdir %s: %w", d.path, err)
        }
        cmd := exec.Command("chown", d.owner, d.path)
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("chown %s: %s: %s", d.path, err, output)
        }
        if err := os.Chmod(d.path, d.mode); err != nil {
            return fmt.Errorf("chmod %s: %w", d.path, err)
        }
    }
    return nil
}

// disableIPv6 prevents IPv6 traffic that could bypass Tor.
func disableIPv6() error {
    content := `# Virtual Private Node — disable IPv6
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
net.ipv6.conf.lo.disable_ipv6 = 1
`
    if err := os.WriteFile("/etc/sysctl.d/99-disable-ipv6.conf", []byte(content), 0644); err != nil {
        return err
    }
    cmd := exec.Command("sysctl", "--system")
    cmd.Stdout = nil
    cmd.Stderr = nil
    return cmd.Run()
}

// configureFirewall sets up UFW. Only SSH is always open.
// Port 9735 opens only for LND hybrid P2P mode.
func configureFirewall(cfg *installConfig) error {
    cmd := exec.Command("apt-get", "install", "-y", "-qq", "ufw")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install ufw: %s: %s", err, output)
    }

    ufwDefault, err := os.ReadFile("/etc/default/ufw")
    if err == nil {
        content := strings.ReplaceAll(string(ufwDefault), "IPV6=yes", "IPV6=no")
        os.WriteFile("/etc/default/ufw", []byte(content), 0644)
    }

    commands := [][]string{
        {"ufw", "default", "deny", "incoming"},
        {"ufw", "default", "allow", "outgoing"},
        {"ufw", "allow", "22/tcp"},
    }

    if cfg.components == "bitcoin+lnd" && cfg.p2pMode == "hybrid" {
        commands = append(commands, []string{"ufw", "allow", "9735/tcp"})
    }

    commands = append(commands, []string{"ufw", "--force", "enable"})

    for _, args := range commands {
        cmd := exec.Command(args[0], args[1:]...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("%v: %s: %s", args, err, output)
        }
    }
    return nil
}

// installUnattendedUpgrades installs and configures automatic
// security updates for the Debian system.
func installUnattendedUpgrades() error {
    cmd := exec.Command("apt-get", "install", "-y", "-qq",
        "unattended-upgrades", "apt-listchanges")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install: %s: %s", err, output)
    }
    return nil
}

// configureUnattendedUpgrades writes the config for auto security
// updates with auto-reboot at 4:00 AM UTC when needed.
func configureUnattendedUpgrades() error {
    // Enable auto-updates
    autoConf := `APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
`
    if err := os.WriteFile("/etc/apt/apt.conf.d/20auto-upgrades",
        []byte(autoConf), 0644); err != nil {
        return err
    }

    // Configure what to upgrade and auto-reboot
    upgradeConf := `// Virtual Private Node — Unattended Upgrades
//
// Only install security updates automatically.
// Auto-reboot at 4:00 AM UTC if kernel update requires it.

Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
};

// Auto-reboot if required, at 4 AM UTC
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "04:00";

// Remove unused kernel packages after update
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
`
    return os.WriteFile("/etc/apt/apt.conf.d/50unattended-upgrades",
        []byte(upgradeConf), 0644)
}

func installFail2ban() error {
    cmd := exec.Command("apt-get", "install", "-y", "-qq", "fail2ban")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install fail2ban: %s: %s", err, output)
    }
    return nil
}

func configureFail2ban() error {
    content := `# Virtual Private Node — Fail2ban
#
# Bans IPs after 5 failed SSH attempts for 10 minutes.
# Uses systemd journal backend (default on Debian 12+).

[sshd]
enabled = true
mode = aggressive
port = ssh
maxretry = 5
findtime = 600
bantime = 600
`
    if err := os.WriteFile("/etc/fail2ban/jail.local",
        []byte(content), 0644); err != nil {
        return err
    }
    for _, args := range [][]string{
        {"systemctl", "enable", "fail2ban"},
        {"systemctl", "restart", "fail2ban"},
    } {
        cmd := exec.Command(args[0], args[1:]...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("%v: %s: %s", args, err, output)
        }
    }
    return nil
}