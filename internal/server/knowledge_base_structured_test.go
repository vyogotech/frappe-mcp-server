package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
)

// kbServer is an MCP server whose knowledge base is a fake rag app answering with the given passages, or with
// `"message": null` when passages is nil.
func kbServer(t *testing.T, passages []map[string]any) *MCPServer {
	t.Helper()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/method/rag.search.search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"message": passages}))
	}))
	t.Cleanup(site.Close)

	erp := config.ERPNextConfig{BaseURL: site.URL, APIKey: "k", APISecret: "s",
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100},
		Retry:     config.RetryConfig{MaxAttempts: 1}}
	client, err := frappe.NewClient(erp)
	require.NoError(t, err)
	s, err := NewMCPServer(&config.Config{ERPNext: erp, Tools: config.ToolsConfig{KnowledgeBase: true}}, client)
	require.NoError(t, err)
	return s
}

// rpc sends one JSON-RPC request to /mcp and returns its result object.
func rpc(t *testing.T, s *MCPServer, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Nil(t, resp.Error, rec.Body.String())
	return resp.Result
}

func callKB(t *testing.T, s *MCPServer, query string) map[string]any {
	t.Helper()
	return rpc(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_knowledge_base",`+
		`"arguments":{"query":`+strconv.Quote(query)+`}}}`)
}

// blockTexts is what the agent's ToolRegistry.ainvoke makes of a result: the text blocks joined by a newline.
func blockTexts(t *testing.T, result map[string]any) string {
	t.Helper()
	blocks, ok := result["content"].([]any)
	require.True(t, ok, "result has no content blocks: %v", result)
	texts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		block, ok := b.(map[string]any)
		require.True(t, ok)
		texts = append(texts, block["text"].(string))
	}
	return strings.Join(texts, "\n")
}

func TestKnowledgeBaseDeclaresItsOutputSchema(t *testing.T) {
	result := rpc(t, kbServer(t, nil), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	var schema map[string]any
	for _, tool := range result["tools"].([]any) {
		if tool.(map[string]any)["name"] == "search_knowledge_base" {
			schema, _ = tool.(map[string]any)["outputSchema"].(map[string]any)
		}
	}
	require.NotNil(t, schema, "search_knowledge_base publishes no outputSchema, so a client cannot know what "+
		"structuredContent holds")
	assert.Equal(t, "object", schema["type"], "an output schema must have type object")
	assert.Equal(t, []any{"passages"}, schema["required"])
	passages := schema["properties"].(map[string]any)["passages"].(map[string]any)
	assert.Equal(t, "array", passages["type"])
	assert.Contains(t, passages["items"].(map[string]any)["properties"], "content")
}

func TestKnowledgeBaseReturnsItsPassagesAsStructuredContent(t *testing.T) {
	passage := map[string]any{"file": "abc", "seq": 0.0, "distance": 0.14,
		"content": "Mileage is reimbursed at 88 cents per kilometre."}
	result := callKB(t, kbServer(t, []map[string]any{passage}), "what is the mileage rate")

	structured, ok := result["structuredContent"].(map[string]any)
	require.True(t, ok, "the tool returned no structuredContent: %v", result)
	rows, ok := structured["passages"].([]any)
	require.True(t, ok, "structuredContent has no passages array: %v", structured)
	require.Len(t, rows, 1)
	assert.Equal(t, passage, rows[0], "a passage must cross whole, not as a sentence about it")

	// The text blocks are the same passages for a client that ignores structure, and today's agent finds them
	// there with `^\[` (frappe-ai-agent src/ai_agent/agent/loop.py:342).
	text := blockTexts(t, result)
	assert.Contains(t, text, `Found 1 passage(s) for "what is the mileage rate"`)
	match := regexp.MustCompile(`(?m)^\[`).FindStringIndex(text)
	require.NotNil(t, match, "no JSON array at a line start; the agent would drop the sources: %q", text)
	var fromText []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text[match[0]:]), &fromText))
	assert.Equal(t, []map[string]any{passage}, fromText)
}

func TestKnowledgeBaseWithNothingFoundIsAnEmptyArray(t *testing.T) {
	result := callKB(t, kbServer(t, nil), "anything")

	structured, ok := result["structuredContent"].(map[string]any)
	require.True(t, ok, "the tool returned no structuredContent: %v", result)
	assert.Equal(t, []any{}, structured["passages"], "no passages is an empty array, which the schema allows")
	assert.Contains(t, blockTexts(t, result), "\n[]", "a `null` here is not an array and the agent drops it")
}
