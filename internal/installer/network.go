package installer

// NetworkConfig holds all values that differ between networks.
type NetworkConfig struct {
    Name           string
    BitcoinFlag    string
    LNDBitcoinFlag string
    RPCPort        int
    P2PPort        int
    ZMQBlockPort   int
    ZMQTxPort      int
    LNCLINetwork   string
    CookiePath     string
    DataSubdir     string
}

func Mainnet() *NetworkConfig {
    return &NetworkConfig{
        Name:           "mainnet",
        BitcoinFlag:    "",
        LNDBitcoinFlag: "bitcoin.mainnet=true",
        RPCPort:        8332,
        P2PPort:        8333,
        ZMQBlockPort:   28332,
        ZMQTxPort:      28333,
        LNCLINetwork:   "mainnet",
        CookiePath:     ".cookie",
        DataSubdir:     "",
    }
}

func Testnet4() *NetworkConfig {
    return &NetworkConfig{
        Name:           "testnet4",
        BitcoinFlag:    "testnet4=1",
        LNDBitcoinFlag: "bitcoin.testnet4=true",
        RPCPort:        48332,
        P2PPort:        48333,
        ZMQBlockPort:   28334,
        ZMQTxPort:      28335,
        LNCLINetwork:   "testnet4",
        CookiePath:     "testnet4/.cookie",
        DataSubdir:     "testnet4",
    }
}

func NetworkConfigFromName(name string) *NetworkConfig {
    if name == "mainnet" {
        return Mainnet()
    }
    return Testnet4()
}