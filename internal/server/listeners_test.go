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

// Every listener must sit behind the auth middleware, so the server opens exactly one.
func TestTheServerOpensOnlyItsAuthenticatedListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	cfg := &config.Config{ERPNext: config.ERPNextConfig{BaseURL: "http://frappe.invalid"},
		Server: config.ServerConfig{Host: "127.0.0.1", Port: port}}
	client, err := frappe.NewClient(cfg.ERPNext)
	require.NoError(t, err)
	s, err := NewMCPServer(cfg, client)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	defer func() { cancel(); <-done }()

	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	}, 5*time.Second, 50*time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port+1), time.Second)
	if err == nil {
		_ = conn.Close()
	}
	require.Error(t, err, "something is listening on port+1, outside the auth middleware")
}
