package config

import (
    "encoding/json"
    "os"
)

// Defaults — used by production code
const (
    DefaultDir  = "/etc/rlvpn"
    DefaultPath = "/etc/rlvpn/config.json"
)

type AppConfig struct {
    InstallComplete    bool   `json:"install_complete"`
    InstallVersion     string `json:"install_version,omitempty"`
    Network            string `json:"network"`
    Components         string `json:"components"`
    PruneSize          int    `json:"prune_size"`
    P2PMode            string `json:"p2p_mode"`
    AutoUnlock         bool   `json:"auto_unlock"`
    LNDInstalled       bool   `json:"lnd_installed"`
    WalletCreated      bool   `json:"wallet_created"`
    LITInstalled       bool   `json:"lit_installed"`
    LITPassword        string `json:"lit_password,omitempty"`
    SyncthingInstalled bool   `json:"syncthing_installed"`
    SyncthingPassword  string `json:"syncthing_password,omitempty"`
}

// Store handles reading/writing config to disk.
type Store struct {
    Dir  string
    Path string
}

func DefaultStore() *Store {
    return &Store{Dir: DefaultDir, Path: DefaultPath}
}

func Default() *AppConfig {
    return &AppConfig{
        Network:    "mainnet",
        Components: "bitcoin",
        PruneSize:  25,
        P2PMode:    "tor",
    }
}

func (s *Store) Load() (*AppConfig, error) {
    data, err := os.ReadFile(s.Path)
    if err != nil {
        return nil, err
    }
    var cfg AppConfig
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

func (s *Store) Save(cfg *AppConfig) error {
    if err := os.MkdirAll(s.Dir, 0750); err != nil {
        return err
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(s.Path, data, 0600)
}

// Convenience functions that use the default store.
// These keep existing call sites working without changes.
func Load() (*AppConfig, error) {
    return DefaultStore().Load()
}

func Save(cfg *AppConfig) error {
    return DefaultStore().Save(cfg)
}

func (c *AppConfig) HasLND() bool {
    return c.LNDInstalled
}

func (c *AppConfig) IsMainnet() bool {
    return c.Network == "mainnet"
}

func (c *AppConfig) WalletExists() bool {
    return c.WalletCreated
}

func (c *AppConfig) NetworkConfig() *NetworkConfig {
    return NetworkConfigFromName(c.Network)
}