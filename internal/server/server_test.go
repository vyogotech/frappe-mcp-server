package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/auth"
	authstrategies "frappe-mcp-server/internal/auth/strategies"
)

// The docker HEALTHCHECK sends no token, so /health and /api/v1/health must skip auth and nothing else may.
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
		{"/metrics", http.StatusUnauthorized},
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

// mockOAuthEndpointTest is a constant because gosec G101 flags TokenInfoURL string literals as hardcoded credentials.
const mockOAuthEndpointTest = "http://localhost:8000"

// These three tools return placeholder numbers, so the REST dispatcher must not find them.
func TestExecuteTool_FabricatedPMToolsHidden(t *testing.T) {
	s := &MCPServer{}
	for _, name := range []string{
		"calculate_project_metrics",
		"project_risk_assessment",
		"portfolio_dashboard",
	} {
		if _, ok := s.tool(name); ok {
			t.Errorf("tool(%q) was found; want not found", name)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/"+name, strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		s.handleToolCall(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("POST /api/v1/tools/%s: got status %d, want %d", name, w.Code, http.StatusNotFound)
		}
	}
}
