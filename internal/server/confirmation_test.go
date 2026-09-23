package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/tools"
)

const testRedeemMethod = "frappe_ai.api.confirm.redeem"

// writeCall is what a write tool is called with and what Frappe should be told it is about. A write tool in
// the catalogue with no entry here fails the test, which is how the gate reaches a tool added later.
var writeCall = map[string]struct{ args, doctype, name string }{
	"create_document": {`{"doctype":"ToDo","data":{"description":"a thing"}}`, "ToDo", ""},
	"update_document": {`{"doctype":"ToDo","name":"TODO-0001","data":{"status":"Closed"}}`, "ToDo", "TODO-0001"},
	"delete_document": {`{"doctype":"ToDo","name":"TODO-0001"}`, "ToDo", "TODO-0001"},
}

type redeemRequest struct {
	Token   string `json:"token"`
	Tool    string `json:"tool"`
	Doctype string `json:"doctype"`
	Name    string `json:"name"`
}

type recordedRequest struct {
	method, path, body string
	headers            http.Header
}

// fakeSite is a Frappe that answers the redeem method for the tokens it was given and records everything it is asked.
type fakeSite struct {
	mu        sync.Mutex
	requests  []recordedRequest
	redeems   []redeemRequest
	live      map[string]bool // tokens not yet spent
	redeemErr int             // status to answer the redeem method with, 0 for the normal answer
	abort     bool            // drop the connection on the redeem method
	server    *httptest.Server
}

func newFakeSite(t *testing.T) *fakeSite {
	t.Helper()
	site := &fakeSite{live: map[string]bool{}}
	site.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		site.mu.Lock()
		site.requests = append(site.requests, recordedRequest{r.Method, r.URL.Path, string(body), r.Header.Clone()})
		site.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/method/"+testRedeemMethod {
			_, _ = w.Write([]byte(`{"data":{"name":"TODO-0001"}}`))
			return
		}

		site.mu.Lock()
		defer site.mu.Unlock()
		if site.abort {
			panic(http.ErrAbortHandler) // the method is there but the connection dies: an unreachable redeem
		}
		if site.redeemErr != 0 {
			w.WriteHeader(site.redeemErr)
			_, _ = w.Write([]byte(`{"exception":"frappe.exceptions.DoesNotExistError"}`))
			return
		}
		var req redeemRequest
		_ = json.Unmarshal(body, &req)
		site.redeems = append(site.redeems, req)
		if !site.live[req.Token] {
			w.WriteHeader(http.StatusExpectationFailed)
			_, _ = w.Write([]byte(`{"exception":"This request expired. Ask again to run it."}`))
			return
		}
		delete(site.live, req.Token) // one-time, as frappe_ai's DEL makes it
		_, _ = w.Write([]byte(`{"message":{"ok":true}}`))
	}))
	t.Cleanup(site.server.Close)
	return site
}

func (f *fakeSite) mint(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.live[token] = true
}

// writes counts the requests that changed a document, which is the only thing that must not happen unconfirmed.
func (f *fakeSite) writes() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recordedRequest
	for _, r := range f.requests {
		if strings.HasPrefix(r.path, "/api/resource/") {
			out = append(out, r)
		}
	}
	return out
}

func newConfirmServer(t *testing.T, site *fakeSite, redeemMethod string) *MCPServer {
	t.Helper()
	cfg := &config.Config{Tools: config.ToolsConfig{ConfirmationRedeemMethod: redeemMethod}}
	cfg.ERPNext.BaseURL = site.server.URL
	cfg.ERPNext.APIKey, cfg.ERPNext.APISecret = "k", "s"
	cfg.ERPNext.RateLimit = config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100}
	cfg.Server.Port = 8080
	client, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPServer(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// entryPoints are every way a tool call reaches a handler: the SDK's /mcp, the REST tool path, and executeTool, which
// is what handleChat and the SSE chat both route through. Each returns what its caller ends up reading.
var entryPoints = []struct {
	name string
	call func(t *testing.T, s *MCPServer, tool, args, token string) string
}{
	{"tools/call", func(t *testing.T, s *MCPServer, tool, args, token string) string {
		t.Helper()
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + args + `}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			req.Header.Set("X-Frappe-Confirmation", token)
		}
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		return rec.Body.String()
	}},
	{"/api/v1/tools/", func(t *testing.T, s *MCPServer, tool, args, token string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/"+tool,
			bytes.NewBufferString(`{"id":"rest","params":`+args+`}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-Frappe-Confirmation", token)
		}
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		return rec.Body.String()
	}},
	{"executeTool", func(t *testing.T, s *MCPServer, tool, args, token string) string {
		t.Helper()
		ctx := context.Background()
		if token != "" {
			ctx = auth.WithConfirmation(ctx, token)
		}
		resp, err := s.executeTool(ctx, tool, json.RawMessage(args))
		if err != nil {
			return err.Error()
		}
		var text strings.Builder
		for _, c := range resp.Content {
			text.WriteString(c.Text)
		}
		return text.String()
	}},
}

