package installer

import (
    "strings"
    "testing"

    "github.com/ripsline/virtual-private-node/internal/config"
)

func TestBitcoinConfigMainnet(t *testing.T) {
    cfg := config.Default()
    content := BuildBitcoinConfig(cfg)

    required := []string{
        "server=1",
        "prune=25000",
        "proxy=127.0.0.1:9050",
        "rpcport=8332",
        "zmqpubrawblock=tcp://127.0.0.1:28332",
        "zmqpubrawtx=tcp://127.0.0.1:28333",
        "listen=1",
        "listenonion=1",
        "dbcache=512",
        "maxmempool=300",
        "bind=127.0.0.1",
        "rpcbind=127.0.0.1",
        "rpcallowip=127.0.0.1",
    }
    for _, req := range required {
        if !strings.Contains(content, req) {
            t.Errorf("mainnet config missing %q", req)
        }
    }

    if strings.Contains(content, "testnet4=1") {
        t.Error("mainnet config should not contain testnet4 flag")
    }
}

func TestBitcoinConfigTestnet4(t *testing.T) {
    cfg := &config.AppConfig{
        Network:   "testnet4",
        PruneSize: 25,
        P2PMode:   "tor",
    }
    content := BuildBitcoinConfig(cfg)

    required := []string{
        "testnet4=1",
        "prune=25000",
        "rpcport=48332",
        "zmqpubrawblock=tcp://127.0.0.1:28334",
        "zmqpubrawtx=tcp://127.0.0.1:28335",
        "[testnet4]",
    }
    for _, req := range required {
        if !strings.Contains(content, req) {
            t.Errorf("testnet4 config missing %q", req)
        }
    }
}

func TestBitcoinConfigPruneSizes(t *testing.T) {
    tests := []struct {
        pruneGB int
        wantMB  string
    }{
        {25, "prune=25000"},
        {50, "prune=50000"},
        {75, "prune=75000"},
        {100, "prune=100000"},
    }
    for _, tt := range tests {
        cfg := &config.AppConfig{
            Network:   "mainnet",
            PruneSize: tt.pruneGB,
        }
        content := BuildBitcoinConfig(cfg)
        if !strings.Contains(content, tt.wantMB) {
            t.Errorf("prune %d GB: expected %q in config",
                tt.pruneGB, tt.wantMB)
        }
    }
}

func TestBitcoinConfigAlwaysHasProxy(t *testing.T) {
    cfg := config.Default()
    content := BuildBitcoinConfig(cfg)
    if !strings.Contains(content, "proxy=127.0.0.1:9050") {
        t.Error("bitcoin config must always have Tor proxy")
    }
}

func TestBitcoinConfigAlwaysHasServer(t *testing.T) {
    cfg := config.Default()
    content := BuildBitcoinConfig(cfg)
    if !strings.Contains(content, "server=1") {
        t.Error("bitcoin config must always have server=1")
    }
}

func TestBitcoinConfigHeader(t *testing.T) {
    cfg := config.Default()
    content := BuildBitcoinConfig(cfg)
    if !strings.Contains(content, "Virtual Private Node") {
        t.Error("bitcoin config should have VPN header comment")
    }
}