package installer

import (
    "fmt"
    "os"
    "strings"

    "github.com/ripsline/virtual-private-node/internal/system"
)

// ── Trusted signing keys ─────────────────────────────────

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
    if _, err := system.RunOutput("which", "gpg"); err == nil {
        return nil
    }
    return system.Run("apt-get", "install", "-y", "-qq", "gnupg")
}

// ── Key import ───────────────────────────────────────────

func importBitcoinCoreKeys() error {
    imported := 0
    for _, signer := range bitcoinCoreSigners {
        keyFile := fmt.Sprintf("/tmp/btc-key-%s.gpg", signer.name)
        if err := system.Download(signer.keyURL, keyFile); err != nil {
            continue
        }
        system.RunSilent("gpg", "--batch", "--import", keyFile)
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
    if err := system.Download(lndSigner.keyURL, keyFile); err != nil {
        return fmt.Errorf("download LND signing key: %w", err)
    }
    defer os.Remove(keyFile)
    if err := system.Run("gpg", "--batch", "--import", keyFile); err != nil {
        return fmt.Errorf("import LND key: %w", err)
    }
    if !gpgHasFingerprint(lndSigner.fingerprint) {
        return fmt.Errorf("LND key fingerprint mismatch")
    }
    return nil
}

func importLITKey() error {
    if err := system.Run("gpg", "--batch", "--keyserver",
        "hkps://keyserver.ubuntu.com", "--recv-keys", litSigner.keyID); err != nil {
        return fmt.Errorf("import LIT key: %w", err)
    }
    if !gpgHasFingerprint(litSigner.fingerprint) {
        return fmt.Errorf("LIT key fingerprint mismatch")
    }
    return nil
}

// ── Signature verification ───────────────────────────────

func verifyBitcoinCoreSigs(minValid int) error {
    sumsFile := "/tmp/SHA256SUMS"
    sigFile := "/tmp/SHA256SUMS.asc"

    if _, err := os.Stat(sumsFile); err != nil {
        return fmt.Errorf("SHA256SUMS not found")
    }
    if _, err := os.Stat(sigFile); err != nil {
        return fmt.Errorf("SHA256SUMS.asc not found")
    }

    output, _ := system.RunOutput("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, sumsFile)

    validCount := strings.Count(output, "GOODSIG")
    if validCount < minValid {
        return fmt.Errorf(
            "insufficient valid signatures: got %d, need %d",
            validCount, minValid)
    }
    return nil
}

func verifyLNDSig(version string) error {
    manifestFile := "/tmp/manifest.txt"
    sigFile := fmt.Sprintf("/tmp/manifest-roasbeef-v%s.sig", version)

    if _, err := os.Stat(manifestFile); err != nil {
        return fmt.Errorf("LND manifest not found at %s", manifestFile)
    }

    sigURL := fmt.Sprintf(
        "https://github.com/lightningnetwork/lnd/releases/download/v%s/manifest-roasbeef-v%s.sig",
        version, version)
    if err := system.Download(sigURL, sigFile); err != nil {
        return fmt.Errorf("download LND signature: %w", err)
    }
    defer os.Remove(sigFile)

    output, err := system.RunOutput("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, manifestFile)
    if err != nil {
        return fmt.Errorf("LND signature verification failed: %s", output)
    }
    if !strings.Contains(output, "GOODSIG") {
        return fmt.Errorf("LND signature invalid")
    }
    return nil
}

func verifyLITSig(version string) error {
    manifestFile := "/tmp/lit-manifest.txt"
    sigFile := fmt.Sprintf("/tmp/manifest-ViktorT-11-v%s.sig", version)

    if _, err := os.Stat(manifestFile); err != nil {
        return fmt.Errorf("LIT manifest not found at %s", manifestFile)
    }

    sigURL := fmt.Sprintf(
        "https://github.com/lightninglabs/lightning-terminal/releases/download/v%s/manifest-ViktorT-11-v%s.sig",
        version, version)
    if err := system.Download(sigURL, sigFile); err != nil {
        return fmt.Errorf("download LIT signature: %w", err)
    }
    defer os.Remove(sigFile)

    output, err := system.RunOutput("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, manifestFile)
    if err != nil {
        return fmt.Errorf("LIT signature verification failed: %s", output)
    }
    if !strings.Contains(output, "GOODSIG") {
        return fmt.Errorf("LIT signature invalid")
    }
    return nil
}

// ── Helpers ──────────────────────────────────────────────

func gpgHasFingerprint(fingerprint string) bool {
    output, err := system.RunOutput("gpg", "--batch", "--list-keys",
        "--with-colons", fingerprint)
    if err != nil {
        return false
    }
    return strings.Contains(output, fingerprint)
}

func downloadBitcoinSigFile(version string) error {
    url := fmt.Sprintf(
        "https://bitcoincore.org/bin/bitcoin-core-%s/SHA256SUMS.asc", version)
    return system.Download(url, "/tmp/SHA256SUMS.asc")
}