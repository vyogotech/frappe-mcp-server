// Package neo4j provides a lightweight Neo4j bolt client for FrappeForge
// graph queries. It is optional: when BoltURL is empty the package returns
// a nil *Client, and every Query call on a nil receiver returns a clear
// "not configured" error so callers can degrade gracefully.
package neo4j

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Client wraps a Neo4j driver. A nil *Client is safe to call Query on.
type Client struct {
	driver neo4j.DriverWithContext
}

// NewClient creates a new Client. Returns (nil, nil) when boltURL is empty
// so callers can opt-out of graph features without errors.
func NewClient(boltURL, username, password string) (*Client, error) {
	if boltURL == "" {
		return nil, nil
	}
	driver, err := neo4j.NewDriverWithContext(
		boltURL,
		neo4j.BasicAuth(username, password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("neo4j: failed to create driver: %w", err)
	}
	return &Client{driver: driver}, nil
}

// Close closes the underlying driver. Safe to call on nil.
func (c *Client) Close() {
	if c == nil || c.driver == nil {
		return
	}
	_ = c.driver.Close(context.Background())
}

// Query executes a read-only Cypher query and returns all records as a
// slice of map[string]any. Safe to call on a nil receiver.
func (c *Client) Query(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error) {
	if c == nil || c.driver == nil {
		return nil, fmt.Errorf("FrappeForge graph not configured or unavailable")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer func() { _ = session.Close(ctx) }()

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("neo4j: query failed: %w", err)
	}

	var rows []map[string]any
	for result.Next(ctx) {
		record := result.Record()
		row := make(map[string]any, len(record.Keys))
		for _, key := range record.Keys {
			val, _ := record.Get(key)
			row[key] = val
		}
		rows = append(rows, row)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("neo4j: result iteration error: %w", err)
	}
	return rows, nil
}
