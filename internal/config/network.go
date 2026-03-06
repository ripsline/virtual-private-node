// internal/config/network.go

package config

import "fmt"

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
	}
}

func NetworkConfigFromName(name string) *NetworkConfig {
	switch name {
	case "mainnet":
		return Mainnet()
	case "testnet4":
		return Testnet4()
	default:
		return Mainnet()
	}
}

func ValidateNetwork(name string) error {
	switch name {
	case "mainnet", "testnet4":
		return nil
	default:
		return fmt.Errorf("unknown network %q: must be mainnet or testnet4", name)
	}
}
