package installer

import (
    "fmt"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/ripsline/virtual-private-node/internal/system"
)

const verifyLogPath = "/var/log/rlvpn-verification.log"

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

// ── Verification log ─────────────────────────────────────

func vlog(format string, args ...interface{}) {
    entry := fmt.Sprintf("[%s] %s\n",
        time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
        fmt.Sprintf(format, args...))
    f, err := os.OpenFile(verifyLogPath,
        os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        // Can't open directly, try via sudo append
        system.SudoRun("bash", "-c",
            fmt.Sprintf("echo -n %q >> %s", entry, verifyLogPath))
        return
    }
    defer f.Close()
    f.WriteString(entry)
}

// ── GPG setup ────────────────────────────────────────────

func ensureGPG() error {
    if _, err := exec.LookPath("gpg"); err == nil {
        return nil
    }
    return system.SudoRun("apt-get", "install", "-y", "-qq", "gnupg")
}

// ── Key import ───────────────────────────────────────────

func importBitcoinCoreKeys() error {
    vlog("--- Bitcoin Core key import ---")
    imported := 0
    for _, signer := range bitcoinCoreSigners {
        keyFile := fmt.Sprintf("/tmp/btc-key-%s.gpg", signer.name)
        if err := system.Download(signer.keyURL, keyFile); err != nil {
            vlog("SKIP %s: download failed: %v", signer.name, err)
            continue
        }

        cmd := exec.Command("gpg", "--batch", "--import", keyFile)
        output, _ := cmd.CombinedOutput()
        os.Remove(keyFile)

        if gpgHasFingerprint(signer.fingerprint) {
            imported++
            vlog("OK %s: imported (fingerprint %s)", signer.name, signer.fingerprint)
        } else {
            vlog("SKIP %s: fingerprint not found after import: %s", signer.name, string(output))
        }
    }

    vlog("Bitcoin Core keys imported: %d/%d", imported, len(bitcoinCoreSigners))
    if imported == 0 {
        vlog("FAIL: no Bitcoin Core signing keys imported")
        return fmt.Errorf("could not import any Bitcoin Core signing keys")
    }
    return nil
}

func importLNDKey() error {
    vlog("--- LND key import ---")
    keyFile := "/tmp/lnd-key-roasbeef.asc"
    if err := system.Download(lndSigner.keyURL, keyFile); err != nil {
        vlog("FAIL: download LND signing key: %v", err)
        return fmt.Errorf("download LND signing key: %w", err)
    }
    defer os.Remove(keyFile)

    cmd := exec.Command("gpg", "--batch", "--import", keyFile)
    output, err := cmd.CombinedOutput()
    if err != nil {
        vlog("FAIL: import LND key: %s", string(output))
        return fmt.Errorf("import LND key: %s: %s", err, output)
    }

    if !gpgHasFingerprint(lndSigner.fingerprint) {
        vlog("FAIL: LND key fingerprint mismatch (expected %s)", lndSigner.fingerprint)
        return fmt.Errorf("LND key fingerprint mismatch")
    }

    vlog("OK roasbeef: imported (fingerprint %s)", lndSigner.fingerprint)
    return nil
}

func importLITKey() error {
    vlog("--- LIT key import ---")
    cmd := exec.Command("gpg", "--batch", "--keyserver",
        "hkps://keyserver.ubuntu.com", "--recv-keys", litSigner.keyID)
    output, err := cmd.CombinedOutput()
    if err != nil {
        vlog("FAIL: import LIT key: %s", string(output))
        return fmt.Errorf("import LIT key: %s: %s", err, output)
    }

    if !gpgHasFingerprint(litSigner.fingerprint) {
        vlog("FAIL: LIT key fingerprint mismatch (expected %s)", litSigner.fingerprint)
        return fmt.Errorf("LIT key fingerprint mismatch")
    }

    vlog("OK ViktorT-11: imported (fingerprint %s)", litSigner.fingerprint)
    return nil
}

// ── Signature verification ───────────────────────────────

