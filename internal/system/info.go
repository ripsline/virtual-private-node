package system

import (
    "fmt"
    "os"
    "os/exec"
    "strings"
)

type DiskInfo struct {
    Total   string
    Used    string
    Percent string
}

type MemInfo struct {
    Total   string
    Used    string
    Percent string
}

func Disk(path string) DiskInfo {
    cmd := exec.Command("df", "-h", "--output=size,used,pcent", path)
    out, _ := cmd.CombinedOutput()
    lines := strings.Split(strings.TrimSpace(string(out)), "\n")
    if len(lines) < 2 {
        return DiskInfo{"N/A", "N/A", "N/A"}
    }
    f := strings.Fields(lines[1])
    if len(f) < 3 {
        return DiskInfo{"N/A", "N/A", "N/A"}
    }
    return DiskInfo{f[0], f[1], f[2]}
}

func Memory() MemInfo {
    data, _ := os.ReadFile("/proc/meminfo")
    var total, avail int
    for _, line := range strings.Split(string(data), "\n") {
        if strings.HasPrefix(line, "MemTotal:") {
            fmt.Sscanf(line, "MemTotal: %d kB", &total)
        }
        if strings.HasPrefix(line, "MemAvailable:") {
            fmt.Sscanf(line, "MemAvailable: %d kB", &avail)
        }
    }
    if total == 0 {
        return MemInfo{"N/A", "N/A", "N/A"}
    }
    used := total - avail
    return MemInfo{
        Total:   fmtKB(total),
        Used:    fmtKB(used),
        Percent: fmt.Sprintf("%.0f%%", float64(used)/float64(total)*100),
    }
}

func DirSize(path string) string {
    out, err := exec.Command("sudo", "du", "-sh", path).CombinedOutput()
    if err != nil {
        return "N/A"
    }
    f := strings.Fields(string(out))
    if len(f) < 1 {
        return "N/A"
    }
    return f[0]
}

func IsServiceActive(name string) bool {
    return exec.Command("systemctl", "is-active", "--quiet", name).Run() == nil
}

func ServiceAction(name, action string) error {
    return SudoRun("systemctl", action, name)
}

func RebootRequired() bool {
    _, err := os.Stat("/var/run/reboot-required")
    return err == nil
}

func PublicIPv4() string {
    ip, err := RunContext(5e9, "curl", "-4", "-s", "--max-time", "5", "ifconfig.me")
    if err != nil {
        return ""
    }
    if len(strings.Split(ip, ".")) != 4 {
        return ""
    }
    return ip
}

func fmtKB(kb int) string {
    if kb >= 1048576 {
        return fmt.Sprintf("%.1f GB", float64(kb)/1048576.0)
    }
    return fmt.Sprintf("%.0f MB", float64(kb)/1024.0)
}