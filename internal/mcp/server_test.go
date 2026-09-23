package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
