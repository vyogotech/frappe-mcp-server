package server

import (
	"context"
	"strings"
	"testing"

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
