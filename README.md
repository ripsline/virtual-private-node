## Virtual Private Node

A single binary that installs and configures a Bitcoin node on a
fresh Debian 12 VPS. Supports Bitcoin Core with optional Lightning
Network (LND), all routed through Tor.

After installation, you manage your node with standard tools:
`bitcoin-cli`, `lncli`, `systemctl`, and `journalctl`. No wrappers,
no abstractions.

### What it installs

- **Tor** — all connections routed through Tor SOCKS proxy
- **Bitcoin Core 29.2** — pruned, configurable storage (10/25/50 GB)
- **LND 0.20.0-beta** (optional) — Lightning Network with Tor hidden services

All services run as a dedicated `bitcoin` system user under systemd.

### Requirements

- Fresh Debian 12+ VPS
- Root access
- 2 vCPU, 4 GB RAM, 90+ GB SSD
- Go 1.25+

## Quick Start

### 1. Install Dependencies
```bash
sudo apt update
sudo apt install git
```

### 2. Install Go
```bash
cd /tmp
wget https://go.dev/dl/go1.25.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.6.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile
go version
```

### 3. Clone and Build

```bash
cd ~
git clone https://github.com/ripsline/virtual-private-node.git
cd virtual-private-node
go mod tidy
go build -o rlvpn ./cmd/
```

This creates a `ripsline` user, downloads the `rlvpn` binary, and
prints SSH instructions. Open a new terminal, SSH in as `ripsline`,
and the interactive installer starts automatically.

### What the installer asks

| Question | Options |
|---|---|
| Network | Mainnet or Testnet4 |
| Components | Bitcoin Core only, or Bitcoin Core + LND |
| Prune size | 10 GB, 25 GB, or 50 GB |
| LND P2P mode | Tor only or Hybrid (Tor + clearnet) |
| SSH port | 22 or custom |
| LND wallet | Password + seed (via `lncli create`) |
| Auto-unlock | Store password for unattended restarts |

### Post-install

After installation, every SSH login as `ripsline` shows a welcome
message with your node status, useful commands, and Tor onion
addresses.

You manage your node with the standard tools:
```bash
#Bitcoin Core

bitcoin-cli -datadir=/var/lib/bitcoin -conf=/etc/bitcoin/bitcoin.conf getblockchaininfo
```
```bash
#LND (testnet4)

lncli --lnddir=/var/lib/lnd --network=testnet4 getinfo
lncli --lnddir=/var/lib/lnd --network=testnet4 walletbalance
lncli --lnddir=/var/lib/lnd --network=testnet4 newaddress p2wkh
```
```bash
#LND (mainnet)

lncli --lnddir=/var/lib/lnd getinfo
```
```bash
#Services

sudo systemctl status bitcoind
sudo systemctl status lnd
sudo journalctl -u lnd -f
```

### Architecture
User SSH → ripsline@VPS
│
└── rlvpn (first login: installer, subsequent: welcome message)

Services (systemd, run as bitcoin user):
tor.service → SOCKS proxy (9050), control port (9051)
bitcoind.service → pruned node, Tor-routed
lnd.service → Lightning, Tor hidden services for gRPC/REST

### Directory layout (FHS-compliant)

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