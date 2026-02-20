package installer

import "testing"

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