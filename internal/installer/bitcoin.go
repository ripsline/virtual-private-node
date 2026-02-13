package installer

import (
    "fmt"
    "os"
    "os/exec"
)

func downloadBitcoin(version string) error {
    filename := fmt.Sprintf("bitcoin-%s-x86_64-linux-gnu.tar.gz", version)
    url := fmt.Sprintf("https://bitcoincore.org/bin/bitcoin-core-%s/%s", version, filename)
    shaURL := fmt.Sprintf("https://bitcoincore.org/bin/bitcoin-core-%s/SHA256SUMS", version)
    if err := download(url, "/tmp/"+filename); err != nil {
        return err
    }
    return download(shaURL, "/tmp/SHA256SUMS")
}

func verifyBitcoin(version string) error {
    cmd := exec.Command("sha256sum", "--ignore-missing", "--check", "SHA256SUMS")
    cmd.Dir = "/tmp"
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("checksum failed: %s: %s", err, output)
    }
    return nil
}

func extractAndInstallBitcoin(version string) error {
    filename := fmt.Sprintf("bitcoin-%s-x86_64-linux-gnu.tar.gz", version)
    cmd := exec.Command("tar", "-xzf", "/tmp/"+filename, "-C", "/tmp")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("extract: %s: %s", err, output)
    }
    extractDir := fmt.Sprintf("/tmp/bitcoin-%s/bin", version)
    entries, err := os.ReadDir(extractDir)
    if err != nil {
        return fmt.Errorf("read dir: %w", err)
    }
    for _, entry := range entries {
        src := fmt.Sprintf("%s/%s", extractDir, entry.Name())
        cmd = exec.Command("install", "-m", "0755", "-o", "root", "-g", "root",
            src, "/usr/local/bin/")
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("install %s: %s: %s", entry.Name(), err, output)
        }
    }
    os.Remove("/tmp/" + filename)
    os.Remove("/tmp/SHA256SUMS")
    os.Remove("/tmp/SHA256SUMS.asc")
    os.RemoveAll(fmt.Sprintf("/tmp/bitcoin-%s", version))
    return nil
}

func writeBitcoinConfig(cfg *installConfig) error {
    pruneMB := cfg.pruneSize * 1000
    content := fmt.Sprintf(`# Virtual Private Node — Bitcoin Core
server=1
%s
prune=%d
dbcache=512
maxmempool=300
disablewallet=1
proxy=127.0.0.1:9050
listen=1
listenonion=1
`, cfg.network.Name, cfg.network.BitcoinFlag, pruneMB)

    if cfg.network.Name == "testnet4" {
        content = fmt.Sprintf(`# Virtual Private Node — Bitcoin Core
server=1
%s
prune=%d
dbcache=512
maxmempool=300
disablewallet=1
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
`, cfg.network.BitcoinFlag, pruneMB,
            cfg.network.RPCPort, cfg.network.ZMQBlockPort, cfg.network.ZMQTxPort)
    } else {
        content = fmt.Sprintf(`# Virtual Private Node — Bitcoin Core
server=1
prune=%d
dbcache=512
maxmempool=300
disablewallet=1
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
            cfg.network.RPCPort, cfg.network.ZMQBlockPort, cfg.network.ZMQTxPort)
    }

    if err := os.WriteFile("/etc/bitcoin/bitcoin.conf", []byte(content), 0640); err != nil {
        return err
    }
    cmd := exec.Command("chown", "root:"+systemUser, "/etc/bitcoin/bitcoin.conf")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("%s: %s", err, output)
    }
    return nil
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
    for _, args := range [][]string{
        {"systemctl", "daemon-reload"},
        {"systemctl", "enable", "bitcoind"},
        {"systemctl", "start", "bitcoind"},
    } {
        cmd := exec.Command(args[0], args[1:]...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("%v: %s: %s", args, err, output)
        }
    }
    return nil
}

func download(url, dest string) error {
    var cmd *exec.Cmd
    if _, err := exec.LookPath("wget"); err == nil {
        cmd = exec.Command("wget", "-q", "-O", dest, url)
    } else {
        cmd = exec.Command("curl", "-sL", "-o", dest, url)
    }
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("download %s: %s: %s", url, err, output)
    }
    return nil
}