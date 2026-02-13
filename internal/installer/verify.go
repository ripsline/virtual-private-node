package installer

import (
    "fmt"
    "os"
    "os/exec"
    "strings"
)

// ── Trusted signing keys ─────────────────────────────────
//
// PRIMARY key fingerprints (from gpg --list-keys after import).
// GPG signs with subkeys but we verify the primary key exists.

var bitcoinCoreSigners = []struct {
    name        string
    fingerprint string
    keyURL      string
}{
    {
        name:        "fanquake",
        fingerprint: "E777299FC265DD04793070EB944D35F9AC3DB76A",
        keyURL:      "https://raw.githubusercontent.com/bitcoin-core/guix.sigs/main/builder-keys/fanquake.gpg",
    },
    {
        name:        "guggero",
        fingerprint: "FDE04B7075113BFB085020B57BBD8D4D95DB9F03",
        keyURL:      "https://raw.githubusercontent.com/bitcoin-core/guix.sigs/main/builder-keys/guggero.gpg",
    },
    {
        name:        "hebasto",
        fingerprint: "CBE89ED88EE8525FD8D79F1EDB56ADFD8B5EF498",
        keyURL:      "https://raw.githubusercontent.com/bitcoin-core/guix.sigs/main/builder-keys/hebasto.gpg",
    },
    {
        name:        "theStack",
        fingerprint: "9343A22960A50972CC1EFD7DB3B5CB8DB648B27F",
        keyURL:      "https://raw.githubusercontent.com/bitcoin-core/guix.sigs/main/builder-keys/theStack.gpg",
    },
    {
        name:        "willcl-ark",
        fingerprint: "A0083660F235A27000CD3C81CE6EC49945C17EA6",
        keyURL:      "https://raw.githubusercontent.com/bitcoin-core/guix.sigs/main/builder-keys/willcl-ark.gpg",
    },
}

var lndSigner = struct {
    name        string
    fingerprint string
    keyURL      string
}{
    name:        "roasbeef",
    fingerprint: "296212681AADF05656A2CDEE90525F7DEEE0AD86",
    keyURL:      "https://raw.githubusercontent.com/lightningnetwork/lnd/master/scripts/keys/roasbeef.asc",
}

var litSigner = struct {
    name        string
    fingerprint string
    keyID       string
}{
    name:        "ViktorT-11",
    fingerprint: "C20A78516A0944900EBFCA29961CC8259AE675D4",
    keyID:       "C20A78516A0944900EBFCA29961CC8259AE675D4",
}

// ── GPG setup ────────────────────────────────────────────

func ensureGPG() error {
    if _, err := exec.LookPath("gpg"); err == nil {
        return nil
    }
    cmd := exec.Command("apt-get", "install", "-y", "-qq", "gnupg")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("install gpg: %s: %s", err, output)
    }
    return nil
}

// ── Key import ───────────────────────────────────────────

// importBitcoinCoreKeys downloads and imports trusted builder keys.
// Continues on individual failures — we only need 2/5 later.
func importBitcoinCoreKeys() error {
    imported := 0
    for _, signer := range bitcoinCoreSigners {
        keyFile := fmt.Sprintf("/tmp/btc-key-%s.gpg", signer.name)
        if err := download(signer.keyURL, keyFile); err != nil {
            continue
        }
        cmd := exec.Command("gpg", "--batch", "--import", keyFile)
        cmd.CombinedOutput()
        os.Remove(keyFile)

        if gpgHasFingerprint(signer.fingerprint) {
            imported++
        }
    }
    if imported == 0 {
        return fmt.Errorf("could not import any Bitcoin Core signing keys")
    }
    return nil
}

func importLNDKey() error {
    keyFile := "/tmp/lnd-key-roasbeef.asc"
    if err := download(lndSigner.keyURL, keyFile); err != nil {
        return fmt.Errorf("download LND signing key: %w", err)
    }
    defer os.Remove(keyFile)

    cmd := exec.Command("gpg", "--batch", "--import", keyFile)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("import LND key: %s: %s", err, output)
    }
    if !gpgHasFingerprint(lndSigner.fingerprint) {
        return fmt.Errorf("LND key fingerprint mismatch")
    }
    return nil
}

