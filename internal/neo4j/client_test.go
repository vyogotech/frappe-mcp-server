package neo4j

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClientNilWhenNoURL verifies a nil client is returned when no BoltURL is configured.
func TestNewClientNilWhenNoURL(t *testing.T) {
	client, err := NewClient("", "", "")
	require.NoError(t, err)
	assert.Nil(t, client, "expected nil client when BoltURL is empty")
}

// TestNewClientReturnsClientWithURL verifies that a client is returned when a URL is provided.
// Note: this does NOT dial – the driver is lazy.
func TestNewClientReturnsClientWithURL(t *testing.T) {
	client, err := NewClient("bolt://localhost:7687", "neo4j", "test")
	require.NoError(t, err)
	assert.NotNil(t, client)
	client.Close()
}

// TestQueryOnNilClientReturnsError verifies the nil guard in Query.
func TestQueryOnNilClientReturnsError(t *testing.T) {
	var c *Client
	rows, err := c.Query(context.Background(), "RETURN 1", nil)
	assert.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "not configured")
}

// TestQueryConnectionRefused verifies that a connection failure returns an error
// without panicking. Uses a port that should be unreachable.
func TestQueryConnectionRefused(t *testing.T) {
	client, err := NewClient("bolt://localhost:19999", "neo4j", "test")
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	rows, err := client.Query(context.Background(), "RETURN 1 AS n", nil)
	assert.Error(t, err, "expected connection error")
	assert.Nil(t, rows)
}
