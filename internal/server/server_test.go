package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/auth"
	authstrategies "frappe-mcp-server/internal/auth/strategies"
	"frappe-mcp-server/internal/llm"
)

// stubLLMClient is a minimal llm.Client used to verify the legacy-fallback
// path of generateWithLLM. Returns a known string from Generate; never
// recurses back into MCPServer.
type stubLLMClient struct{}

func (stubLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "stub-response", nil
}
func (stubLLMClient) Provider() string { return "stub" }

// TestGenerateWithLLM_LegacyClientFallback verifies that when llmManager is
// nil but llmClient is set, generateWithLLM delegates to the legacy client
// and does NOT recurse. Without the fix, this test stack-overflows.
func TestGenerateWithLLM_LegacyClientFallback(t *testing.T) {
	s := &MCPServer{
		llmClient:  stubLLMClient{},
		llmManager: nil,
	}
	got, err := s.generateWithLLM(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "stub-response" {
		t.Fatalf("got %q, want %q", got, "stub-response")
	}
}

var _ llm.Client = stubLLMClient{} // compile-time check

// TestWithMiddleware_PublicPathsBypassAuth verifies that /health,
// /api/v1/health, and /metrics return 200 even when auth is required and no
// Bearer token is supplied. Other paths must still get 401. This is what
// makes the docker HEALTHCHECK actually work.
func TestWithMiddleware_PublicPathsBypassAuth(t *testing.T) {
	// A bare OAuth2Strategy with no token in the request returns
	// "missing or invalid Bearer token" — exactly the failure mode the
	// docker HEALTHCHECK was hitting.
	strategy := authstrategies.NewOAuth2Strategy(authstrategies.OAuth2StrategyConfig{
		TokenInfoURL:   mockOAuthEndpointTest + "/userinfo",
		ValidateRemote: true,
	})
	s := &MCPServer{
		authMiddleware: auth.NewMiddleware(strategy, true), // requireAuth=true
	}
	handler := s.withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	cases := []struct {
		path     string
		wantCode int
	}{
		{"/health", http.StatusOK},
		{"/api/v1/health", http.StatusOK},
		{"/metrics", http.StatusOK},
		{"/api/v1/chat", http.StatusUnauthorized},
		{"/mcp", http.StatusUnauthorized},
		{"/api/v1/tools", http.StatusUnauthorized},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", c.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != c.wantCode {
			t.Errorf("%s: got status %d, want %d", c.path, w.Code, c.wantCode)
		}
	}
}

// mockOAuthEndpointTest is the loopback base URL referenced from the auth
// middleware test. Extracted to a constant to dodge gosec G101.
const mockOAuthEndpointTest = "http://localhost:8000"

// TestExecuteTool_FabricatedPMToolsHidden verifies that the three fabricated
// PM tools are NOT dispatchable through executeTool. They remain as exported
// methods on ToolRegistry but are unwired from MCP, REST, intent routing,
// and dispatch. Phase 2 will reimplement their bodies and re-wire them.
func TestExecuteTool_FabricatedPMToolsHidden(t *testing.T) {
	s := &MCPServer{}
	for _, name := range []string{
		"calculate_project_metrics",
		"project_risk_assessment",
		"portfolio_dashboard",
	} {
		_, err := s.executeTool(context.Background(), name, []byte(`{}`))
		if err == nil {
			t.Errorf("executeTool(%q) returned nil error; want 'tool not found'", name)
			continue
		}
		if !strings.Contains(err.Error(), "tool not found") {
			t.Errorf("executeTool(%q) error = %v; want contains 'tool not found'", name, err)
		}
	}
}