// The whole point: a write tool the server did not publish as read-only does nothing at all unless the header carries
// a token Frappe redeems, through every entry point there is.
func TestAWriteWithoutAConfirmationIsRefused(t *testing.T) {
	for _, tool := range writeTools {
		call, ok := writeCall[tool]
		if !ok {
			t.Fatalf("%s is a write tool with no writeCall entry: the gate is untested for it", tool)
		}
		for _, entry := range entryPoints {
			t.Run(tool+"/"+entry.name, func(t *testing.T) {
				t.Run("no token", func(t *testing.T) {
					site := newFakeSite(t)
					s := newConfirmServer(t, site, testRedeemMethod)
					got := entry.call(t, s, tool, call.args, "")
					requireRefused(t, site, got)
				})

				t.Run("a token frappe rejects", func(t *testing.T) {
					site := newFakeSite(t)
					s := newConfirmServer(t, site, testRedeemMethod)
					got := entry.call(t, s, tool, call.args, "a-token-nobody-minted")
					requireRefused(t, site, got)
					if len(site.redeems) != 1 {
						t.Errorf("redeem was called %d times, want 1", len(site.redeems))
					}
				})

				t.Run("the redeem method is absent", func(t *testing.T) {
					site := newFakeSite(t)
					site.redeemErr = http.StatusNotFound
					s := newConfirmServer(t, site, testRedeemMethod)
					site.mint("a-real-token")
					requireRefused(t, site, entry.call(t, s, tool, call.args, "a-real-token"))
				})

				t.Run("the redeem method is unreachable", func(t *testing.T) {
					site := newFakeSite(t)
					site.abort = true
					s := newConfirmServer(t, site, testRedeemMethod)
					site.mint("a-real-token")
					requireRefused(t, site, entry.call(t, s, tool, call.args, "a-real-token"))
				})

				t.Run("a redeemed token writes once and cannot be replayed", func(t *testing.T) {
					site := newFakeSite(t)
					s := newConfirmServer(t, site, testRedeemMethod)
					const token = "ffffffffffffffffffffffffffffffff"
					site.mint(token)

					got := entry.call(t, s, tool, call.args, token)
					if strings.Contains(got, tools.RefusalNotConfirmed) {
						t.Fatalf("a redeemed token was refused: %s", got)
					}
					if n := len(site.writes()); n != 1 {
						t.Fatalf("the confirmed call made %d writes, want 1", n)
					}
					if len(site.redeems) != 1 {
						t.Fatalf("redeem was called %d times, want 1", len(site.redeems))
					}
					if r := site.redeems[0]; r.Token != token || r.Tool != tool || r.Doctype != call.doctype || r.Name != call.name {
						t.Errorf("redeem got %+v, want token %q tool %q doctype %q name %q",
							r, token, tool, call.doctype, call.name)
					}
					requireTokenWentNowhereElse(t, site, token, got)

					// the same token again: frappe spent it, so the second call writes nothing
					second := entry.call(t, s, tool, call.args, token)
					if !strings.Contains(second, tools.RefusalNotConfirmed) {
						t.Errorf("a replayed token got %q, want %q", second, tools.RefusalNotConfirmed)
					}
					if n := len(site.writes()); n != 1 {
						t.Errorf("a replayed token brought the write count to %d, want 1", n)
					}
				})
			})
		}
	}
}

func requireRefused(t *testing.T, site *fakeSite, got string) {
	t.Helper()
	if !strings.Contains(got, tools.RefusalNotConfirmed) {
		t.Errorf("caller read %q, want %q", got, tools.RefusalNotConfirmed)
	}
	if w := site.writes(); len(w) != 0 {
		t.Errorf("%d write(s) reached Frappe unconfirmed: %+v", len(w), w)
	}
}

// The token is a credential the model must never come to hold: it belongs in the redeem call and nowhere else.
func requireTokenWentNowhereElse(t *testing.T, site *fakeSite, token, result string) {
	t.Helper()
	if strings.Contains(result, token) {
		t.Error("the tool result the model reads contains the confirmation token")
	}
	site.mu.Lock()
	defer site.mu.Unlock()
	for _, r := range site.requests {
		if r.path == "/api/method/"+testRedeemMethod {
			continue
		}
		if strings.Contains(r.body, token) {
			t.Errorf("%s %s carried the confirmation token in its body", r.method, r.path)
		}
		for name, values := range r.headers {
			for _, v := range values {
				if strings.Contains(v, token) {
					t.Errorf("%s %s carried the confirmation token in header %s", r.method, r.path, name)
				}
			}
		}
	}
}
