## Virtual Private Node

A one-command installer for a private Bitcoin node on Debian —
Bitcoin Core and Tor, configured and running in minutes.

After installation, manage your node with `bitcoin-cli` and
`systemctl`. No wrappers, no abstractions.

### What it installs

- **Tor** — all connections routed through Tor
- **Bitcoin Core 29.3** — pruned (25 GB default), Tor-routed

### Additional software (from Add-ons tab)

- **LND 0.20.0-beta** — Lightning with Tor hidden services
- **Lightning Terminal v0.16.0-alpha** — browser UI for channel management
- **Syncthing** — file sync with automatic LND channel backup

### Requirements

- Fresh Debian 13+
- 2 vCPU, 4 GB RAM, 90+ GB SSD
- [Mynymbox VPS with exact specs](https://client.mynymbox.io/store/custom/custom-vps-2-4-90-nl?aff=8)

### Quick Start

SSH into your VPS as root and run:

```bash
apt update && apt install -y git curl
```
```bash
curl -sL https://raw.githubusercontent.com/ripsline/virtual-private-node/main/virtual-private-node.sh | bash
```

This creates a`ripsline` user, downloads the`rlvpn` binary, and
disables root SSH. Follow the on-screen instructions to SSH in as
`ripsline` — Bitcoin Core begins installing and syncing automatically.

For testnet4 (developers only):

```bash
curl -sL https://raw.githubusercontent.com/ripsline/virtual-private-node/main/virtual-private-node.sh | bash -s -- --testnet4
```

### Dashboard

Every SSH login as`ripsline` opens a dashboard with five tabs:

- **Dashboard** — Services, System, Bitcoin, and installed add-on cards
- **Pairing** — Zeus and Sparrow wallet connection with QR codes
- **Logs** — journal logs per service
- **Add-ons** — install LND, Lightning Terminal, and Syncthing
- **Settings** — prune size (25 GB minimum)

Press`q` to drop to a shell:

```bash
bitcoin-cli getblockchaininfo
bitcoin-cli getpeerinfo

# After installing LND from Add-ons:
lncli getinfo
lncli walletbalance

# Services
sudo systemctl status bitcoind
sudo systemctl status lnd
sudo journalctl -u lnd -n 50 --no-pager
```

### Build from Source

```bash
apt update && apt install -y git wget sudo curl

cd /tmp
wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile

cd ~
git clone https://github.com/ripsline/virtual-private-node.git
cd virtual-private-node
go mod tidy
go build -o rlvpn ./cmd/
sudo install -m 755 ./rlvpn /usr/local/bin/rlvpn
curl -sL https://raw.githubusercontent.com/ripsline/virtual-private-node/main/virtual-private-node.sh | bash
```

The bootstrap script detects that`rlvpn` is already installed and
skips the download.

### Software Verification

All software is verified with GPG signatures and SHA256 checksums:

- **Bitcoin Core** — 5 trusted builder keys from
  [bitcoin-core/guix.sigs](https://github.com/bitcoin-core/guix.sigs).
  Requires 2 of 5 valid signatures.
- **LND** — Roasbeef's signing key verified against known fingerprint.
- **Lightning Terminal** — ViktorT-11's signing key from Ubuntu keyserver.

Verification failure is a hard stop.

After installation, review the verification log:

```bash
sudo cat /var/log/rlvpn-verification.log
```

For manual binary verification before installation, see
[Release Verification](docs/release-verification.md).

### Connecting Wallets

#### Zeus (Lightning — LND REST over Tor)

1. Install LND from Add-ons tab, create wallet
2. Open Pairing tab → Zeus card
3. In Zeus: Advanced Set-Up → LND (REST)
4. Enter server address, REST port (8080), and macaroon from Pairing tab
5. Or scan QR code from Pairing tab

#### Sparrow (On-chain — Bitcoin Core RPC over Tor)

1. Open Pairing tab → Sparrow card
2. In Sparrow: Settings → Server → Bitcoin Core
3. Enter URL, port, and cookie credentials from Pairing tab
4. Test Connection

Note: cookie password changes when Bitcoin Core restarts.

### Security

- All connections through Tor (SOCKS5 port 9050)
- IPv6 disabled to prevent Tor bypass
- Stream isolation (separate circuit per connection)
- UFW firewall: SSH only (+ 9735 for hybrid P2P)
- Fail2ban: SSH brute-force protection
- Root SSH disabled after bootstrap
- Services run as dedicated bitcoin system user
- Cookie authentication for Bitcoin Core RPC
- GPG signature verification for all software
- Unattended security upgrades with auto-reboot
- LND channel backup auto-synced via Syncthing

### Architecture

```
User SSH → ripsline@VPS → rlvpn dashboard
                             press q → shell with bitcoin-cli, lncli

Services (systemd, run as bitcoin user):
  tor.service → SOCKS proxy, hidden services
  bitcoind.service   → pruned node, Tor-routed
  lnd.service → Lightning (add-on)
  litd.service       → Lightning Terminal (add-on)
  syncthing.service  → file sync (add-on)
```

### Directory Layout

| Path | Contents |
| --- | --- |
| /etc/bitcoin/bitcoin.conf | Bitcoin Core configuration |
| /etc/lnd/lnd.conf | LND configuration |
| /etc/lit/lit.conf | Lightning Terminal configuration |
| /etc/syncthing/ | Syncthing configuration |
| /etc/rlvpn/config.json | Install state and credentials |
| /var/lib/bitcoin/ | Blockchain data |
| /var/lib/lnd/ | LND data and wallet |
| /var/lib/lit/ | Lightning Terminal data |
| /var/lib/syncthing/lnd-backup/ | Auto-synced channel.backup |

## License

Copyright (C) 2026 ripsline

This project is free software licensed under the
[GNU Affero General Public License v3.0](LICENSE). You are free to
use, modify, and distribute it under the same terms. If you run a
modified version as a network service, you must make the source
available to its users.