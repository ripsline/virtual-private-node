## Virtual Private Node

A one-command installer for private Bitcoin and Lightning nodes on
Debian — Bitcoin Core, LND, and Tor, configured and running in minutes.

After installation, manage your node with `bitcoin-cli`, `lncli`,
and `systemctl`. No wrappers, no abstractions.

### What it installs

- **Tor** — all connections routed through Tor
- **Bitcoin Core 29.3** — pruned, configurable (10/25/50 GB)
- **LND 0.20.0-beta** (optional) — Lightning with Tor hidden services
- **Unattended security upgrades** — auto-patching with reboot at 4 AM UTC

### Additional software (from the dashboard)

- **Lightning Terminal v0.16.0-alpha** — browser UI for channel management
- **Syncthing** — file sync with automatic LND channel backup

### Requirements

- Fresh Debian 12+ VPS
- 2 vCPU, 4 GB RAM, 90+ GB SSD

- Mynymbox affiliate link with exact specs:
https://client.mynymbox.io/store/custom/custom-vps-2-4-90-nl?aff=8

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
apt update
apt install -y git wget sudo
~~~

#### 2. Install Go

~~~bash
cd /tmp
wget https://go.dev/dl/go1.25.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.6.linux-amd64.tar.gz
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

# Auto-launch on ripsline login (source .bashrc first)
cat > /home/ripsline/.bash_profile << 'EOF'
[ -f ~/.bashrc ] && source ~/.bashrc
if [ -n "$SSH_CONNECTION" ] && [ -t 0 ]; then
    sudo /usr/local/bin/rlvpn
fi
EOF
chown ripsline:ripsline /home/ripsline/.bash_profile
~~~

Save the printed password, then open a new terminal:

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

### Post-install Dashboard

Every SSH login as `ripsline` opens a dashboard with four tabs:

- **Dashboard** — four product cards: Services (start/stop/restart),
  System (disk, RAM, update), Bitcoin (sync status), Lightning
  (wallet creation, node details)
- **Pairing** — Zeus and Sparrow wallet connection setup with QR code
- **Logs** — select a service to view journal logs
- **Software** — install Lightning Terminal and Syncthing

Press `q` to drop to a shell:

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
sudo journalctl -u lnd -n 50 --no-pager
~~~

### Software Verification

All software is verified with GPG signatures and SHA256 checksums:

- **Bitcoin Core** — 5 trusted builder keys imported from
  [bitcoin-core/guix.sigs](https://github.com/bitcoin-core/guix.sigs).
  Requires 2 out of 5 valid signatures. Hard abort if fewer than 2.
- **LND** — Roasbeef's signing key verified against known fingerprint.
- **Lightning Terminal** — ViktorT-11's signing key from Ubuntu keyserver.

Verification failure is a hard stop — the installer will not proceed
with unverified software.

### Connecting Wallets

#### Zeus (Lightning — LND REST over Tor)

1. coming soon...
2. coming soon...
3. coming soon...

#### Sparrow (On-chain — Bitcoin Core RPC over Tor)

1. coming soon...
2. coming soon...
3. coming soon...

Note: the cookie password changes when Bitcoin Core restarts.

### Additional Software

#### Lightning Terminal (LIT)

Browser-based interface for channel management. Installed from the
Software tab. Accessed via Tor Browser at the onion address shown
in the Lightning detail screen. Self-signed certificate warning is
expected — the connection is encrypted by Tor.

#### Syncthing

File synchronization between your node and local devices.
Automatically backs up LND channel state (channel.backup) to a
sync folder. Install from the Software tab, then pair your local
Syncthing instance through the web UI (accessed via Tor Browser).

### Architecture

~~~
User SSH → ripsline@VPS → rlvpn dashboard
                             press q → shell with bitcoin-cli, lncli

Services (systemd, run as bitcoin user):
  tor.service              → SOCKS proxy (9050), control port (9051)
  bitcoind.service         → pruned node, Tor-routed
  lnd.service              → Lightning, Tor hidden services
  litd.service             → Lightning Terminal web UI
  syncthing.service        → file sync with channel backup
  lnd-backup-watch.path    → watches channel.backup for changes
~~~

### Directory Layout

| Path | Contents |
|---|---|
| /etc/bitcoin/bitcoin.conf | Bitcoin Core configuration |
| /etc/lnd/lnd.conf | LND configuration |
| /etc/lit/lit.conf | Lightning Terminal configuration |
| /etc/syncthing/ | Syncthing configuration |
| /etc/rlvpn/config.json | Install choices and credentials |
| /var/lib/bitcoin/ | Blockchain data |
| /var/lib/lnd/ | LND data and wallet |
| /var/lib/lit/ | Lightning Terminal data |
| /var/lib/syncthing/ | Syncthing data and backup folder |
| /var/lib/syncthing/lnd-backup/ | Auto-synced channel.backup |

### Security

- All connections through Tor (SOCKS5 port 9050)
- IPv6 disabled to prevent Tor bypass
- Stream isolation (separate circuit per connection)
- UFW firewall: SSH only (+ 9735 for hybrid P2P)
- Fail2ban: SSH brute-force protection (5 attempts, 10-minute ban)
- Root SSH disabled after bootstrap
- Passwordless sudo for ripsline
- Services run as dedicated bitcoin system user
- Cookie authentication for Bitcoin Core RPC
- GPG signature verification for all software
- Unattended security upgrades with auto-reboot
- LND channel backup auto-synced via Syncthing

### Plugins

This is a base layer. Additional software can be built on top:

- **[electrum-go](https://github.com/ripsline/electrum-go)** — forward-
  indexing electrum style server (beta software - run in testnet and provide feedback)

Plugins use `lncli` and `bitcoin-cli` directly.

## License

MIT License - see [LICENSE](LICENSE) file for details.