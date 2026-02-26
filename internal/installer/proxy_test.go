// internal/installer/proxy_test.go

package installer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/ripsline/virtual-private-node/internal/config"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	// Test the certificate generation logic directly
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("203.0.113.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	if len(cert.IPAddresses) != 2 {
		t.Errorf("IP SANs: got %d, want 2", len(cert.IPAddresses))
	}
	if cert.IPAddresses[0].String() != "127.0.0.1" {
		t.Errorf("first IP: got %s, want 127.0.0.1", cert.IPAddresses[0])
	}
	if cert.IPAddresses[1].String() != "203.0.113.1" {
		t.Errorf("second IP: got %s, want 203.0.113.1", cert.IPAddresses[1])
	}
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "localhost" {
		t.Errorf("DNS SANs: got %v, want [localhost]", cert.DNSNames)
	}
	if cert.NotAfter.Before(time.Now().Add(9 * 365 * 24 * time.Hour)) {
		t.Error("cert should be valid for ~10 years")
	}
}

func TestCertPEMEncoding(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("PEM decode returned nil")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("PEM type: got %q, want CERTIFICATE", block.Type)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		t.Fatal("key PEM decode returned nil")
	}
	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("key PEM type: got %q, want EC PRIVATE KEY", keyBlock.Type)
	}
}

func TestNeedsProxy(t *testing.T) {
	tests := []struct {
		name   string
		hub    bool
		mode   string
		expect bool
	}{
		{"tor only no hub", false, "tor", false},
		{"tor only with hub", true, "tor", false},
		{"hybrid no hub", false, "hybrid", false},
		{"hybrid with hub", true, "hybrid", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				LndHubInstalled: tt.hub,
				P2PMode:         tt.mode,
			}
			if got := needsProxy(cfg); got != tt.expect {
				t.Errorf("needsProxy: got %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestCertWithoutPublicIP(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Simulate empty publicIP — no additional IP added
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, _ := x509.ParseCertificate(certDER)
	if len(cert.IPAddresses) != 1 {
		t.Errorf("should only have localhost IP, got %d", len(cert.IPAddresses))
	}
}
