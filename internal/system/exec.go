package system

import (
    "context"
    "fmt"
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

// RunSilent executes a command and discards all output.
func RunSilent(name string, args ...string) error {
    cmd := exec.Command(name, args...)
    cmd.Stdout = nil
    cmd.Stderr = nil
    return cmd.Run()
}

// Download fetches a URL to a local path using wget or curl.
func Download(url, dest string) error {
    if _, err := exec.LookPath("wget"); err == nil {
        return Run("wget", "-q", "-O", dest, url)
    }
    return Run("curl", "-sL", "-o", dest, url)
}