func importLITKey() error {
    cmd := exec.Command("gpg", "--batch", "--keyserver",
        "hkps://keyserver.ubuntu.com", "--recv-keys", litSigner.keyID)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("import LIT key: %s: %s", err, output)
    }
    if !gpgHasFingerprint(litSigner.fingerprint) {
        return fmt.Errorf("LIT key fingerprint mismatch")
    }
    return nil
}

// ── Signature verification ───────────────────────────────

// verifyBitcoinCoreSigs counts GOODSIG lines in GPG output.
// Since we only import trusted keys, every GOODSIG is from
// a trusted builder. Requires minValid valid signatures.
func verifyBitcoinCoreSigs(minValid int) error {
    sumsFile := "/tmp/SHA256SUMS"
    sigFile := "/tmp/SHA256SUMS.asc"

    if _, err := os.Stat(sumsFile); err != nil {
        return fmt.Errorf("SHA256SUMS not found")
    }
    if _, err := os.Stat(sigFile); err != nil {
        return fmt.Errorf("SHA256SUMS.asc not found")
    }

    cmd := exec.Command("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, sumsFile)
    output, _ := cmd.CombinedOutput()

    validCount := strings.Count(string(output), "GOODSIG")

    if validCount < minValid {
        return fmt.Errorf(
            "insufficient valid signatures: got %d, need %d",
            validCount, minValid)
    }
    return nil
}

// verifyLNDSig verifies the LND manifest GPG signature.
func verifyLNDSig(version string) error {
    manifestFile := "/tmp/manifest.txt"
    sigFile := fmt.Sprintf("/tmp/manifest-roasbeef-v%s.sig", version)

    if _, err := os.Stat(manifestFile); err != nil {
        return fmt.Errorf("LND manifest not found at %s", manifestFile)
    }

    sigURL := fmt.Sprintf(
        "https://github.com/lightningnetwork/lnd/releases/download/v%s/manifest-roasbeef-v%s.sig",
        version, version)
    if err := download(sigURL, sigFile); err != nil {
        return fmt.Errorf("download LND signature: %w", err)
    }
    defer os.Remove(sigFile)

    cmd := exec.Command("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, manifestFile)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("LND signature verification failed: %s", output)
    }
    if !strings.Contains(string(output), "GOODSIG") {
        return fmt.Errorf("LND signature invalid")
    }
    return nil
}

// verifyLITSig verifies the LIT manifest GPG signature.
func verifyLITSig(version string) error {
    manifestFile := "/tmp/lit-manifest.txt"
    sigFile := fmt.Sprintf("/tmp/manifest-ViktorT-11-v%s.sig", version)

    if _, err := os.Stat(manifestFile); err != nil {
        return fmt.Errorf("LIT manifest not found at %s", manifestFile)
    }

    sigURL := fmt.Sprintf(
        "https://github.com/lightninglabs/lightning-terminal/releases/download/v%s/manifest-ViktorT-11-v%s.sig",
        version, version)
    if err := download(sigURL, sigFile); err != nil {
        return fmt.Errorf("download LIT signature: %w", err)
    }
    defer os.Remove(sigFile)

    cmd := exec.Command("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, manifestFile)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("LIT signature verification failed: %s", output)
    }
    if !strings.Contains(string(output), "GOODSIG") {
        return fmt.Errorf("LIT signature invalid")
    }
    return nil
}

// ── Helpers ──────────────────────────────────────────────

func gpgHasFingerprint(fingerprint string) bool {
    cmd := exec.Command("gpg", "--batch", "--list-keys",
        "--with-colons", fingerprint)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return false
    }
    return strings.Contains(string(output), fingerprint)
}

func downloadBitcoinSigFile(version string) error {
    url := fmt.Sprintf(
        "https://bitcoincore.org/bin/bitcoin-core-%s/SHA256SUMS.asc", version)
    return download(url, "/tmp/SHA256SUMS.asc")
}