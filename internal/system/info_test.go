// internal/system/info_test.go

package system

import "testing"

func TestParseSourceIPStandardVPS(t *testing.T) {
	output := "1.1.1.1 via 10.0.0.1 dev eth0 src 203.0.113.50 uid 0"
	got := ParseSourceIP(output)
	if got != "203.0.113.50" {
		t.Errorf("got %q, want 203.0.113.50", got)
	}
}

func TestParseSourceIPDirectRoute(t *testing.T) {
	output := "1.1.1.1 dev eth0 src 198.51.100.25 uid 0"
	got := ParseSourceIP(output)
	if got != "198.51.100.25" {
		t.Errorf("got %q, want 198.51.100.25", got)
	}
}

func TestParseSourceIPPrivateIP(t *testing.T) {
	output := "1.1.1.1 via 10.0.0.1 dev eth0 src 10.0.0.5 uid 0"
	got := ParseSourceIP(output)
	if got != "" {
		t.Errorf("private IP should return empty, got %q", got)
	}
}

func TestParseSourceIPPrivate172(t *testing.T) {
	output := "1.1.1.1 via 172.16.0.1 dev eth0 src 172.16.0.5 uid 0"
	got := ParseSourceIP(output)
	if got != "" {
		t.Errorf("172.16 private IP should return empty, got %q", got)
	}
}

func TestParseSourceIPPrivate192(t *testing.T) {
	output := "1.1.1.1 via 192.168.1.1 dev eth0 src 192.168.1.50 uid 0"
	got := ParseSourceIP(output)
	if got != "" {
		t.Errorf("192.168 private IP should return empty, got %q", got)
	}
}

func TestParseSourceIPLoopback(t *testing.T) {
	output := "1.1.1.1 dev lo src 127.0.0.1 uid 0"
	got := ParseSourceIP(output)
	if got != "" {
		t.Errorf("loopback should return empty, got %q", got)
	}
}

func TestParseSourceIPEmpty(t *testing.T) {
	got := ParseSourceIP("")
	if got != "" {
		t.Errorf("empty should return empty, got %q", got)
	}
}

func TestParseSourceIPMalformed(t *testing.T) {
	got := ParseSourceIP("RTNETLINK answers: Network is unreachable")
	if got != "" {
		t.Errorf("malformed should return empty, got %q", got)
	}
}

func TestParseSourceIPNoSrc(t *testing.T) {
	got := ParseSourceIP("1.1.1.1 via 10.0.0.1 dev eth0")
	if got != "" {
		t.Errorf("no src field should return empty, got %q", got)
	}
}

func TestParseSourceIPMultipleSpaces(t *testing.T) {
	output := "1.1.1.1 via 10.0.0.1 dev eth0  src  203.0.113.50  uid 0"
	got := ParseSourceIP(output)
	if got != "203.0.113.50" {
		t.Errorf("got %q, want 203.0.113.50", got)
	}
}
