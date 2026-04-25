package mcp

import (
	"context"
	"encoding/json"
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
	assert.NotNil(t, server.sdkServer)
}

func TestRegisterTool(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	testHandler := func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{
			ID: request.ID,
			Content: []Content{
				{Type: "text", Text: "test response"},
			},
		}, nil
	}

	server.RegisterTool("test_tool", testHandler)

	assert.Contains(t, server.toolNames, "test_tool")
}

func TestRegisterResource(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	server.RegisterResource("test://resource", "Test Resource")

	assert.Contains(t, server.resourceURIs, "test://resource")
}

func TestExecuteToolRequest(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

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
	assert.Equal(t, 500, response.Error.Code)
}

func TestExecuteToolRequestError(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

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

func TestToolRequestGeneration(t *testing.T) {
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

	// Register a tool that checks context cancellation.
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

	// Create a context that will be cancelled before the tool finishes.
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
