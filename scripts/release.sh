#!/bin/bash
set -eo pipefail

# ═══════════════════════════════════════════════════════════
# Virtual Private Node — Release Build Script
#
# Builds the binary, creates tarball, generates checksums,
# and signs with GPG.
#
# Usage:
#   ./scripts/release.sh 0.1.0
# ═══════════════════════════════════════════════════════════

VERSION="$1"

if [ -z "$VERSION" ]; then
    echo "Usage: ./scripts/release.sh <version>"
    echo "Example: ./scripts/release.sh 0.1.0"
    exit 1
fi

BINARY="rlvpn"
OUTDIR="release"

echo ""
echo "  Building Virtual Private Node v${VERSION}"
echo ""

# ── Clean ───────────────────────────────────────────────────

rm -rf "$OUTDIR"
mkdir -p "$OUTDIR"

# ── Build ───────────────────────────────────────────────────

echo "  Building linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o "${OUTDIR}/${BINARY}" ./cmd/
echo "  ✓ Binary built"

# ── Create tarball ──────────────────────────────────────────

TARBALL="${BINARY}-${VERSION}-amd64.tar.gz"
cd "$OUTDIR"
tar -czf "$TARBALL" "$BINARY"
rm "$BINARY"
echo "  ✓ Created ${TARBALL}"

# ── Generate checksums ──────────────────────────────────────

sha256sum "$TARBALL" > SHA256SUMS
echo "  ✓ Generated SHA256SUMS"
cat SHA256SUMS

# ── Sign checksums ──────────────────────────────────────────

echo ""
echo "  Signing SHA256SUMS..."
gpg --armor --detach-sign SHA256SUMS
echo "  ✓ Created SHA256SUMS.asc"

# ── Verify ──────────────────────────────────────────────────

echo ""
echo "  Verifying signature..."
gpg --verify SHA256SUMS.asc SHA256SUMS
echo "  ✓ Signature valid"

# ── Summary ─────────────────────────────────────────────────

echo ""
echo "  ═══════════════════════════════════════════════════"
echo ""
echo "  Release files in ./${OUTDIR}/:"
echo ""
ls -lh "$TARBALL" SHA256SUMS SHA256SUMS.asc
echo ""
echo "  Next steps:"
echo "    1. git tag -s v${VERSION} -m 'v${VERSION}'"
echo "    2. git push origin v${VERSION}"
echo "    3. Upload these 3 files to the GitHub release"
echo ""
echo "  ═══════════════════════════════════════════════════"
echo ""