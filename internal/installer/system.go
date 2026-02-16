package installer

import (
    "fmt"
    "os"
    "os/user"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"
)

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

func createSystemUser(username string) error {
    if _, err := user.Lookup(username); err == nil {
        return nil
    }
    return system.Run("adduser",
        "--system", "--group",
        "--home", "/var/lib/bitcoin",
        "--shell", "/usr/sbin/nologin",
        username)
}

func createBitcoinDirs(username string) error {
    dirs := []struct {
        path  string
        owner string
        mode  os.FileMode
    }{
        {"/etc/bitcoin", "root:" + username, 0750},
        {"/var/lib/bitcoin", username + ":" + username, 0750},
    }
    for _, d := range dirs {
        if err := os.MkdirAll(d.path, d.mode); err != nil {
            return fmt.Errorf("mkdir %s: %w", d.path, err)
        }
        if err := system.Run("chown", d.owner, d.path); err != nil {
            return err
        }
        if err := os.Chmod(d.path, d.mode); err != nil {
            return fmt.Errorf("chmod %s: %w", d.path, err)
        }
    }
    return nil
}

func disableIPv6() error {
    content := `# Virtual Private Node — disable IPv6
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
net.ipv6.conf.lo.disable_ipv6 = 1
`
    if err := os.WriteFile("/etc/sysctl.d/99-disable-ipv6.conf", []byte(content), 0644); err != nil {
        return err
    }
    return system.RunSilent("sysctl", "--system")
}

func configureFirewall(cfg *config.AppConfig) error {
    if err := system.Run("apt-get", "install", "-y", "-qq", "ufw"); err != nil {
        return err
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

    if cfg.HasLND() && cfg.P2PMode == "hybrid" {
        commands = append(commands, []string{"ufw", "allow", "9735/tcp"})
    }

    commands = append(commands, []string{"ufw", "--force", "enable"})

    for _, args := range commands {
        if err := system.Run(args[0], args[1:]...); err != nil {
            return err
        }
    }
    return nil
}

func installUnattendedUpgrades() error {
    return system.Run("apt-get", "install", "-y", "-qq",
        "unattended-upgrades", "apt-listchanges")
}

func configureUnattendedUpgrades() error {
    autoConf := `APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
`
    if err := os.WriteFile("/etc/apt/apt.conf.d/20auto-upgrades",
        []byte(autoConf), 0644); err != nil {
        return err
    }

    upgradeConf := `// Virtual Private Node — Unattended Upgrades
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
};
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "04:00";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
`
    return os.WriteFile("/etc/apt/apt.conf.d/50unattended-upgrades",
        []byte(upgradeConf), 0644)
}

func installFail2ban() error {
    return system.Run("apt-get", "install", "-y", "-qq", "fail2ban")
}

func configureFail2ban() error {
    content := `# Virtual Private Node — Fail2ban
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
    if err := system.Run("systemctl", "enable", "fail2ban"); err != nil {
        return err
    }
    return system.Run("systemctl", "restart", "fail2ban")
}