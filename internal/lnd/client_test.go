package lnd

import (
    "encoding/json"
    "testing"
)

func TestGetInfoParsing(t *testing.T) {
    raw := `{
        "version": "0.20.0-beta",
        "identity_pubkey": "02abc123def456789abc123def456789abc123def456789abc123def456789abcd",
        "alias": "mynode",
        "num_active_channels": 5,
        "num_peers": 10,
        "synced_to_chain": true,
        "synced_to_graph": true,
        "block_height": 850000
    }`

    var resp getInfoResponse
    if err := json.Unmarshal([]byte(raw), &resp); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    if resp.IdentityPubkey != "02abc123def456789abc123def456789abc123def456789abc123def456789abcd" {
        t.Errorf("IdentityPubkey: got %q", resp.IdentityPubkey)
    }
    if resp.NumActiveChannels != 5 {
        t.Errorf("NumActiveChannels: got %d, want 5", resp.NumActiveChannels)
    }
    if !resp.SyncedToChain {
        t.Error("SyncedToChain: expected true")
    }
    if !resp.SyncedToGraph {
        t.Error("SyncedToGraph: expected true")
    }
}

func TestGetInfoNotSynced(t *testing.T) {
    raw := `{
        "identity_pubkey": "02abc123",
        "num_active_channels": 0,
        "synced_to_chain": false,
        "synced_to_graph": false
    }`

    var resp getInfoResponse
    if err := json.Unmarshal([]byte(raw), &resp); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    if resp.SyncedToChain {
        t.Error("SyncedToChain: expected false")
    }
    if resp.NumActiveChannels != 0 {
        t.Errorf("NumActiveChannels: got %d, want 0", resp.NumActiveChannels)
    }
}

func TestGetInfoExtraFields(t *testing.T) {
    raw := `{
        "version": "0.20.0-beta",
        "commit_hash": "abc123",
        "identity_pubkey": "02abc123",
        "alias": "test",
        "color": "#000000",
        "num_pending_channels": 1,
        "num_active_channels": 3,
        "num_inactive_channels": 0,
        "num_peers": 8,
        "block_height": 850000,
        "block_hash": "0000abc",
        "best_header_timestamp": 1700000000,
        "synced_to_chain": true,
        "synced_to_graph": true,
        "chains": [{"chain": "bitcoin", "network": "mainnet"}]
    }`

    var resp getInfoResponse
    if err := json.Unmarshal([]byte(raw), &resp); err != nil {
        t.Fatalf("unmarshal should ignore extra fields: %v", err)
    }

    if resp.NumActiveChannels != 3 {
        t.Errorf("NumActiveChannels: got %d, want 3", resp.NumActiveChannels)
    }
}

func TestWalletBalanceParsing(t *testing.T) {
    raw := `{
        "total_balance": "1000000",
        "confirmed_balance": "900000",
        "unconfirmed_balance": "100000",
        "locked_balance": "0"
    }`

    var resp walletBalanceResponse
    if err := json.Unmarshal([]byte(raw), &resp); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    if resp.TotalBalance != "1000000" {
        t.Errorf("TotalBalance: got %q, want 1000000", resp.TotalBalance)
    }
}

func TestWalletBalanceZero(t *testing.T) {
    raw := `{
        "total_balance": "0",
        "confirmed_balance": "0",
        "unconfirmed_balance": "0"
    }`

    var resp walletBalanceResponse
    if err := json.Unmarshal([]byte(raw), &resp); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    if resp.TotalBalance != "0" {
        t.Errorf("TotalBalance: got %q, want 0", resp.TotalBalance)
    }
}

func TestLncliArgsMainnet(t *testing.T) {
    args := lncliArgs("mainnet", "getinfo")
    expected := []string{"-u", "bitcoin", "lncli",
        "--lnddir=/var/lib/lnd",
        "--network=mainnet",
        "getinfo"}

    if len(args) != len(expected) {
        t.Fatalf("args length: got %d, want %d", len(args), len(expected))
    }
    for i, arg := range args {
        if arg != expected[i] {
            t.Errorf("arg[%d]: got %q, want %q", i, arg, expected[i])
        }
    }
}

func TestLncliArgsTestnet4(t *testing.T) {
    args := lncliArgs("testnet4", "walletbalance")
    found := false
    for _, arg := range args {
        if arg == "--network=testnet4" {
            found = true
        }
    }
    if !found {
        t.Error("testnet4 network flag not found in args")
    }

    lastArg := args[len(args)-1]
    if lastArg != "walletbalance" {
        t.Errorf("last arg: got %q, want walletbalance", lastArg)
    }
}