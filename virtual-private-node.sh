#!/bin/bash
set -eo pipefail

# ═══════════════════════════════════════════════════════════
# Virtual Private Node — Bootstrap Script
#
# This script runs as root on a fresh Debian 13+.
# It creates the ripsline user, downloads the rlvpn binary,
# configures auto-launch, and disables root SSH.
#
# Usage:
#   curl -sL ripsline.com/virtual-private-node.sh | sudo bash
# ═══════════════════════════════════════════════════════════

VERSION="0.1.1"
BINARY_NAME="rlvpn"
ADMIN_USER="ripsline"

BASE_URL="https://github.com/ripsline/virtual-private-node/releases/download/v${VERSION}"
PUBKEY_URL="https://raw.githubusercontent.com/ripsline/virtual-private-node/main/docs/release.pub.asc"

# ── Preflight checks ────────────────────────────────────────

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: Run as root."
    echo "  curl -sL ripsline.com/virtual-private-node.sh | sudo bash"
    exit 1
fi

if ! grep -q "ID=debian" /etc/os-release 2>/dev/null; then
    echo "ERROR: This installer requires Debian."
    exit 1
fi

DEBIAN_VER=$(grep -oP 'VERSION_ID="\K[0-9]+' /etc/os-release 2>/dev/null || echo "0")
if [ "$DEBIAN_VER" -lt 12 ]; then
    echo "ERROR: Debian 12+ required."
    exit 1
fi

echo ""
echo "  ╔══════════════════════════════════════════╗"
echo "  ║  Virtual Private Node — Bootstrap        ║"
echo "  ╚══════════════════════════════════════════╝"
echo ""

# ── Ensure dependencies ─────────────────────────────────────

apt-get update -qq
apt-get install -y -qq sudo gnupg

# ── Create admin user ───────────────────────────────────────

if id "$ADMIN_USER" &>/dev/null; then
    echo "  User $ADMIN_USER already exists, skipping."
    PASSWORD="(unchanged)"
else
    PASSWORD=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 25 || true)
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

# ── Download helpers ────────────────────────────────────────

download() {
    local url="$1"
    local out="$2"
    if command -v wget &>/dev/null; then
        wget -q -O "$out" "$url"
    elif command -v curl &>/dev/null; then
        curl -sL -o "$out" "$url"
    else
        echo "ERROR: Neither wget nor curl found."
        exit 1
    fi
}

# ── Download rlvpn tarball ──────────────────────────────────

ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    *)
        echo "ERROR: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

TARBALL="${BINARY_NAME}-${VERSION}-${ARCH}.tar.gz"

if command -v "$BINARY_NAME" &>/dev/null; then
    echo "  rlvpn binary already installed, skipping download."
else
    echo "  Downloading ${TARBALL}..."
    download "${BASE_URL}/${TARBALL}" "/tmp/${TARBALL}"

    if [ ! -s "/tmp/${TARBALL}" ]; then
        echo "ERROR: Download failed. Check your connection and try again."
        rm -f "/tmp/${TARBALL}"
        exit 1
    fi

    # ── Verify checksums + GPG signature ────────────────────────

    echo "  Downloading SHA256SUMS + signature..."
    download "${BASE_URL}/SHA256SUMS" "/tmp/SHA256SUMS"
    download "${BASE_URL}/SHA256SUMS.asc" "/tmp/SHA256SUMS.asc"

    echo "  Importing release public key..."
    download "${PUBKEY_URL}" "/tmp/release.pub.asc"
    gpg --batch --import /tmp/release.pub.asc >/dev/null 2>&1

    echo "  Verifying checksum signature..."
    if ! gpg --batch --verify /tmp/SHA256SUMS.asc /tmp/SHA256SUMS 2>/dev/null; then
        echo "ERROR: GPG signature verification failed."
        echo "  The download may be corrupted or tampered with."
        rm -f /tmp/${TARBALL} /tmp/SHA256SUMS /tmp/SHA256SUMS.asc /tmp/release.pub.asc
        exit 1
    fi
    echo "  ✓ Signature verified"

    echo "  Verifying checksum..."
    cd /tmp
    if ! sha256sum -c SHA256SUMS --ignore-missing 2>/dev/null | grep -q "${TARBALL}: OK"; then
        echo "ERROR: Checksum verification failed."
        rm -f /tmp/${TARBALL} /tmp/SHA256SUMS /tmp/SHA256SUMS.asc /tmp/release.pub.asc
        exit 1
    fi
    echo "  ✓ Checksum verified"
    cd - >/dev/null

    # ── Extract + install binary ────────────────────────────────

    tar -xzf "/tmp/${TARBALL}" -C /tmp

    if [ ! -s "/tmp/${BINARY_NAME}" ]; then
        echo "ERROR: Extracted binary not found."
        rm -f /tmp/${TARBALL} /tmp/SHA256SUMS /tmp/SHA256SUMS.asc /tmp/release.pub.asc
        exit 1
    fi

    install -m 755 "/tmp/${BINARY_NAME}" /usr/local/bin/$BINARY_NAME
    echo "  ✓ Installed rlvpn to /usr/local/bin/"

    # ── Cleanup ─────────────────────────────────────────────────

    rm -f /tmp/${TARBALL} /tmp/SHA256SUMS /tmp/SHA256SUMS.asc /tmp/release.pub.asc /tmp/${BINARY_NAME}
fi

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
systemctl restart sshd 2>/dev/null || systemctl restart ssh
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