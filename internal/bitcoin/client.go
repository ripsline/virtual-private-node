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

func GetBlockchainInfo(datadir, conf string) *BlockchainInfo {
    output, err := system.RunContext(5*time.Second,
        "sudo", "-u", "bitcoin", "bitcoin-cli",
        "-datadir="+datadir, "-conf="+conf,
        "getblockchaininfo")
    if err != nil {
        return &BlockchainInfo{Responding: false}
    }

    var raw map[string]interface{}
    if err := json.Unmarshal([]byte(output), &raw); err != nil {
        return &BlockchainInfo{Responding: false}
    }

    info := &BlockchainInfo{Responding: true}
    if v, ok := raw["blocks"].(float64); ok {
        info.Blocks = int(v)
    }
    if v, ok := raw["headers"].(float64); ok {
        info.Headers = int(v)
    }
    if v, ok := raw["verificationprogress"].(float64); ok {
        info.Progress = v
    }
    if v, ok := raw["initialblockdownload"].(bool); ok {
        info.Synced = !v
    }
    return info
}

func FormatProgress(progress float64) string {
    return fmt.Sprintf("%.2f%%", progress*100)
}