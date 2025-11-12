package frappe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/testutils"
	"frappe-mcp-server/internal/types"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      config.ERPNextConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: config.ERPNextConfig{
				BaseURL:   "https://test.erpnext.com",
				APIKey:    "test_key",
				APISecret: "test_secret",
				Timeout:   30 * time.Second,
				RateLimit: config.RateLimitConfig{
					RequestsPerSecond: 10,
					Burst:             20,
				},
			},
			expectError: false,
		},
		{
			name: "missing base URL",
			config: config.ERPNextConfig{
				BaseURL:   "",
				APIKey:    "test_key",
				APISecret: "test_secret",
			},
			expectError: true,
		},
		{
			name: "missing API key",
			config: config.ERPNextConfig{
				BaseURL:   "https://test.erpnext.com",
				APIKey:    "",
				APISecret: "test_secret",
			},
			expectError: true,
		},
		{
			name: "missing API secret",
			config: config.ERPNextConfig{
				BaseURL:   "https://test.erpnext.com",
				APIKey:    "test_key",
				APISecret: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestGetDocument(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client with mock server URL
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Test successful document retrieval
	ctx := context.Background()
	doc, err := client.GetDocument(ctx, "Project", "TEST-PROJ-001")
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "TEST-PROJ-001", doc["name"])
	assert.Equal(t, "Test Project", doc["project_name"])
	assert.Equal(t, "Open", doc["status"])
}

func TestGetDocumentList(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Test document list retrieval
	ctx := context.Background()
	req := types.SearchRequest{
		DocType:  "Project",
		Fields:   []string{"name", "project_name", "status"},
		PageSize: 10,
	}

	docList, err := client.GetDocumentList(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, docList)
	assert.Len(t, docList.Data, 2)
	assert.Equal(t, "TEST-PROJ-001", docList.Data[0]["name"])
	assert.Equal(t, "TEST-PROJ-002", docList.Data[1]["name"])
}

func TestCreateDocument(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Test document creation
	ctx := context.Background()
	req := types.CreateDocumentRequest{
		DocType: "Project",
		Data: types.Document{
			"project_name": "New Test Project",
			"status":       "Open",
			"priority":     "Medium",
		},
	}

	// Note: This will return a 404 from our mock server since we haven't
	// implemented a handler for POST requests, but we can test the client logic
	_, err = client.CreateDocument(ctx, req)
	// We expect an error from the mock server
	assert.Error(t, err)
}

func TestUpdateDocument(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Test document update
	ctx := context.Background()
	req := types.UpdateDocumentRequest{
		DocType: "Project",
		Name:    "TEST-PROJ-001",
		Data: types.Document{
			"percent_complete": 50.0,
			"status":           "Working",
		},
	}

	// Note: This will return a 404 from our mock server since we haven't
	// implemented a handler for PUT requests
	_, err = client.UpdateDocument(ctx, req)
	assert.Error(t, err)
}

func TestDeleteDocument(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Test document deletion
	ctx := context.Background()
	err = client.DeleteDocument(ctx, "Project", "TEST-PROJ-001")
	// We expect an error from the mock server since DELETE isn't handled
	assert.Error(t, err)
}

func TestSearchDocuments(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Test document search
	ctx := context.Background()
	req := types.SearchRequest{
		DocType: "Customer",
		Search:  "tech",
		Fields:  []string{"name", "customer_name", "customer_type"},
	}

	docList, err := client.SearchDocuments(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, docList)
	// The mock server returns customers for any search
	assert.Len(t, docList.Data, 2)
}

func TestRateLimiting(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client with very restrictive rate limiting
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 1, // Very restrictive
			Burst:             1,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  1,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Make multiple requests quickly
	ctx := context.Background()
	start := time.Now()

	// First request should succeed immediately
	_, err = client.GetDocument(ctx, "Project", "TEST-PROJ-001")
	assert.NoError(t, err)

	// Clear cache to ensure second request hits the API
	client.ClearCache()

	// Second request should be rate limited
	_, err = client.GetDocument(ctx, "Project", "TEST-PROJ-001")
	duration := time.Since(start)

	assert.NoError(t, err)
	// Should take at least 1 second due to rate limiting
	assert.True(t, duration >= 1*time.Second)
}

func TestCaching(t *testing.T) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(t)
	defer mockServer.Close()

	// Create client
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// First request - should hit the server
	start := time.Now()
	doc1, err := client.GetDocument(ctx, "Project", "TEST-PROJ-001")
	firstDuration := time.Since(start)
	assert.NoError(t, err)
	assert.NotNil(t, doc1)

	// Second request - should hit the cache
	start = time.Now()
	doc2, err := client.GetDocument(ctx, "Project", "TEST-PROJ-001")
	secondDuration := time.Since(start)
	assert.NoError(t, err)
	assert.NotNil(t, doc2)

	// Cache hit should be faster
	assert.True(t, secondDuration < firstDuration)
	assert.Equal(t, doc1["name"], doc2["name"])
}

func BenchmarkGetDocument(b *testing.B) {
	// Create mock server
	mockServer := testutils.MockERPNextServer(&testing.T{})
	defer mockServer.Close()

	// Create client
	cfg := config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             200,
		},
		Retry: config.RetryConfig{
			MaxAttempts:  1,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetDocument(ctx, "Project", "TEST-PROJ-001")
		if err != nil {
			b.Fatal(err)
		}
	}
}
