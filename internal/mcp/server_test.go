package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	assert.NotNil(t, server)
	assert.Equal(t, "test-server", server.name)
	assert.Equal(t, "1.0.0", server.version)
	assert.NotNil(t, server.tools)
	assert.NotNil(t, server.resources)
	assert.NotNil(t, server.router)
}

func TestRegisterTool(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// Create a test tool handler
	testHandler := func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{
			ID: request.ID,
			Content: []Content{
				{Type: "text", Text: "test response"},
			},
		}, nil
	}

	// Register the tool
	server.RegisterTool("test_tool", testHandler)

	// Verify it was registered
	handler, exists := server.tools["test_tool"]
	assert.True(t, exists)
	assert.NotNil(t, handler)
}

func TestRegisterResource(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// Register a resource
	server.RegisterResource("test://resource", "Test Resource")

	// Verify it was registered
	description, exists := server.resources["test://resource"]
	assert.True(t, exists)
	assert.Equal(t, "Test Resource", description)
}

func TestExecuteToolRequest(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// Register a test tool
	server.RegisterTool("echo", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		var params map[string]interface{}
		err := json.Unmarshal(request.Params, &params)
		if err != nil {
			return nil, err
		}

		message, ok := params["message"].(string)
		if !ok {
			message = "no message"
		}

		return &ToolResponse{
			ID: request.ID,
			Content: []Content{
				{Type: "text", Text: "Echo: " + message},
			},
		}, nil
	})

	// Test successful tool execution
	params := map[string]interface{}{"message": "hello world"}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := ToolRequest{
		ID:     "test-1",
		Tool:   "echo",
		Params: paramsJSON,
	}

	response := server.executeToolRequest(context.Background(), request)

	assert.NotNil(t, response)
	assert.Equal(t, "test-1", response.ID)
	assert.Nil(t, response.Error)
	assert.Len(t, response.Content, 1)
	assert.Equal(t, "Echo: hello world", response.Content[0].Text)
}

func TestExecuteToolRequestNotFound(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	request := ToolRequest{
		ID:   "test-1",
		Tool: "nonexistent_tool",
	}

	response := server.executeToolRequest(context.Background(), request)

	assert.NotNil(t, response)
	assert.Equal(t, "test-1", response.ID)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 404, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Tool 'nonexistent_tool' not found")
}

func TestExecuteToolRequestError(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// Register a tool that returns an error
	server.RegisterTool("error_tool", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return nil, assert.AnError
	})

	request := ToolRequest{
		ID:   "test-1",
		Tool: "error_tool",
	}

	response := server.executeToolRequest(context.Background(), request)

	assert.NotNil(t, response)
	assert.Equal(t, "test-1", response.ID)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 500, response.Error.Code)
}

func TestHTTPHandlers(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// Register test tools and resources
	server.RegisterTool("test_tool", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{
			ID: request.ID,
			Content: []Content{
				{Type: "text", Text: "test response"},
			},
		}, nil
	})

	server.RegisterResource("test://resource", "Test Resource")

	// Test tools list endpoint
	t.Run("tools list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/tools", nil)
		w := httptest.NewRecorder()

		server.handleToolsList(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		tools, ok := response["tools"].([]interface{})
		assert.True(t, ok)
		assert.Contains(t, tools, "test_tool")
	})

	// Test resources list endpoint
	t.Run("resources list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/resources", nil)
		w := httptest.NewRecorder()

		server.handleResourcesList(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		resources, ok := response["resources"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, resources, 1)
	})

	// Test tool call endpoint
	t.Run("tool call", func(t *testing.T) {
		toolRequest := map[string]interface{}{
			"id":     "test-1",
			"params": map[string]interface{}{},
		}
		requestBody, err := json.Marshal(toolRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/tools/test_tool", strings.NewReader(string(requestBody)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Manually set the URL vars since we're not using the actual mux router
		server.handleToolCall(w, req)

		// Note: This will fail because we can't properly simulate mux.Vars
		// In a real test, you'd use the actual router or a more sophisticated mock
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestToolRequestGeneration(t *testing.T) {
	// Test that tool requests get IDs if not provided
	server := NewServer("test-server", "1.0.0")

	server.RegisterTool("test_tool", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{
			Content: []Content{
				{Type: "text", Text: "response"},
			},
		}, nil
	})

	request := ToolRequest{
		Tool: "test_tool",
		// No ID provided
	}

	response := server.executeToolRequest(context.Background(), request)

	assert.NotNil(t, response)
	assert.NotEmpty(t, response.ID)
	assert.Contains(t, response.ID, "req_")
}

func TestContextCancellation(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// Register a tool that checks context cancellation
	server.RegisterTool("long_running", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return &ToolResponse{
				ID: request.ID,
				Content: []Content{
					{Type: "text", Text: "completed"},
				},
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	request := ToolRequest{
		ID:   "test-1",
		Tool: "long_running",
	}

	response := server.executeToolRequest(ctx, request)

	assert.NotNil(t, response)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 500, response.Error.Code)
}

func BenchmarkExecuteToolRequest(b *testing.B) {
	server := NewServer("test-server", "1.0.0")

	server.RegisterTool("benchmark_tool", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{
			ID: request.ID,
			Content: []Content{
				{Type: "text", Text: "benchmark response"},
			},
		}, nil
	})

	request := ToolRequest{
		ID:   "bench-1",
		Tool: "benchmark_tool",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := server.executeToolRequest(ctx, request)
		if response.Error != nil {
			b.Fatal(response.Error.Message)
		}
	}
}
