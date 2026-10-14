package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"

	"github.com/stretchr/testify/require"
)

func TestMCPServerServesHTTPSWithAutoCert(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	cfg := &config.Config{
		ERPNext: config.ERPNextConfig{BaseURL: "http://frappe.invalid"},
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: port,
			TLS: config.TLSConfig{
				Enabled:      true,
				AutoGenerate: true,
			},
		},
	}
	client, err := frappe.NewClient(cfg.ERPNext)
	require.NoError(t, err)
	s, err := NewMCPServer(cfg, client)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	defer func() { cancel(); <-done }()

	// Client with InsecureSkipVerify to test against self-signed cert
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- test code verifying TLS endpoint
	}
	httpClient := &http.Client{Transport: tr, Timeout: 2 * time.Second}

	require.Eventually(t, func() bool {
		resp, err := httpClient.Get(fmt.Sprintf("https://127.0.0.1:%d/health", port))
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 50*time.Millisecond)

	// Plain HTTP to the HTTPS port should either error or return 400 Bad Request
	plainClient := &http.Client{Timeout: 1 * time.Second}
	resp, err := plainClient.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err == nil {
		_ = resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	}
}
