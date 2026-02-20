package lnd

import (
    "encoding/json"
    "time"

    "github.com/ripsline/virtual-private-node/internal/system"
)

const (
    lndDir  = "/var/lib/lnd"
    svcUser = "bitcoin"
)

type NodeInfo struct {
    Pubkey      string
    Channels    int
    SyncedChain bool
    SyncedGraph bool
}

type WalletBalance struct {
    TotalBalance string
}

func lncliArgs(network string, subcmd ...string) []string {
    args := []string{"-u", svcUser, "lncli",
        "--lnddir=" + lndDir,
        "--network=" + network}
    return append(args, subcmd...)
}

func GetInfo(network string) (*NodeInfo, error) {
    args := lncliArgs(network, "getinfo")
    output, err := system.SudoRunContext(5*time.Second,
        args[0], args[1:]...)
    if err != nil {
        return nil, err
    }
    var raw map[string]interface{}
    if err := json.Unmarshal([]byte(output), &raw); err != nil {
        return nil, err
    }
    info := &NodeInfo{}
    if v, ok := raw["identity_pubkey"].(string); ok {
        info.Pubkey = v
    }
    if v, ok := raw["num_active_channels"].(float64); ok {
        info.Channels = int(v)
    }
    if v, ok := raw["synced_to_chain"].(bool); ok {
        info.SyncedChain = v
    }
    if v, ok := raw["synced_to_graph"].(bool); ok {
        info.SyncedGraph = v
    }
    return info, nil
}

func GetBalance(network string) (*WalletBalance, error) {
    args := lncliArgs(network, "walletbalance")
    output, err := system.SudoRunContext(5*time.Second,
        args[0], args[1:]...)
    if err != nil {
        return nil, err
    }
    var raw map[string]interface{}
    if err := json.Unmarshal([]byte(output), &raw); err != nil {
        return nil, err
    }
    bal := &WalletBalance{}
    if v, ok := raw["total_balance"].(string); ok {
        bal.TotalBalance = v
    }
    return bal, nil
}

func GetChannelCount(network string) (int, error) {
    info, err := GetInfo(network)
    if err != nil {
        return 0, err
    }
    return info.Channels, nil
}

func GetPubkey(network string) (string, error) {
    info, err := GetInfo(network)
    if err != nil {
        return "", err
    }
    return info.Pubkey, nil
}