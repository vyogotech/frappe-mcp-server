package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"

	"github.com/stretchr/testify/require"
)

// An IPv6 host needs brackets in the listen address, or the server stops at startup.
func TestTheServerListensOnAnIPv6Host(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback here")
	}
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	cfg := &config.Config{ERPNext: config.ERPNextConfig{BaseURL: "http://frappe.invalid"},
		Server: config.ServerConfig{Host: "::1", Port: port}}
	client, err := frappe.NewClient(cfg.ERPNext)
	require.NoError(t, err)
	s, err := NewMCPServer(cfg, client)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	defer func() { cancel(); <-done }()

	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://[::1]:%d/health", port))
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	}, 5*time.Second, 50*time.Millisecond)

	require.Equal(t, fmt.Sprintf("[::1]:%d", port), s.httpServer.Addr)
}
