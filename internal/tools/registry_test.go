package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/testutils"
)

func createTestClient(t *testing.T) *frappe.Client {
	mockServer := testutils.MockERPNextServer(t)
	t.Cleanup(func() { mockServer.Close() })

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

	client, err := frappe.NewClient(cfg)
	require.NoError(t, err)
	return client
}

func TestNewRegistry(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	assert.NotNil(t, registry)
}

func TestGetDocument(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	// Test valid request
	params := map[string]interface{}{
		"doctype": "Project",
		"name":    "TEST-PROJ-001",
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "get_document",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.GetDocument(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-1", response.ID)
	assert.Len(t, response.Content, 2)
	assert.Equal(t, "text", response.Content[0].Type)
	assert.Contains(t, response.Content[0].Text, "Retrieved Project document: TEST-PROJ-001")
}

func TestGetDocumentInvalidParams(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	// Test invalid JSON
	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "get_document",
		Params: []byte(`{"invalid": json}`),
	}

	ctx := context.Background()
	response, err := registry.GetDocument(ctx, request)

	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestGetDocumentMissingParams(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	// Test missing doctype
	params := map[string]interface{}{
		"name": "TEST-PROJ-001",
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "get_document",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.GetDocument(ctx, request)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "doctype and name are required")
}

func TestListDocuments(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"doctype": "Project",
		"fields":  []string{"name", "project_name", "status"},
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "list_documents",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.ListDocuments(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-1", response.ID)
	assert.Len(t, response.Content, 2)
	assert.Contains(t, response.Content[0].Text, "Retrieved 2 Project documents")
}

func TestCreateDocument(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"doctype": "Project",
		"data": map[string]interface{}{
			"project_name": "New Test Project",
			"status":       "Open",
		},
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "create_document",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.CreateDocument(ctx, request)

	// We expect an error since our mock server doesn't handle POST requests
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestUpdateDocument(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"doctype": "Project",
		"name":    "TEST-PROJ-001",
		"data": map[string]interface{}{
			"percent_complete": 75.0,
		},
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "update_document",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.UpdateDocument(ctx, request)

	// We expect an error since our mock server doesn't handle PUT requests
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestDeleteDocument(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	// Test without confirmation
	params := map[string]interface{}{
		"doctype": "Project",
		"name":    "TEST-PROJ-001",
		"confirm": false,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "delete_document",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.DeleteDocument(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Are you sure you want to delete")

	// Test with confirmation
	params["confirm"] = true
	paramsJSON, err = json.Marshal(params)
	require.NoError(t, err)

	request.Params = paramsJSON
	response, err = registry.DeleteDocument(ctx, request)

	// We expect an error since our mock server doesn't handle DELETE requests
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestSearchDocuments(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"doctype": "Customer",
		"search":  "tech",
		"fields":  []string{"name", "customer_name"},
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "search_documents",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.SearchDocuments(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Found 2 Customer documents matching 'tech'")
}

func TestGetProjectStatus(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"project_name": "TEST-PROJ-001",
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "get_project_status",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.GetProjectStatus(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Project Status for: TEST-PROJ-001")
}

func TestAnalyzeProjectTimeline(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"project_name": "TEST-PROJ-001",
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "analyze_project_timeline",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.AnalyzeProjectTimeline(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Timeline Analysis for Project: TEST-PROJ-001")
}

func TestCalculateProjectMetrics(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"project_name": "TEST-PROJ-001",
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "calculate_project_metrics",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.CalculateProjectMetrics(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Project Metrics for: TEST-PROJ-001")
}

func TestGetResourceAllocation(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "get_resource_allocation",
		Params: []byte(`{}`),
	}

	ctx := context.Background()
	response, err := registry.GetResourceAllocation(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Resource Allocation Analysis")
}

func TestProjectRiskAssessment(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"project_name": "TEST-PROJ-001",
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "project_risk_assessment",
		Params: paramsJSON,
	}

	ctx := context.Background()
	response, err := registry.ProjectRiskAssessment(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Risk Assessment for Project: TEST-PROJ-001")
}

func TestGenerateProjectReport(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	tests := []struct {
		name       string
		reportType string
		expected   string
	}{
		{
			name:       "executive report",
			reportType: "executive",
			expected:   "executive Report for Project: TEST-PROJ-001",
		},
		{
			name:       "detailed report",
			reportType: "detailed",
			expected:   "detailed Report for Project: TEST-PROJ-001",
		},
		{
			name:       "summary report (default)",
			reportType: "",
			expected:   "summary Report for Project: TEST-PROJ-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{
				"project_name": "TEST-PROJ-001",
			}
			if tt.reportType != "" {
				params["report_type"] = tt.reportType
			}

			paramsJSON, err := json.Marshal(params)
			require.NoError(t, err)

			request := mcp.ToolRequest{
				ID:     "test-1",
				Tool:   "generate_project_report",
				Params: paramsJSON,
			}

			ctx := context.Background()
			response, err := registry.GenerateProjectReport(ctx, request)

			assert.NoError(t, err)
			assert.NotNil(t, response)
			assert.Contains(t, response.Content[0].Text, tt.expected)
		})
	}
}

func TestPortfolioDashboard(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "portfolio_dashboard",
		Params: []byte(`{}`),
	}

	ctx := context.Background()
	response, err := registry.PortfolioDashboard(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Portfolio Dashboard")
}

func TestResourceUtilizationAnalysis(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "resource_utilization_analysis",
		Params: []byte(`{}`),
	}

	ctx := context.Background()
	response, err := registry.ResourceUtilizationAnalysis(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Resource Utilization Analysis")
}

func TestBudgetVarianceAnalysis(t *testing.T) {
	client := createTestClient(t)
	registry := NewRegistry(client)

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "budget_variance_analysis",
		Params: []byte(`{}`),
	}

	ctx := context.Background()
	response, err := registry.BudgetVarianceAnalysis(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, response.Content[0].Text, "Budget Variance Analysis")
}

func BenchmarkGetDocument(b *testing.B) {
	client := createTestClient(&testing.T{})
	registry := NewRegistry(client)

	params := map[string]interface{}{
		"doctype": "Project",
		"name":    "TEST-PROJ-001",
	}
	paramsJSON, _ := json.Marshal(params)

	request := mcp.ToolRequest{
		ID:     "bench-1",
		Tool:   "get_document",
		Params: paramsJSON,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := registry.GetDocument(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}
