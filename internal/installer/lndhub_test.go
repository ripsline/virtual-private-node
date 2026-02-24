// internal/installer/lndhub_test.go

package installer

import (
	"encoding/json"
	"testing"
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

	// Login should be empty when error response
	if account.Login != "" {
		t.Errorf("Login should be empty on error response, got %q", account.Login)
	}
}
