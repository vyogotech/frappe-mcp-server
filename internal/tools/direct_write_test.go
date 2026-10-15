package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/types"
)

const testDirectMethod = "test_direct.authorize"

const testProposeMethod = "test_direct.propose_change"

const testOAuthToken = "examplia-oauth-token" //nolint:gosec // G101: a made-up token for a fake site

const testAPIKeyPair = "examplia-key:examplia-secret" //nolint:gosec // G101: a made-up key for a fake site

// directSite is a Frappe whose direct-write method answers with a fixed verdict, recording what it was asked.
type directSite struct {
	mu         sync.Mutex
	authorizes []map[string]any
	headers    []http.Header
	writes     int
	ok         bool
	reason     string
	propose    bool             // answer "propose" for this caller, as frappe_ai does for Vyogo Forge AI
	proposals  []map[string]any // what the propose method was sent
	status     int              // non-zero: answer the method with this status, as a site without frappe_ai would
	server     *httptest.Server
}

func newDirectSite(t *testing.T, ok bool, reason string) *directSite {
	t.Helper()
	site := &directSite{ok: ok, reason: reason}
	site.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		site.mu.Lock()
		defer site.mu.Unlock()
		if r.URL.Path == "/api/method/"+testProposeMethod {
			var args map[string]any
			_ = json.Unmarshal(body, &args)
			site.proposals = append(site.proposals, args)
			_, _ = w.Write([]byte(`{"message":{"name":"VF-PR-00007","status":"Pending","approve_url":"https://site.test/app/vyogo-forge-ai-proposed-release/VF-PR-00007"}}`))
			return
		}
		if r.URL.Path == "/api/method/"+testDirectMethod {
			if site.status != 0 {
				w.WriteHeader(site.status)
				_, _ = w.Write([]byte(`{"exc_type":"DoesNotExistError"}`))
				return
			}
			var args map[string]any
			_ = json.Unmarshal(body, &args)
			site.authorizes = append(site.authorizes, args)
			site.headers = append(site.headers, r.Header.Clone())
			out, _ := json.Marshal(map[string]any{"message": map[string]any{"ok": site.ok, "reason": site.reason, "propose": site.propose}})
			_, _ = w.Write(out)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/resource/") && r.Method != http.MethodGet {
			site.writes++
		}
		_, _ = w.Write([]byte(`{"data":{"name":"CRM-LEAD-0001"}}`))
	}))
	t.Cleanup(site.server.Close)
	return site
}

func directRegistry(t *testing.T, site *directSite) *ToolRegistry {
	t.Helper()
	client, err := frappe.NewClient(config.ERPNextConfig{
		BaseURL:   site.server.URL,
		Timeout:   5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	require.NoError(t, err)
	registry := NewRegistry(client)
	registry.DirectWriteMethod = testDirectMethod
	registry.ProposeMethod = testProposeMethod
	return registry
}

func oauthUser() context.Context {
	return auth.WithUser(context.Background(), &types.User{Email: "writer@example.test", Token: testOAuthToken})
}

func updateLead(t *testing.T, registry *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) {
	t.Helper()
	return registry.UpdateDocument(ctx, mcp.ToolRequest{
		ID:     "direct",
		Tool:   "update_document",
		Params: json.RawMessage(`{"doctype":"CRM Lead","name":"CRM-LEAD-0001","data":{"website":"https://examplia.test"}}`),
	})
}

// A client signed in with OAuth (Claude Desktop, Claude Code) has no sidebar token: the site decides.
func TestAnOAuthClientsWriteIsDecidedByTheSite(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		site := newDirectSite(t, true, "")
		_, err := updateLead(t, directRegistry(t, site), oauthUser())
		require.NoError(t, err)
		assert.Equal(t, 1, site.writes)
		require.Len(t, site.authorizes, 1)
		// the site is told which write, never the change's data
		assert.Equal(t, map[string]any{"tool": "update_document", "doctype": "CRM Lead", "name": "CRM-LEAD-0001"},
			site.authorizes[0])
		// and is asked as the user, through the same token
		assert.Equal(t, "Bearer "+testOAuthToken, site.headers[0].Get("Authorization"))
	})

	t.Run("refused, with the site's reason", func(t *testing.T) {
		site := newDirectSite(t, false, "This site is read only for other MCP clients (AI Assistant Settings).")
		_, err := updateLead(t, directRegistry(t, site), oauthUser())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read only for other MCP clients")
		assert.Zero(t, site.writes)
	})

	t.Run("a site without the method refuses", func(t *testing.T) {
		site := newDirectSite(t, true, "")
		site.status = http.StatusNotFound
		_, err := updateLead(t, directRegistry(t, site), oauthUser())
		require.Error(t, err)
		assert.Contains(t, err.Error(), RefusalNotConfirmed)
		assert.Zero(t, site.writes)
	})
}

// A client signed in with an API key and secret takes the same path, and acts as the key's user throughout.
func TestAnAPIKeyClientsWriteIsDecidedByTheSite(t *testing.T) {
	site := newDirectSite(t, true, "")
	ctx := auth.WithUser(context.Background(), &types.User{Email: "writer@example.test", APIKey: testAPIKeyPair})
	_, err := updateLead(t, directRegistry(t, site), ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, site.writes)
	require.Len(t, site.authorizes, 1)
	assert.Equal(t, "token "+testAPIKeyPair, site.headers[0].Get("Authorization"))
}

