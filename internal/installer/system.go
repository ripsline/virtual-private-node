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
    return system.SudoRun("adduser",
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
        if err := system.SudoRun("mkdir", "-p", d.path); err != nil {
            return fmt.Errorf("mkdir %s: %w", d.path, err)
        }
        if err := system.SudoRun("chown", d.owner, d.path); err != nil {
            return err
        }
        if err := system.SudoRun("chmod", fmt.Sprintf("%o", d.mode), d.path); err != nil {
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
    if err := system.SudoWriteFile("/etc/sysctl.d/99-disable-ipv6.conf", []byte(content), 0644); err != nil {
        return err
    }
    return system.SudoRunSilent("sysctl", "--system")
}

func configureFirewall(cfg *config.AppConfig) error {
    if err := system.SudoRun("apt-get", "install", "-y", "-qq", "ufw"); err != nil {
        return err
    }

    ufwDefault, err := system.SudoRunOutput("cat", "/etc/default/ufw")
    if err == nil {
        content := strings.ReplaceAll(ufwDefault, "IPV6=yes", "IPV6=no")
        system.SudoWriteFile("/etc/default/ufw", []byte(content), 0644)
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
        if err := system.SudoRun(args[0], args[1:]...); err != nil {
            return err
        }
    }
    return nil
}

func installUnattendedUpgrades() error {
    return system.SudoRun("apt-get", "install", "-y", "-qq",
        "unattended-upgrades", "apt-listchanges")
}

func configureUnattendedUpgrades() error {
    autoConf := `APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
`
    if err := system.SudoWriteFile("/etc/apt/apt.conf.d/20auto-upgrades",
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
    return system.SudoWriteFile("/etc/apt/apt.conf.d/50unattended-upgrades",
        []byte(upgradeConf), 0644)
}

func installFail2ban() error {
    return system.SudoRun("apt-get", "install", "-y", "-qq", "fail2ban")
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
    if err := system.SudoWriteFile("/etc/fail2ban/jail.local",
        []byte(content), 0644); err != nil {
        return err
    }
    if err := system.SudoRun("systemctl", "enable", "fail2ban"); err != nil {
        return err
    }
    return system.SudoRun("systemctl", "restart", "fail2ban")
}