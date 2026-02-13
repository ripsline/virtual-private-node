package config

import (
    "encoding/json"
    "os"
)

const configDir = "/etc/rlvpn"
const configPath = "/etc/rlvpn/config.json"

type AppConfig struct {
    Network            string `json:"network"`
    Components         string `json:"components"`
    PruneSize          int    `json:"prune_size"`
    P2PMode            string `json:"p2p_mode"`
    AutoUnlock         bool   `json:"auto_unlock"`
    SSHPort            int    `json:"ssh_port"`
    LITInstalled       bool   `json:"lit_installed"`
    LITPassword        string `json:"lit_password,omitempty"`
    SyncthingInstalled bool   `json:"syncthing_installed"`
    SyncthingPassword  string `json:"syncthing_password,omitempty"`
}

func Default() *AppConfig {
    return &AppConfig{
        Network:    "testnet4",
        Components: "bitcoin+lnd",
        PruneSize:  25,
        P2PMode:    "tor",
        SSHPort:    22,
    }
}

func Load() (*AppConfig, error) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, err
    }
    var cfg AppConfig
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

func Save(cfg *AppConfig) error {
    if err := os.MkdirAll(configDir, 0755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(configPath, data, 0600)
}

func (c *AppConfig) HasLND() bool {
    return c.Components == "bitcoin+lnd"
}

func (c *AppConfig) IsMainnet() bool {
    return c.Network == "mainnet"
}

func (c *AppConfig) WalletExists() bool {
    network := c.Network
    if c.IsMainnet() {
        network = "mainnet"
    }
    path := "/var/lib/lnd/data/chain/bitcoin/" + network + "/wallet.db"
    _, err := os.Stat(path)
    return err == nil
}