// The sidebar's agent acts on the user's browser session: without its token nothing is asked and nothing is written.
func TestASessionsWriteNeverReachesTheDirectPath(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"sid":              auth.WithUser(context.Background(), &types.User{Email: "u@example.test", SessionID: "a-sid"}),
		"sid and token":    auth.WithUser(context.Background(), &types.User{Email: "u@example.test", SessionID: "a-sid", Token: "t"}),
		"the server's key": context.Background(),
	} {
		t.Run(name, func(t *testing.T) {
			site := newDirectSite(t, true, "")
			_, err := updateLead(t, directRegistry(t, site), ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), RefusalNotConfirmed)
			assert.Empty(t, site.authorizes, "a session or the server's own key must not be offered the direct path")
			assert.Zero(t, site.writes)
		})
	}
}

func TestEveryWriteToolTakesTheDirectPath(t *testing.T) {
	calls := map[string]func(*ToolRegistry, context.Context) (*mcp.ToolResponse, error){
		"create_document": func(r *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) {
			return r.CreateDocument(ctx, mcp.ToolRequest{Params: json.RawMessage(`{"doctype":"CRM Lead","data":{"first_name":"Examplia"}}`)})
		},
		"create_documents": func(r *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) {
			return r.CreateDocuments(ctx, mcp.ToolRequest{Params: json.RawMessage(`{"doctype":"CRM Lead","rows":[{"first_name":"A"},{"first_name":"B"}]}`)})
		},
		"update_document": func(r *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) {
			return updateLead(t, r, ctx)
		},
		"delete_document": func(r *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) {
			return r.DeleteDocument(ctx, mcp.ToolRequest{Params: json.RawMessage(`{"doctype":"CRM Lead","name":"CRM-LEAD-0001"}`)})
		},
	}
	want := map[string]string{"create_document": "", "create_documents": "2 rows", "update_document": "CRM-LEAD-0001", "delete_document": "CRM-LEAD-0001"}
	for tool, call := range calls {
		t.Run(tool, func(t *testing.T) {
			site := newDirectSite(t, false, "no")
			_, err := call(directRegistry(t, site), oauthUser())
			require.Error(t, err)
			require.Len(t, site.authorizes, 1)
			assert.Equal(t, tool, site.authorizes[0]["tool"])
			assert.Equal(t, want[tool], site.authorizes[0]["name"])
		})
	}
}

// Vyogo Forge AI writes through the gateway too: the site says "propose", and the write is filed as a proposed release
// a System Manager approves. The tool succeeds with where the proposal waits; nothing is written.
func TestVyogoForgeAIsWriteBecomesAProposal(t *testing.T) {
	cases := map[string]struct {
		call func(*ToolRegistry, context.Context) (*mcp.ToolResponse, error)
		want map[string]any
	}{
		"update_document": {
			call: func(r *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) { return updateLead(t, r, ctx) },
			want: map[string]any{"tool": "update_document", "doctype": "CRM Lead", "name": "CRM-LEAD-0001",
				"data": map[string]any{"website": "https://examplia.test"}},
		},
		"create_documents": {
			call: func(r *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) {
				return r.CreateDocuments(ctx, mcp.ToolRequest{Params: json.RawMessage(`{"doctype":"CRM Lead","rows":[{"first_name":"A"},{"first_name":"B"}]}`)})
			},
			want: map[string]any{"tool": "create_documents", "doctype": "CRM Lead", "name": "2 rows",
				"data": []any{map[string]any{"first_name": "A"}, map[string]any{"first_name": "B"}}},
		},
		"delete_document": {
			call: func(r *ToolRegistry, ctx context.Context) (*mcp.ToolResponse, error) {
				return r.DeleteDocument(ctx, mcp.ToolRequest{Params: json.RawMessage(`{"doctype":"CRM Lead","name":"CRM-LEAD-0001"}`)})
			},
			want: map[string]any{"tool": "delete_document", "doctype": "CRM Lead", "name": "CRM-LEAD-0001", "data": nil},
		},
	}
	for tool, c := range cases {
		t.Run(tool, func(t *testing.T) {
			site := newDirectSite(t, false, "Vyogo Forge AI's changes are proposed for a System Manager's approval.")
			site.propose = true
			ctx := auth.WithUser(context.Background(), &types.User{Email: "forge-agent@vyogoforge.invalid", APIKey: testAPIKeyPair})
			response, err := c.call(directRegistry(t, site), ctx)
			require.NoError(t, err)
			require.NotEmpty(t, response.Content)
			text := response.Content[0].Text
			assert.Contains(t, text, "VF-PR-00007")
			assert.Contains(t, text, "https://site.test/app/vyogo-forge-ai-proposed-release/VF-PR-00007")
			assert.Contains(t, text, "approve")
			assert.Zero(t, site.writes, "a proposal changes nothing until it is approved")
			require.Len(t, site.proposals, 1)
			assert.Equal(t, c.want, site.proposals[0])
		})
	}
}
