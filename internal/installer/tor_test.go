package installer

import (
    "strings"
    "testing"

    "github.com/ripsline/virtual-private-node/internal/config"
)

func TestTorConfigBitcoinOnly(t *testing.T) {
    cfg := config.Default()
    content := BuildTorConfig(cfg)

    required := []string{
        "SOCKSPort 9050",
        "bitcoin-p2p",
        "HiddenServicePort 8333",
    }
    for _, req := range required {
        if !strings.Contains(content, req) {
            t.Errorf("missing %q in bitcoin-only torrc", req)
        }
    }

    forbidden := []string{
        "ControlPort",
        "bitcoin-rpc",
        "lnd-rest",
        "lnd-grpc",
        "lnd-lit",
        "syncthing",
    }
    for _, f := range forbidden {
        if strings.Contains(content, f) {
            t.Errorf("bitcoin-only torrc should not contain %q", f)
        }
    }
}

func TestTorConfigWithLND(t *testing.T) {
    cfg := config.Default()
    cfg.LNDInstalled = true
    content := BuildTorConfig(cfg)

    required := []string{
        "SOCKSPort 9050",
        "ControlPort 9051",
        "CookieAuthentication 1",
        "CookieAuthFileGroupReadable 1",
        "bitcoin-p2p",
        "lnd-grpc",
        "lnd-rest",
        "HiddenServicePort 10009",
        "HiddenServicePort 8080",
    }
    for _, req := range required {
        if !strings.Contains(content, req) {
            t.Errorf("missing %q in LND torrc", req)
        }
    }
}

func TestTorConfigWithLIT(t *testing.T) {
    cfg := config.Default()
    cfg.LNDInstalled = true
    cfg.LITInstalled = true
    content := BuildTorConfig(cfg)

    if !strings.Contains(content, "lnd-lit") {
        t.Error("missing lnd-lit hidden service")
    }
    if !strings.Contains(content, "HiddenServicePort 8443") {
        t.Error("missing LIT port 8443")
    }
}

func TestTorConfigWithSyncthing(t *testing.T) {
    cfg := config.Default()
    cfg.SyncthingInstalled = true
    content := BuildTorConfig(cfg)

    required := []string{
        "syncthing",
        "HiddenServicePort 8384",
        "syncthing-sync",
        "HiddenServicePort 22000",
    }
    for _, req := range required {
        if !strings.Contains(content, req) {
            t.Errorf("missing %q in Syncthing torrc", req)
        }
    }
}

func TestTorConfigNoSyncthingWithoutInstall(t *testing.T) {
    cfg := config.Default()
    content := BuildTorConfig(cfg)

    if strings.Contains(content, "syncthing") {
        t.Error("should not have syncthing without install")
    }
}

func TestTorConfigNoLITWithoutInstall(t *testing.T) {
    cfg := config.Default()
    cfg.LNDInstalled = true
    content := BuildTorConfig(cfg)

    if strings.Contains(content, "lnd-lit") {
        t.Error("should not have lnd-lit without LIT install")
    }
}

func TestTorConfigFullStack(t *testing.T) {
    cfg := &config.AppConfig{
        Network:            "mainnet",
        LNDInstalled:       true,
        LITInstalled:       true,
        SyncthingInstalled: true,
    }
    content := BuildTorConfig(cfg)

    required := []string{
        "SOCKSPort 9050",
        "ControlPort 9051",
        "bitcoin-p2p",
        "lnd-grpc",
        "lnd-rest",
        "lnd-lit",
        "syncthing",
        "syncthing-sync",
    }
    for _, req := range required {
        if !strings.Contains(content, req) {
            t.Errorf("full stack torrc missing %q", req)
        }
    }
}

func TestTorConfigMainnetPorts(t *testing.T) {
    cfg := config.Default()
    content := BuildTorConfig(cfg)

    if !strings.Contains(content, "HiddenServicePort 8333") {
        t.Error("mainnet torrc should use port 8333 for P2P")
    }
    if strings.Contains(content, "HiddenServicePort 8332") {
        t.Error("mainnet torrc should not have RPC hidden service")
    }
}

func TestTorConfigTestnet4Ports(t *testing.T) {
    cfg := &config.AppConfig{Network: "testnet4"}
    content := BuildTorConfig(cfg)

    if !strings.Contains(content, "HiddenServicePort 48333") {
        t.Error("testnet4 torrc should use port 48333 for P2P")
    }
    if strings.Contains(content, "HiddenServicePort 48332") {
        t.Error("testnet4 torrc should not have RPC hidden service")
    }
}

func TestTorConfigNoControlPortWithoutLND(t *testing.T) {
    cfg := config.Default()
    content := BuildTorConfig(cfg)

    if strings.Contains(content, "ControlPort") {
        t.Error("should not have ControlPort without LND")
    }
}