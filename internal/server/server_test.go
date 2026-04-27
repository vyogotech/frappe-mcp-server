package server

import (
	"context"
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
