// internal/installer/lndhub_test.go

package installer

import (
	"encoding/json"
	"testing"

	"github.com/ripsline/virtual-private-node/internal/config"
	"github.com/ripsline/virtual-private-node/internal/paths"
)

func TestLndHubVersionStr(t *testing.T) {
	v := LndHubVersionStr()
	if v == "" {
		t.Error("LndHubVersionStr returned empty")
	}
	if v != lndhubVersion {
		t.Errorf("got %q, want %q", v, lndhubVersion)
	}
}

func TestLndHubVersionConstants(t *testing.T) {
	if lndhubVersion == "" {
		t.Error("lndhubVersion is empty")
	}
	if lndhubRepo == "" {
		t.Error("lndhubRepo is empty")
	}
	if goVersion == "" {
		t.Error("goVersion is empty")
	}
	if goDownloadURL == "" {
		t.Error("goDownloadURL is empty")
	}
}

func TestGoConstants(t *testing.T) {
	if goVersion != "1.26.0" {
		t.Errorf("goVersion: got %q, want 1.26.0", goVersion)
	}
	if goInstallDir != "/usr/local/go" {
		t.Errorf("goInstallDir: got %q, want /usr/local/go", goInstallDir)
	}
}

func TestLndHubAccountParsing(t *testing.T) {
	raw := `{"login":"MLS53uD0uTQUiDt3qd9H","password":"T87I8RUd7eQandfGGnpn"}`

	var account LndHubAccount
	if err := json.Unmarshal([]byte(raw), &account); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if account.Login != "MLS53uD0uTQUiDt3qd9H" {
		t.Errorf("Login: got %q", account.Login)
	}
	if account.Password != "T87I8RUd7eQandfGGnpn" {
		t.Errorf("Password: got %q", account.Password)
	}
}

func TestLndHubAccountParsingEmpty(t *testing.T) {
	raw := `{}`

	var account LndHubAccount
	if err := json.Unmarshal([]byte(raw), &account); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if account.Login != "" {
		t.Errorf("Login should be empty, got %q", account.Login)
	}
}

func TestLndHubAccountParsingError(t *testing.T) {
	raw := `{"error":true,"code":1,"message":"bad auth"}`

	var account LndHubAccount
	if err := json.Unmarshal([]byte(raw), &account); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if account.Login != "" {
		t.Errorf("Login should be empty on error response, got %q", account.Login)
	}
}

func TestLndHubAccountParsingWithLabel(t *testing.T) {
	raw := `{"login":"MLS53uD0uTQUiDt3qd9H","password":"T87I8RUd7eQandfGGnpn"}`

	var account LndHubAccount
	if err := json.Unmarshal([]byte(raw), &account); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if account.Login == "" {
		t.Error("Login should not be empty")
	}
}

func TestConfigLndHubAccount(t *testing.T) {
	cfg := config.Default()
	cfg.LndHubAccounts = append(cfg.LndHubAccounts, config.LndHubAccount{
		Label:     "Alice",
		Login:     "abc123",
		CreatedAt: "2026-02-23",
		Active:    true,
	})

	if len(cfg.LndHubAccounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(cfg.LndHubAccounts))
	}
	if cfg.LndHubAccounts[0].Label != "Alice" {
		t.Errorf("Label: got %q, want Alice", cfg.LndHubAccounts[0].Label)
	}
	if !cfg.LndHubAccounts[0].Active {
		t.Error("account should be active")
	}
}

func TestConfigLndHubAccountDeactivated(t *testing.T) {
	acct := config.LndHubAccount{
		Label:               "Bob",
		Login:               "def456",
		CreatedAt:           "2026-02-23",
		Active:              false,
		DeactivatedAt:       "2026-02-24",
		BalanceOnDeactivate: "5000",
	}

	if acct.Active {
		t.Error("should be inactive")
	}
	if acct.BalanceOnDeactivate != "5000" {
		t.Errorf("balance: got %q, want 5000", acct.BalanceOnDeactivate)
	}
	if acct.DeactivatedAt != "2026-02-24" {
		t.Errorf("deactivated: got %q", acct.DeactivatedAt)
	}
}

// ── Login validation tests ───────────────────────────────

func TestValidateLoginValid(t *testing.T) {
	valid := []string{
		"MLS53uD0uTQUiDt3qd9H",
		"abc123",
		"ABCDEF",
		"a",
		"0123456789",
	}
	for _, login := range valid {
		if err := validateLogin(login); err != nil {
			t.Errorf("validateLogin(%q) should pass: %v", login, err)
		}
	}
}

func TestValidateLoginInvalid(t *testing.T) {
	invalid := []string{
		"",
		"abc 123",
		"abc'123",
		"abc;DROP TABLE users;--",
		"abc\n123",
		"abc/123",
		"abc\"123",
		"abc$123",
		"abc|123",
		"abc&123",
	}
	for _, login := range invalid {
		if err := validateLogin(login); err == nil {
			t.Errorf("validateLogin(%q) should fail", login)
		}
	}
}

func TestValidateLoginMaxLength(t *testing.T) {
	// 40 chars should pass
	long40 := "abcdefghijklmnopqrstuvwxyz01234567890123"
	if len(long40) != 40 {
		t.Fatalf("test string length: %d", len(long40))
	}
	if err := validateLogin(long40); err != nil {
		t.Errorf("40-char login should pass: %v", err)
	}

	// 41 chars should fail
	long41 := long40 + "x"
	if err := validateLogin(long41); err == nil {
		t.Error("41-char login should fail")
	}
}

// ── Port constants tests ─────────────────────────────────

func TestLndHubPortConstants(t *testing.T) {
	if paths.LndHubInternalPort != "3004" {
		t.Errorf("internal port: got %q, want 3004", paths.LndHubInternalPort)
	}
	if paths.LndHubExternalPort != "3000" {
		t.Errorf("external port: got %q, want 3000", paths.LndHubExternalPort)
	}
}
