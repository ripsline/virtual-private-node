## Release Verification

Verify the bootstrap binary before installation.

### Import the release signing key

```bash
curl -sL https://raw.githubusercontent.com/ripsline/virtual-private-node/main/docs/ripsline-signing-key.asc | gpg --import
```

### Download the release files

```bash
VERSION="0.2.1"
wget -q "https://github.com/ripsline/virtual-private-node/releases/download/v${VERSION}/rlvpn-${VERSION}-amd64.tar.gz"
wget -q "https://github.com/ripsline/virtual-private-node/releases/download/v${VERSION}/SHA256SUMS"
wget -q "https://github.com/ripsline/virtual-private-node/releases/download/v${VERSION}/SHA256SUMS.asc"
```

### Verify the signature

```bash
gpg --verify SHA256SUMS.asc SHA256SUMS
```

### Verify the checksum

```bash
sha256sum --check --ignore-missing SHA256SUMS
```

The bootstrap script performs this verification automatically during
installation. This section is for users who want to verify manually
before running the one-liner.