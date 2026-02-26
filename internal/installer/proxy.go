// internal/installer/proxy.go

package installer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ripsline/virtual-private-node/internal/config"
	"github.com/ripsline/virtual-private-node/internal/logger"
	"github.com/ripsline/virtual-private-node/internal/paths"
	"github.com/ripsline/virtual-private-node/internal/system"
)

// generateProxyCert creates a self-signed TLS certificate with the given IP
// as a Subject Alternative Name. The certificate is valid for 10 years.
//
// This uses the same approach as LND: ECDSA P-256 key, self-signed,
// IP in SAN. The Go standard library handles all cryptography.
func generateProxyCert(publicIP string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Virtual Private Node"},
			CommonName:   "lndhub-proxy",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	if ip := net.ParseIP(publicIP); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := system.SudoWriteFile(paths.LndHubProxyCert, certPEM, 0644); err != nil {
		return err
	}
	if err := system.SudoRun("chown", "root:"+systemUser, paths.LndHubProxyCert); err != nil {
		return err
	}
	if err := system.SudoWriteFile(paths.LndHubProxyKey, keyPEM, 0600); err != nil {
		return err
	}
	if err := system.SudoRun("chown", "root:"+systemUser, paths.LndHubProxyKey); err != nil {
		return err
	}

	logger.Install("Generated TLS certificate for LndHub proxy (IP: %s)", publicIP)
	return nil
}

func writeLndHubProxyService() error {
	content := fmt.Sprintf(`[Unit]
Description=LndHub TLS Proxy
After=lndhub.service
Wants=lndhub.service

[Service]
Type=simple
User=%s
Group=%s
ExecStart=/usr/local/bin/rlvpn proxy
Restart=on-failure
RestartSec=10
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
`, systemUser, systemUser)
	return system.SudoWriteFile(paths.LndHubProxyService, []byte(content), 0644)
}

func startLndHubProxy() error {
	if err := system.SudoRun("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := system.SudoRun("systemctl", "enable", "lndhub-proxy"); err != nil {
		return err
	}
	return system.SudoRun("systemctl", "start", "lndhub-proxy")
}

func stopLndHubProxy() error {
	system.SudoRunSilent("systemctl", "stop", "lndhub-proxy")
	system.SudoRunSilent("systemctl", "disable", "lndhub-proxy")
	return nil
}

// installLndHubProxy generates the cert, writes the service, and starts the proxy.
// Called when both LndHub and hybrid P2P mode are active.
func installLndHubProxy(cfg *config.AppConfig, publicIP string) error {
	if err := generateProxyCert(publicIP); err != nil {
		return fmt.Errorf("generate proxy cert: %w", err)
	}
	if err := writeLndHubProxyService(); err != nil {
		return fmt.Errorf("write proxy service: %w", err)
	}
	if err := startLndHubProxy(); err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}
	logger.Install("LndHub TLS proxy installed and started")
	return nil
}

// needsProxy returns true when the TLS proxy should be running.
func needsProxy(cfg *config.AppConfig) bool {
	return cfg.LndHubInstalled && cfg.P2PMode == "hybrid"
}
