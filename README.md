## Virtual Private Node

A one-command installer for private Bitcoin and Lightning nodes on
Debian — Bitcoin Core, LND, and Tor, configured and running in minutes.

After installation, manage your node with `bitcoin-cli`, `lncli`,
and `systemctl`. No wrappers, no abstractions.

### What it installs

- **Tor** — all connections routed through Tor
- **Bitcoin Core 29.2** — pruned, configurable (10/25/50 GB)
- **LND 0.20.0-beta** (optional) — Lightning with Tor hidden services

### Requirements

- Fresh Debian 12+ Server
- Root access
- 2 vCPU, 4 GB RAM, 90+ GB SSD

### Quick Start

SSH into your VPS as root and run:

~~~
curl -sL https://raw.githubusercontent.com/ripsline/virtual-private-node/main/virtual-private-node.sh | bash
~~~

This creates a `ripsline` user and downloads the `rlvpn` binary.
Follow the on-screen instructions to SSH in as `ripsline` — the
node installer starts automatically.

### Build from Source

#### 1. Install Dependencies

~~~bash
sudo apt update
sudo apt install -y git wget
~~~

#### 2. Install Go

~~~bash
cd /tmp
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile
go version
~~~

#### 3. Clone and Build

~~~bash
cd ~
git clone https://github.com/ripsline/virtual-private-node.git
cd virtual-private-node
go mod tidy
go build -o rlvpn ./cmd/
~~~

#### 4. Manual Bootstrap

Run these commands as root to set up the `ripsline` user and
install the binary:

~~~bash
# Create ripsline user
adduser --disabled-password --gecos "Virtual Private Node" ripsline
PASSWORD=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 25)
echo "ripsline:$PASSWORD" | chpasswd
echo "Password for ripsline: $PASSWORD"

# Passwordless sudo
echo "ripsline ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/ripsline
chmod 440 /etc/sudoers.d/ripsline

# Install the binary
cp ./rlvpn /usr/local/bin/rlvpn
chmod 755 /usr/local/bin/rlvpn

# Auto-launch on ripsline login
cat > /home/ripsline/.bash_profile << 'EOF'
if [ -n "$SSH_CONNECTION" ] && [ -t 0 ]; then
    sudo /usr/local/bin/rlvpn
fi
EOF
chown ripsline:ripsline /home/ripsline/.bash_profile
~~~

Save the printed password, then open a new terminal and connect:

~~~bash
ssh ripsline@YOUR_SERVER_IP
~~~

The installer starts automatically.

### What the installer asks

| Question | Options |
|---|---|
| Network | Mainnet or Testnet4 |
| Components | Bitcoin Core only, or Bitcoin Core + LND |
| Prune size | 10 GB, 25 GB, or 50 GB |
| LND P2P mode | Tor only or Hybrid (Tor + clearnet) |
| SSH port | 22 or custom |
| LND wallet | Created via `lncli create` |
| Auto-unlock | Store password for unattended restarts |

### Post-install

Every SSH login as `ripsline` opens a dashboard showing node
health, service status, and system resources. Use the tabs
to view logs or get wallet pairing details for Zeus and Sparrow.

Press `q` to drop to a shell. Manage your node directly:

~~~bash
# Bitcoin Core
bitcoin-cli getblockchaininfo
bitcoin-cli getpeerinfo

# LND
lncli getinfo
lncli walletbalance
lncli newaddress p2wkh
lncli addinvoice --amt=1000 --memo="test"
lncli listchannels

# Services
sudo systemctl status bitcoind
sudo systemctl status lnd
sudo systemctl status tor@default
sudo journalctl -u lnd -f
~~~

### Connecting wallets

#### Zeus (Lightning — LND REST over Tor)

1. Download & Verify Zeus Mobile
2. Advanced Set-Up
3. Create or connect a wallet
4. Wallet interface dropdown → LND(REST)
5. Paste the Server address, REST Port, and Macaroon

#### Sparrow (On-chain — Bitcoin Core RPC over Tor)

1. Open the **Pairing** tab for your RPC URL, port, and credentials
2. In Sparrow Wallet: Sparrow → Settings → Server → Bitcoin Core
3. Enter the URL, port, username, and password
4. Test Connection

### Architecture

~~~
User SSH → ripsline@VPS → rlvpn (dashboard)
                             press q → shell with bitcoin-cli, lncli

Services (systemd, run as bitcoin user):
  tor.service      → SOCKS proxy (9050), control port (9051)
  bitcoind.service → pruned node, Tor-routed
  lnd.service      → Lightning, Tor hidden services
~~~

### Directory layout

| Path | Contents |
|---|---|
| `/etc/bitcoin/bitcoin.conf` | Bitcoin Core configuration |
| `/etc/lnd/lnd.conf` | LND configuration |
| `/etc/rlvpn/config.json` | Installer choices (network, components) |
| `/var/lib/bitcoin/` | Blockchain data |
| `/var/lib/lnd/` | LND data, wallet, macaroons |
| `/usr/local/bin/rlvpn` | Installer binary |
| `/usr/local/bin/bitcoind` | Bitcoin Core daemon |
| `/usr/local/bin/lnd` | LND daemon |
| `/usr/local/bin/lncli` | LND CLI |

### Security

- All outbound connections routed through Tor (SOCKS5 on port 9050)
- IPv6 disabled at kernel level to prevent Tor bypass
- Stream isolation enabled (separate Tor circuit per connection)
- UFW firewall: only SSH open (+ 9735 for hybrid P2P mode)
- Bitcoin Core RPC: cookie authentication, localhost only
- LND gRPC and REST: Tor hidden services only, never clearnet
- Root SSH disabled after bootstrap
- Passwordless sudo for `ripsline` user
- Services run as dedicated `bitcoin` system user

### Tor hidden services

The installer creates static Tor hidden services for:

| Service | Port | Use case |
|---|---|---|
| Bitcoin RPC | 8332 / 48332 | Sparrow Wallet connection |
| Bitcoin P2P | 8333 / 48333 | Static peer address |
| LND gRPC | 10009 | Wallet apps over Tor |
| LND REST | 8080 | Zeus, wallet apps over Tor |

Onion addresses are displayed after installation and on every login.

### Plugins

This node is a base layer. Additional software can be installed
on top:

- **[electrum-go](https://github.com/ripsline/electrum-go)** — forward-
  indexing electrum server (beta available)

- **[lnvoice](https://github.com/ripsline/lnvoice)** — voice
  control for LND (beta coming soon)

Plugins use `lncli` and `bitcoin-cli` directly. No proprietary
APIs or custom interfaces.

## License

MIT License - see [LICENSE](LICENSE) file for details.