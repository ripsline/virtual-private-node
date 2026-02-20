package config

import "testing"

func TestMainnetConfig(t *testing.T) {
    net := Mainnet()

    tests := []struct {
        field string
        got   interface{}
        want  interface{}
    }{
        {"Name", net.Name, "mainnet"},
        {"BitcoinFlag", net.BitcoinFlag, ""},
        {"LNDBitcoinFlag", net.LNDBitcoinFlag, "bitcoin.mainnet=true"},
        {"RPCPort", net.RPCPort, 8332},
        {"P2PPort", net.P2PPort, 8333},
        {"ZMQBlockPort", net.ZMQBlockPort, 28332},
        {"ZMQTxPort", net.ZMQTxPort, 28333},
        {"LNCLINetwork", net.LNCLINetwork, "mainnet"},
        {"CookiePath", net.CookiePath, ".cookie"},
    }
    for _, tt := range tests {
        if tt.got != tt.want {
            t.Errorf("%s: got %v, want %v", tt.field, tt.got, tt.want)
        }
    }
}

func TestTestnet4Config(t *testing.T) {
    net := Testnet4()

    tests := []struct {
        field string
        got   interface{}
        want  interface{}
    }{
        {"Name", net.Name, "testnet4"},
        {"BitcoinFlag", net.BitcoinFlag, "testnet4=1"},
        {"LNDBitcoinFlag", net.LNDBitcoinFlag, "bitcoin.testnet4=true"},
        {"RPCPort", net.RPCPort, 48332},
        {"P2PPort", net.P2PPort, 48333},
        {"ZMQBlockPort", net.ZMQBlockPort, 28334},
        {"ZMQTxPort", net.ZMQTxPort, 28335},
        {"LNCLINetwork", net.LNCLINetwork, "testnet4"},
        {"CookiePath", net.CookiePath, "testnet4/.cookie"},
    }
    for _, tt := range tests {
        if tt.got != tt.want {
            t.Errorf("%s: got %v, want %v", tt.field, tt.got, tt.want)
        }
    }
}

func TestNetworkConfigFromNameMainnet(t *testing.T) {
    net := NetworkConfigFromName("mainnet")
    if net.Name != "mainnet" {
        t.Errorf("got %q, want mainnet", net.Name)
    }
}

func TestNetworkConfigFromNameTestnet4(t *testing.T) {
    net := NetworkConfigFromName("testnet4")
    if net.Name != "testnet4" {
        t.Errorf("got %q, want testnet4", net.Name)
    }
}

func TestNetworkConfigFromNameUnknown(t *testing.T) {
    net := NetworkConfigFromName("bogus")
    if net.Name != "testnet4" {
        t.Errorf("got %q, want testnet4 (default fallback)", net.Name)
    }
}

func TestNetworkConfigFromNameEmpty(t *testing.T) {
    net := NetworkConfigFromName("")
    if net.Name != "testnet4" {
        t.Errorf("got %q, want testnet4 (default fallback)", net.Name)
    }
}