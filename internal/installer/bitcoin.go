package installer

import (
    "fmt"
    "os"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"
)

func downloadBitcoin(version string) error {
    filename := fmt.Sprintf("bitcoin-%s-x86_64-linux-gnu.tar.gz", version)
    url := fmt.Sprintf("https://bitcoincore.org/bin/bitcoin-core-%s/%s", version, filename)
    shaURL := fmt.Sprintf("https://bitcoincore.org/bin/bitcoin-core-%s/SHA256SUMS", version)
    if err := system.Download(url, "/tmp/"+filename); err != nil {
        return err
    }
    return system.Download(shaURL, "/tmp/SHA256SUMS")
}

func extractAndInstallBitcoin(version string) error {
    filename := fmt.Sprintf("bitcoin-%s-x86_64-linux-gnu.tar.gz", version)
    if err := system.Run("tar", "-xzf", "/tmp/"+filename, "-C", "/tmp"); err != nil {
        return err
    }
    extractDir := fmt.Sprintf("/tmp/bitcoin-%s/bin", version)
    entries, err := os.ReadDir(extractDir)
    if err != nil {
        return fmt.Errorf("read dir: %w", err)
    }
    for _, entry := range entries {
        src := fmt.Sprintf("%s/%s", extractDir, entry.Name())
        if err := system.Run("install", "-m", "0755", "-o", "root", "-g", "root", src, "/usr/local/bin/"); err != nil {
            return err
        }
    }
    os.Remove("/tmp/" + filename)
    os.Remove("/tmp/SHA256SUMS")
    os.Remove("/tmp/SHA256SUMS.asc")
    os.RemoveAll(fmt.Sprintf("/tmp/bitcoin-%s", version))
    return nil
}

func writeBitcoinConfig(cfg *config.AppConfig) error {
    net := cfg.NetworkConfig()
    pruneMB := cfg.PruneSize * 1000

    var content string
    if net.Name == "testnet4" {
        content = fmt.Sprintf(`# Virtual Private Node — Bitcoin Core
server=1
%s
prune=%d
dbcache=512
maxmempool=300
proxy=127.0.0.1:9050
listen=1
listenonion=1

[testnet4]
bind=127.0.0.1
rpcbind=127.0.0.1
rpcport=%d
rpcallowip=127.0.0.1
zmqpubrawblock=tcp://127.0.0.1:%d
zmqpubrawtx=tcp://127.0.0.1:%d
`, net.BitcoinFlag, pruneMB,
            net.RPCPort, net.ZMQBlockPort, net.ZMQTxPort)
    } else {
        content = fmt.Sprintf(`# Virtual Private Node — Bitcoin Core
server=1
prune=%d
dbcache=512
maxmempool=300
proxy=127.0.0.1:9050
listen=1
listenonion=1

bind=127.0.0.1
rpcbind=127.0.0.1
rpcport=%d
rpcallowip=127.0.0.1
zmqpubrawblock=tcp://127.0.0.1:%d
zmqpubrawtx=tcp://127.0.0.1:%d
`, pruneMB,
            net.RPCPort, net.ZMQBlockPort, net.ZMQTxPort)
    }

    if err := os.WriteFile("/etc/bitcoin/bitcoin.conf", []byte(content), 0640); err != nil {
        return err
    }
    return system.Run("chown", "root:"+systemUser, "/etc/bitcoin/bitcoin.conf")
}

func writeBitcoindService(username string) error {
    content := fmt.Sprintf(`[Unit]
Description=Bitcoin Core
After=network-online.target tor.service
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=/usr/local/bin/bitcoind -conf=/etc/bitcoin/bitcoin.conf -datadir=/var/lib/bitcoin
Restart=on-failure
RestartSec=30
TimeoutStopSec=600
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`, username, username)
    return os.WriteFile("/etc/systemd/system/bitcoind.service", []byte(content), 0644)
}

func startBitcoind() error {
    if err := system.Run("systemctl", "daemon-reload"); err != nil {
        return err
    }
    if err := system.Run("systemctl", "enable", "bitcoind"); err != nil {
        return err
    }
    return system.Run("systemctl", "start", "bitcoind")
}