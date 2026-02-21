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

type getInfoResponse struct {
    IdentityPubkey    string `json:"identity_pubkey"`
    NumActiveChannels int    `json:"num_active_channels"`
    SyncedToChain     bool   `json:"synced_to_chain"`
    SyncedToGraph     bool   `json:"synced_to_graph"`
}

type walletBalanceResponse struct {
    TotalBalance string `json:"total_balance"`
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

    var resp getInfoResponse
    if err := json.Unmarshal([]byte(output), &resp); err != nil {
        return nil, err
    }

    return &NodeInfo{
        Pubkey:      resp.IdentityPubkey,
        Channels:    resp.NumActiveChannels,
        SyncedChain: resp.SyncedToChain,
        SyncedGraph: resp.SyncedToGraph,
    }, nil
}

func GetBalance(network string) (*WalletBalance, error) {
    args := lncliArgs(network, "walletbalance")
    output, err := system.SudoRunContext(5*time.Second,
        args[0], args[1:]...)
    if err != nil {
        return nil, err
    }

    var resp walletBalanceResponse
    if err := json.Unmarshal([]byte(output), &resp); err != nil {
        return nil, err
    }

    return &WalletBalance{
        TotalBalance: resp.TotalBalance,
    }, nil
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