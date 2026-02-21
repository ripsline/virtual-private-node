package bitcoin

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/ripsline/virtual-private-node/internal/system"
)

type BlockchainInfo struct {
    Blocks     int
    Headers    int
    Progress   float64
    Synced     bool
    Responding bool
}

type blockchainInfoResponse struct {
    Blocks               int     `json:"blocks"`
    Headers              int     `json:"headers"`
    VerificationProgress float64 `json:"verificationprogress"`
    InitialBlockDownload bool    `json:"initialblockdownload"`
}

func GetBlockchainInfo(datadir, conf string) *BlockchainInfo {
    output, err := system.SudoRunContext(5*time.Second,
        "-u", "bitcoin", "bitcoin-cli",
        "-datadir="+datadir, "-conf="+conf,
        "getblockchaininfo")
    if err != nil {
        return &BlockchainInfo{Responding: false}
    }

    var resp blockchainInfoResponse
    if err := json.Unmarshal([]byte(output), &resp); err != nil {
        return &BlockchainInfo{Responding: false}
    }

    return &BlockchainInfo{
        Blocks:     resp.Blocks,
        Headers:    resp.Headers,
        Progress:   resp.VerificationProgress,
        Synced:     !resp.InitialBlockDownload,
        Responding: true,
    }
}

func FormatProgress(progress float64) string {
    return fmt.Sprintf("%.2f%%", progress*100)
}