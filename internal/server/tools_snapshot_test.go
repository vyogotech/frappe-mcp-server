package server

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"

	"github.com/stretchr/testify/require"
)

const golden = "testdata/tools_list.golden.json"

var update = flag.Bool("update", false, "rewrite "+golden+" from the live registration")

func TestToolsListMatchesSnapshot(t *testing.T) {
	// ragbot's config switches the knowledge base on, and that is the list its model sees
	cfg := &config.Config{ERPNext: config.ERPNextConfig{BaseURL: "http://frappe.invalid"}, Tools: config.ToolsConfig{KnowledgeBase: true}}
	client, err := frappe.NewClient(cfg.ERPNext)
	require.NoError(t, err)
	s, err := NewMCPServer(cfg, client)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	s.server.HandleStreamableHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	tools := resp.Result.Tools
	sort.Slice(tools, func(i, j int) bool { return tools[i]["name"].(string) < tools[j]["name"].(string) })
	got, err := json.MarshalIndent(tools, "", "  ")
	require.NoError(t, err)

	if *update {
		require.NoError(t, os.WriteFile(golden, append(got, '\n'), 0o600))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got), "tools/list changed: rerun with -update and review the diff")
}