func verifyBitcoinCoreSigs(minValid int) error {
    vlog("--- Bitcoin Core signature verification ---")
    sumsFile := "/tmp/SHA256SUMS"
    sigFile := "/tmp/SHA256SUMS.asc"

    if _, err := os.Stat(sumsFile); err != nil {
        vlog("FAIL: SHA256SUMS not found")
        return fmt.Errorf("SHA256SUMS not found")
    }
    if _, err := os.Stat(sigFile); err != nil {
        vlog("FAIL: SHA256SUMS.asc not found")
        return fmt.Errorf("SHA256SUMS.asc not found")
    }

    cmd := exec.Command("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, sumsFile)
    output, _ := cmd.CombinedOutput()
    outputStr := string(output)

    validCount := ParseGoodSigCount(outputStr)
    badCount := ParseBadSigCount(outputStr)

    for _, line := range strings.Split(outputStr, "\n") {
        if strings.Contains(line, "GOODSIG") {
            vlog("GOODSIG: %s", strings.TrimSpace(line))
        }
        if strings.Contains(line, "BADSIG") {
            vlog("BADSIG: %s", strings.TrimSpace(line))
        }
    }

    vlog("Bitcoin Core valid signatures: %d/%d required (bad: %d)",
        validCount, minValid, badCount)

    if badCount > 0 {
        vlog("FAIL: %d bad signatures detected", badCount)
        return fmt.Errorf("bad signatures detected: %d", badCount)
    }

    if validCount < minValid {
        vlog("FAIL: insufficient valid signatures: got %d, need %d",
            validCount, minValid)
        return fmt.Errorf(
            "insufficient valid signatures: got %d, need %d",
            validCount, minValid)
    }

    vlog("OK Bitcoin Core: %d valid signatures", validCount)
    return nil
}

func verifyLNDSig(version string) error {
    vlog("--- LND signature verification ---")
    manifestFile := "/tmp/manifest.txt"
    sigFile := fmt.Sprintf("/tmp/manifest-roasbeef-v%s.sig", version)

    if _, err := os.Stat(manifestFile); err != nil {
        vlog("FAIL: LND manifest not found")
        return fmt.Errorf("LND manifest not found at %s", manifestFile)
    }

    sigURL := fmt.Sprintf(
        "https://github.com/lightningnetwork/lnd/releases/download/v%s/manifest-roasbeef-v%s.sig",
        version, version)
    if err := system.Download(sigURL, sigFile); err != nil {
        vlog("FAIL: download LND signature: %v", err)
        return fmt.Errorf("download LND signature: %w", err)
    }
    defer os.Remove(sigFile)

    cmd := exec.Command("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, manifestFile)
    output, _ := cmd.CombinedOutput()
    outputStr := string(output)

    if !strings.Contains(outputStr, "GOODSIG") {
        vlog("FAIL: LND signature invalid: %s", outputStr)
        return fmt.Errorf("LND signature verification failed")
    }

    // Log the GOODSIG line
    for _, line := range strings.Split(outputStr, "\n") {
        if strings.Contains(line, "GOODSIG") {
            vlog("GOODSIG: %s", strings.TrimSpace(line))
        }
    }

    vlog("OK LND: signature valid")
    return nil
}

func verifyLITSig(version string) error {
    vlog("--- LIT signature verification ---")
    manifestFile := "/tmp/lit-manifest.txt"
    sigFile := fmt.Sprintf("/tmp/manifest-ViktorT-11-v%s.sig", version)

    if _, err := os.Stat(manifestFile); err != nil {
        vlog("FAIL: LIT manifest not found")
        return fmt.Errorf("LIT manifest not found at %s", manifestFile)
    }

    sigURL := fmt.Sprintf(
        "https://github.com/lightninglabs/lightning-terminal/releases/download/v%s/manifest-ViktorT-11-v%s.sig",
        version, version)
    if err := system.Download(sigURL, sigFile); err != nil {
        vlog("FAIL: download LIT signature: %v", err)
        return fmt.Errorf("download LIT signature: %w", err)
    }
    defer os.Remove(sigFile)

    cmd := exec.Command("gpg", "--batch", "--verify",
        "--status-fd", "1", sigFile, manifestFile)
    output, _ := cmd.CombinedOutput()
    outputStr := string(output)

    if !strings.Contains(outputStr, "GOODSIG") {
        vlog("FAIL: LIT signature invalid: %s", outputStr)
        return fmt.Errorf("LIT signature verification failed")
    }

    for _, line := range strings.Split(outputStr, "\n") {
        if strings.Contains(line, "GOODSIG") {
            vlog("GOODSIG: %s", strings.TrimSpace(line))
        }
    }

    vlog("OK LIT: signature valid")
    return nil
}

// ── Checksum verification ────────────────────────────────

func verifyBitcoin(version string) error {
    vlog("--- Bitcoin Core checksum verification ---")
    cmd := exec.Command("sha256sum", "--ignore-missing", "--check", "SHA256SUMS")
    cmd.Dir = "/tmp"
    output, err := cmd.CombinedOutput()
    if err != nil {
        vlog("FAIL: Bitcoin Core checksum: %s", string(output))
        return fmt.Errorf("checksum failed: %s: %s", err, output)
    }
    vlog("OK Bitcoin Core checksum: %s", strings.TrimSpace(string(output)))
    return nil
}

func verifyLND(version string) error {
    vlog("--- LND checksum verification ---")
    if _, err := os.Stat("/tmp/manifest.txt"); err != nil {
        vlog("FAIL: LND manifest not found")
        return fmt.Errorf("LND manifest not found")
    }
    cmd := exec.Command("sha256sum", "--ignore-missing", "--check", "manifest.txt")
    cmd.Dir = "/tmp"
    output, err := cmd.CombinedOutput()
    if err != nil {
        vlog("FAIL: LND checksum: %s", string(output))
        return fmt.Errorf("checksum failed: %s: %s", err, output)
    }
    vlog("OK LND checksum: %s", strings.TrimSpace(string(output)))
    return nil
}

func verifyLIT(version string) error {
    vlog("--- LIT checksum verification ---")
    if _, err := os.Stat("/tmp/lit-manifest.txt"); err != nil {
        vlog("FAIL: LIT manifest not found")
        return fmt.Errorf("LIT manifest not found")
    }
    cmd := exec.Command("sha256sum", "--ignore-missing", "--check", "lit-manifest.txt")
    cmd.Dir = "/tmp"
    output, err := cmd.CombinedOutput()
    if err != nil {
        vlog("FAIL: LIT checksum: %s", string(output))
        return fmt.Errorf("checksum failed: %s: %s", err, output)
    }
    vlog("OK LIT checksum: %s", strings.TrimSpace(string(output)))
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
    return system.Download(url, "/tmp/SHA256SUMS.asc")
}

// ParseGoodSigCount counts GOODSIG lines in GPG status output.
func ParseGoodSigCount(output string) int {
    return strings.Count(output, "GOODSIG")
}

// ParseBadSigCount counts BADSIG lines in GPG status output.
func ParseBadSigCount(output string) int {
    return strings.Count(output, "BADSIG")
}

// HasGoodSig checks if GPG output contains at least one GOODSIG.
func HasGoodSig(output string) bool {
    return strings.Contains(output, "GOODSIG")
}

