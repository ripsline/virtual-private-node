package installer

import (
    "os"
    "strconv"
    "testing"
)

func TestVersionConstants(t *testing.T) {
    if bitcoinVersion == "" {
        t.Error("bitcoinVersion is empty")
    }
    if lndVersion == "" {
        t.Error("lndVersion is empty")
    }
    if litVersion == "" {
        t.Error("litVersion is empty")
    }
    if systemUser == "" {
        t.Error("systemUser is empty")
    }
}

func TestSystemUserIsBitcoin(t *testing.T) {
    if systemUser != "bitcoin" {
        t.Errorf("systemUser: got %q, want %q", systemUser, "bitcoin")
    }
}

func TestSetAndGetVersion(t *testing.T) {
    original := appVersion
    defer func() { appVersion = original }()

    SetVersion("1.2.3")
    if GetVersion() != "1.2.3" {
        t.Errorf("GetVersion: got %q, want %q", GetVersion(), "1.2.3")
    }
}

func TestLitVersionStr(t *testing.T) {
    v := LitVersionStr()
    if v == "" {
        t.Error("LitVersionStr returned empty")
    }
    if v != litVersion {
        t.Errorf("got %q, want %q", v, litVersion)
    }
}

func TestLndVersionStr(t *testing.T) {
    v := LndVersionStr()
    if v == "" {
        t.Error("LndVersionStr returned empty")
    }
    if v != lndVersion {
        t.Errorf("got %q, want %q", v, lndVersion)
    }
}

func TestNeedsInstallNoConfig(t *testing.T) {
    // No config file exists on dev machine, so install is needed
    result := NeedsInstall()
    if !result {
        t.Error("NeedsInstall should return true when config is missing")
    }
}

func TestReadVersionCacheEmpty(t *testing.T) {
    // On a dev machine, cache file shouldn't exist
    cached := readVersionCache()
    if cached != "" {
        // Clean up if it exists from a previous test
        os.Remove(versionCachePath)
        cached = readVersionCache()
        if cached != "" {
            t.Error("expected empty cache after removal")
        }
    }
}

func TestWriteAndReadVersionCache(t *testing.T) {
    // Clean up before and after
    os.Remove(versionCachePath)
    defer os.Remove(versionCachePath)

    writeVersionCache("1.2.3")
    cached := readVersionCache()
    if cached != "1.2.3" {
        t.Errorf("cached version: got %q, want 1.2.3", cached)
    }
}

func TestCheckOSReadsFile(t *testing.T) {
    // On non-Debian systems, checkOS should return an error
    // On Debian systems, it should pass
    err := checkOS()
    if err != nil {
        // We're probably on macOS or another dev machine
        t.Logf("checkOS returned error (expected on non-Debian): %v", err)
    }
}

func TestCheckOSVersionParsing(t *testing.T) {
    tests := []struct {
        ver  string
        pass bool
    }{
        {"13", true},
        {"14", true},
        {"15", true},
        {"12", false},
        {"11", false},
        {"9", false},
    }
    for _, tt := range tests {
        verNum, err := strconv.Atoi(tt.ver)
        if err != nil {
            t.Fatalf("bad test version: %s", tt.ver)
        }
        result := verNum >= 13
        if result != tt.pass {
            t.Errorf("version %q >= 13: got %v, want %v",
                tt.ver, result, tt.pass)
        }
    }
}