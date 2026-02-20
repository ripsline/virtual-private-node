package system

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "time"
)

// Run executes a command and returns an error with output on failure.
func Run(name string, args ...string) error {
    cmd := exec.Command(name, args...)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("%s %s: %s: %s", name, strings.Join(args, " "), err, output)
    }
    return nil
}

// SudoRun executes a command via sudo.
func SudoRun(name string, args ...string) error {
    sudoArgs := append([]string{name}, args...)
    return Run("sudo", sudoArgs...)
}

// RunOutput executes a command and returns stdout as a string.
func RunOutput(name string, args ...string) (string, error) {
    cmd := exec.Command(name, args...)
    cmd.Stderr = nil
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
    }
    return strings.TrimSpace(string(output)), nil
}

// SudoRunOutput executes a command via sudo and returns stdout.
func SudoRunOutput(name string, args ...string) (string, error) {
    sudoArgs := append([]string{name}, args...)
    return RunOutput("sudo", sudoArgs...)
}

// RunContext executes a command with a timeout.
func RunContext(timeout time.Duration, name string, args ...string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.Stderr = nil
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
    }
    return strings.TrimSpace(string(output)), nil
}

// SudoRunContext executes a command via sudo with a timeout.
func SudoRunContext(timeout time.Duration, name string, args ...string) (string, error) {
    sudoArgs := append([]string{name}, args...)
    return RunContext(timeout, "sudo", sudoArgs...)
}

// RunSilent executes a command and discards all output.
func RunSilent(name string, args ...string) error {
    cmd := exec.Command(name, args...)
    cmd.Stdout = nil
    cmd.Stderr = nil
    return cmd.Run()
}

// SudoRunSilent executes a command via sudo and discards all output.
func SudoRunSilent(name string, args ...string) error {
    sudoArgs := append([]string{name}, args...)
    return RunSilent("sudo", sudoArgs...)
}

// SudoWriteFile writes content to a system path via sudo.
// Writes to /tmp first, then sudo copies to the destination.
func SudoWriteFile(path string, content []byte, perm os.FileMode) error {
    tmpFile := fmt.Sprintf("/tmp/rlvpn-%d.tmp", os.Getpid())
    if err := os.WriteFile(tmpFile, content, 0600); err != nil {
        return fmt.Errorf("write temp file: %w", err)
    }
    defer os.Remove(tmpFile)
    if err := SudoRun("cp", tmpFile, path); err != nil {
        return err
    }
    return SudoRun("chmod", fmt.Sprintf("%o", perm), path)
}

// Download fetches a URL to a local path using wget or curl.
func Download(url, dest string) error {
    if _, err := exec.LookPath("wget"); err == nil {
        return Run("wget", "-q", "-O", dest, url)
    }
    return Run("curl", "-sL", "-o", dest, url)
}