// internal/proxy/proxy.go

// Package proxy implements a TLS reverse proxy for LndHub clearnet connections.
//
// Usage: rlvpn proxy
//
// The proxy listens on 0.0.0.0:3000 with a self-signed TLS certificate
// and forwards requests to LndHub on 127.0.0.1:3004.
//
// The certificate is generated at install time with the server's public IP
// as a SAN. Users accept the certificate warning on first connection,
// same as LND REST over clearnet.
package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"golang.org/x/time/rate"

	"github.com/ripsline/virtual-private-node/internal/paths"
)

// Run starts the TLS reverse proxy. Called by "rlvpn proxy" subcommand.
func Run() error {
	certFile := paths.LndHubProxyCert
	keyFile := paths.LndHubProxyKey

	if _, err := os.Stat(certFile); err != nil {
		return fmt.Errorf("TLS certificate not found at %s — run install first", certFile)
	}
	if _, err := os.Stat(keyFile); err != nil {
		return fmt.Errorf("TLS key not found at %s — run install first", keyFile)
	}

	target, _ := url.Parse("http://127.0.0.1:" + paths.LndHubInternalPort)
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Rate limiter: 10 requests/second with burst of 20.
	// Protects LndHub from DoS. Normal wallet usage is well within limits.
	limiter := rate.NewLimiter(rate.Limit(10), 20)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// Strip X-Forwarded-For to prevent logging user IPs in LndHub.
		// LndHub authenticates by credentials, not by source IP.
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Real-Ip")

		// Security headers
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		proxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    "0.0.0.0:" + paths.LndHubExternalPort,
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 16, // 64KB max headers
	}

	fmt.Printf("LndHub TLS proxy listening on %s → 127.0.0.1:%s\n",
		server.Addr, paths.LndHubInternalPort)
	return server.ListenAndServeTLS(certFile, keyFile)
}
