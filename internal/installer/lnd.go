package installer

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/ripsline/virtual-private-node/internal/config"
    "github.com/ripsline/virtual-private-node/internal/system"
)

func downloadLND(version string) error {
    filename := fmt.Sprintf("lnd-linux-amd64-v%s.tar.gz", version)
    url := fmt.Sprintf("https://github.com/lightningnetwork/lnd/releases/download/v%s/%s",
        version, filename)
    manifestURL := fmt.Sprintf("https://github.com/lightningnetwork/lnd/releases/download/v%s/manifest-v%s.txt",
        version, version)
    if err := system.Download(url, "/tmp/"+filename); err != nil {
        return err
    }
    if err := system.Download(manifestURL, "/tmp/manifest.txt"); err != nil {
        return fmt.Errorf("download LND manifest: %w", err)
    }
    return nil
}

func extractAndInstallLND(version string) error {
    filename := fmt.Sprintf("lnd-linux-amd64-v%s.tar.gz", version)
    if err := system.Run("tar", "-xzf", "/tmp/"+filename, "-C", "/tmp"); err != nil {
        return err
    }
    extractDir := fmt.Sprintf("/tmp/lnd-linux-amd64-v%s", version)
    for _, bin := range []string{"lnd", "lncli"} {
        src := fmt.Sprintf("%s/%s", extractDir, bin)
        if err := system.SudoRun("install", "-m", "0755", "-o", "root", "-g", "root", src, "/usr/local/bin/"); err != nil {
            return err
        }
    }
    os.Remove("/tmp/" + filename)
    os.Remove("/tmp/manifest.txt")
    os.RemoveAll(extractDir)
    return nil
}

func createLNDDirs(username string) error {
    dirs := []struct {
        path  string
        owner string
        mode  os.FileMode
    }{
        {"/etc/lnd", "root:" + username, 0750},
        {"/var/lib/lnd", username + ":" + username, 0750},
    }
    for _, d := range dirs {
        if err := system.SudoRun("mkdir", "-p", d.path); err != nil {
            return err
        }
        if err := system.SudoRun("chown", d.owner, d.path); err != nil {
            return err
        }
        if err := system.SudoRun("chmod", fmt.Sprintf("%o", d.mode), d.path); err != nil {
            return err
        }
    }
    return nil
}

func writeLNDConfig(cfg *config.AppConfig, publicIPv4 string) error {
    net := cfg.NetworkConfig()
    restOnion := strings.TrimSpace(readFileOrDefault("/var/lib/tor/lnd-rest/hostname", ""))

    listenLine := "listen=localhost:9735"
    externalLine := ""
    if cfg.P2PMode == "hybrid" && publicIPv4 != "" {
        listenLine = "listen=0.0.0.0:9735"
        externalLine = fmt.Sprintf("externalhosts=%s:9735", publicIPv4)
    }

    tlsExtraDomain := ""
    if restOnion != "" {
        tlsExtraDomain = fmt.Sprintf("tlsextradomain=%s", restOnion)
    }

    cookiePath := fmt.Sprintf("/var/lib/bitcoin/%s", net.CookiePath)

    content := fmt.Sprintf(`# Virtual Private Node — LND
[Application Options]
lnddir=/var/lib/lnd
%s
rpclisten=localhost:10009
restlisten=localhost:8080
debuglevel=info
%s
%s

[Bitcoin]
%s
bitcoin.node=bitcoind

[Bitcoind]
bitcoind.dir=/var/lib/bitcoin
bitcoind.config=/etc/bitcoin/bitcoin.conf
bitcoind.rpccookie=%s
bitcoind.rpchost=127.0.0.1:%d
bitcoind.zmqpubrawblock=tcp://127.0.0.1:%d
bitcoind.zmqpubrawtx=tcp://127.0.0.1:%d

[Tor]
tor.active=true
tor.socks=127.0.0.1:9050
tor.control=127.0.0.1:9051
tor.targetipaddress=127.0.0.1
tor.v3=true
tor.streamisolation=true
`, listenLine, externalLine, tlsExtraDomain,
        net.LNDBitcoinFlag, cookiePath,
        net.RPCPort, net.ZMQBlockPort, net.ZMQTxPort)

    if err := system.SudoWriteFile("/etc/lnd/lnd.conf", []byte(content), 0640); err != nil {
        return err
    }
    return system.SudoRun("chown", "root:"+systemUser, "/etc/lnd/lnd.conf")
}

func writeLNDServiceInitial(username string) error {
    content := fmt.Sprintf(`[Unit]
Description=LND Lightning Network Daemon
After=bitcoind.service tor.service
Wants=bitcoind.service

[Service]
Type=simple
User=%s
Group=%s
ExecStart=/usr/local/bin/lnd --configfile=/etc/lnd/lnd.conf
Restart=on-failure
RestartSec=30
TimeoutStopSec=300
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`, username, username)
    return system.SudoWriteFile("/etc/systemd/system/lnd.service", []byte(content), 0644)
}

func startLND() error {
    if err := system.SudoRun("systemctl", "daemon-reload"); err != nil {
        return err
    }
    if err := system.SudoRun("systemctl", "enable", "lnd"); err != nil {
        return err
    }
    return system.SudoRun("systemctl", "start", "lnd")
}

func setupAutoUnlock(password string) error {
    // Write password to a temp file, then sudo move it
    tmpPw := "/tmp/rlvpn-wallet-pw.tmp"
    if err := os.WriteFile(tmpPw, []byte(password), 0600); err != nil {
        return err
    }
    defer os.Remove(tmpPw)

    passwordFile := "/var/lib/lnd/wallet_password"
    if err := system.SudoRun("cp", tmpPw, passwordFile); err != nil {
        return err
    }
    if err := system.SudoRun("chmod", "0400", passwordFile); err != nil {
        return err
    }
    system.SudoRunSilent("chown", systemUser+":"+systemUser, passwordFile)

    content := fmt.Sprintf(`[Unit]
Description=LND Lightning Network Daemon
After=bitcoind.service tor.service
Wants=bitcoind.service

[Service]
Type=simple
User=%s
Group=%s
ExecStart=/usr/local/bin/lnd --configfile=/etc/lnd/lnd.conf --wallet-unlock-password-file=/var/lib/lnd/wallet_password
Restart=on-failure
RestartSec=30
TimeoutStopSec=300
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`, systemUser, systemUser)

    if err := system.SudoWriteFile("/etc/systemd/system/lnd.service", []byte(content), 0644); err != nil {
        return err
    }
    if err := system.SudoRun("systemctl", "daemon-reload"); err != nil {
        return err
    }
    return system.SudoRun("systemctl", "restart", "lnd")
}

func waitForLND() error {
    for i := 0; i < 60; i++ {
        client := buildLNDClient()
        resp, err := client.Get("https://localhost:8080/v1/state")
        if err == nil {
            resp.Body.Close()
            return nil
        }
        time.Sleep(2 * time.Second)
    }
    return fmt.Errorf("LND did not respond after 120 seconds")
}

func buildLNDClient() *http.Client {
    tlsConfig := &tls.Config{InsecureSkipVerify: true}
    certData, err := os.ReadFile("/var/lib/lnd/tls.cert")
    if err == nil {
        pool := x509.NewCertPool()
        if pool.AppendCertsFromPEM(certData) {
            tlsConfig = &tls.Config{RootCAs: pool}
        }
    }
    return &http.Client{
        Transport: &http.Transport{TLSClientConfig: tlsConfig},
        Timeout:   5 * time.Second,
    }
}