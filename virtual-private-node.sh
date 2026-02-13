#!/bin/bash
set -eo pipefail

# ═══════════════════════════════════════════════════════════
# Virtual Private Node — Bootstrap Script
#
# This script runs as root on a fresh Debian 12+ VPS.
# It creates the ripsline user, downloads the rlvpn binary,
# configures auto-launch, and disables root SSH.
#
# Usage:
#   curl -sL ripsline.com/virtual-private-node.sh | sudo bash
# ═══════════════════════════════════════════════════════════

VERSION="0.1.0"
BINARY_NAME="rlvpn"
ADMIN_USER="ripsline"

# ── Preflight checks ────────────────────────────────────────

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: Run as root."
    echo "  curl -sL ripsline.com/virtual-private-node.sh | sudo bash"
    exit 1
fi

if ! grep -q "ID=debian" /etc/os-release 2>/dev/null; then
    echo "ERROR: This installer requires Debian 12+."
    exit 1
fi

echo ""
echo "  ╔══════════════════════════════════════════╗"
echo "  ║  Virtual Private Node — Bootstrap        ║"
echo "  ╚══════════════════════════════════════════╝"
echo ""

# ── Create admin user ───────────────────────────────────────

if id "$ADMIN_USER" &>/dev/null; then
    echo "  User $ADMIN_USER already exists, skipping."
    PASSWORD="(unchanged)"
else
    PASSWORD=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 25)
    adduser --disabled-password --gecos "Virtual Private Node" "$ADMIN_USER"
    echo "$ADMIN_USER:$PASSWORD" | chpasswd
    echo "  ✓ Created user $ADMIN_USER"
fi

# ── Passwordless sudo ───────────────────────────────────────

echo "$ADMIN_USER ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/$ADMIN_USER
chmod 440 /etc/sudoers.d/$ADMIN_USER
echo "  ✓ Configured passwordless sudo"

# ── Copy SSH keys if root has them ──────────────────────────

if [ -f /root/.ssh/authorized_keys ]; then
    mkdir -p /home/$ADMIN_USER/.ssh
    cp /root/.ssh/authorized_keys /home/$ADMIN_USER/.ssh/
    chown -R $ADMIN_USER:$ADMIN_USER /home/$ADMIN_USER/.ssh
    chmod 700 /home/$ADMIN_USER/.ssh
    chmod 600 /home/$ADMIN_USER/.ssh/authorized_keys
    echo "  ✓ Copied SSH keys to $ADMIN_USER"
fi

# ── Download rlvpn binary ───────────────────────────────────

ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    *)
        echo "ERROR: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

URL="https://github.com/ripsline/virtual-private-node/releases/download/v${VERSION}/${BINARY_NAME}-linux-${ARCH}"

echo "  Downloading rlvpn v${VERSION}..."
if command -v wget &>/dev/null; then
    wget -q -O /usr/local/bin/$BINARY_NAME "$URL"
elif command -v curl &>/dev/null; then
    curl -sL -o /usr/local/bin/$BINARY_NAME "$URL"
else
    echo "ERROR: Neither wget nor curl found."
    exit 1
fi

# Verify download succeeded
if [ ! -s /usr/local/bin/$BINARY_NAME ]; then
    echo "ERROR: Download failed. Check the URL and try again."
    rm -f /usr/local/bin/$BINARY_NAME
    exit 1
fi

chmod 755 /usr/local/bin/$BINARY_NAME
echo "  ✓ Installed rlvpn to /usr/local/bin/"

# ── Auto-launch on ripsline login ───────────────────────────

cat > /home/$ADMIN_USER/.bash_profile << 'BASHEOF'
# Source .bashrc for environment variables and shell functions
[ -f ~/.bashrc ] && source ~/.bashrc

# Virtual Private Node — auto-launch
if [ -n "$SSH_CONNECTION" ] && [ -t 0 ]; then
    sudo /usr/local/bin/rlvpn
fi
BASHEOF
chown $ADMIN_USER:$ADMIN_USER /home/$ADMIN_USER/.bash_profile
echo "  ✓ Configured auto-launch"

# ── Disable root SSH login ──────────────────────────────────

sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
systemctl restart sshd
echo "  ✓ Disabled root SSH login"

# ── Print instructions ──────────────────────────────────────

if command -v curl &>/dev/null; then
    SERVER_IP=$(curl -4 -s --max-time 5 ifconfig.me 2>/dev/null || echo "YOUR_SERVER_IP")
elif command -v wget &>/dev/null; then
    SERVER_IP=$(wget -qO- --timeout=5 ifconfig.me 2>/dev/null || echo "YOUR_SERVER_IP")
else
    SERVER_IP="YOUR_SERVER_IP"
fi

echo ""
echo "  ═══════════════════════════════════════════════════"
echo ""
echo "  Bootstrap complete."
echo ""
echo "  Open a NEW terminal and connect:"
echo ""
echo "    ssh $ADMIN_USER@$SERVER_IP"
echo "    Password: $PASSWORD"
echo ""
echo "  The node installer will start automatically."
echo ""
echo "  ⚠️  Save this password. Root SSH is now disabled."
echo "  ⚠️  Recovery: use your VPS provider's console."
echo ""
echo "  ═══════════════════════════════════════════════════"
echo ""