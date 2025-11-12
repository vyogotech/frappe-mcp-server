package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/types"
)

// ToolRegistry contains all the MCP tools
type ToolRegistry struct {
	frappeClient *frappe.Client
}

// NewRegistry creates a new tool registry
func NewRegistry(frappeClient *frappe.Client) *ToolRegistry {
	return &ToolRegistry{
		frappeClient: frappeClient,
	}
}

// GetDocument retrieves a single document
func (t *ToolRegistry) GetDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType string `json:"doctype"`
		Name    string `json:"name"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" || params.Name == "" {
		return nil, fmt.Errorf("doctype and name are required")
	}

	doc, err := t.frappeClient.GetDocument(ctx, params.DocType, params.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	result, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Retrieved %s document: %s", params.DocType, params.Name),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// ListDocuments retrieves a list of documents with pagination
func (t *ToolRegistry) ListDocuments(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.SearchRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	// Set defaults
	if params.PageSize == 0 {
		params.PageSize = 20
	}

	docList, err := t.frappeClient.GetDocumentList(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get document list: %w", err)
	}

	result, err := json.Marshal(docList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Retrieved %d %s documents (page %d)", len(docList.Data), params.DocType, params.Page),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// CreateDocument creates a new document
func (t *ToolRegistry) CreateDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.CreateDocumentRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" || params.Data == nil {
		return nil, fmt.Errorf("doctype and data are required")
	}

	doc, err := t.frappeClient.CreateDocument(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	result, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	// Get the document name from the created document
	docName := "unknown"
	if name, ok := doc["name"]; ok {
		if nameStr, ok := name.(string); ok {
			docName = nameStr
		}
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Successfully created %s document: %s", params.DocType, docName),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// UpdateDocument updates an existing document
func (t *ToolRegistry) UpdateDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.UpdateDocumentRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" || params.Name == "" || params.Data == nil {
		return nil, fmt.Errorf("doctype, name, and data are required")
	}

	doc, err := t.frappeClient.UpdateDocument(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	result, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Successfully updated %s document: %s", params.DocType, params.Name),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// DeleteDocument deletes a document
func (t *ToolRegistry) DeleteDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType string `json:"doctype"`
		Name    string `json:"name"`
		Confirm bool   `json:"confirm"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" || params.Name == "" {
		return nil, fmt.Errorf("doctype and name are required")
	}

	if !params.Confirm {
		return &mcp.ToolResponse{
			ID: request.ID,
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("Are you sure you want to delete %s document: %s? Set 'confirm' to true to proceed.", params.DocType, params.Name),
				},
			},
		}, nil
	}

	err := t.frappeClient.DeleteDocument(ctx, params.DocType, params.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to delete document: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Successfully deleted %s document: %s", params.DocType, params.Name),
			},
		},
	}, nil
}

// SearchDocuments performs full-text search across documents
func (t *ToolRegistry) SearchDocuments(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.SearchRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	// Set defaults
	if params.PageSize == 0 {
		params.PageSize = 20
	}

	docList, err := t.frappeClient.SearchDocuments(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	result, err := json.Marshal(docList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	searchDesc := "all documents"
	if params.Search != "" {
		searchDesc = fmt.Sprintf("documents matching '%s'", params.Search)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Found %d %s %s", len(docList.Data), params.DocType, searchDesc),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// GetProjectStatus retrieves comprehensive project status
func (t *ToolRegistry) GetProjectStatus(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		ProjectName string `json:"project_name"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	// Get project document
	project, err := t.frappeClient.GetDocument(ctx, "Project", params.ProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Get associated tasks
	taskReq := types.SearchRequest{
		DocType: "Task",
		Filters: map[string]interface{}{
			"project": params.ProjectName,
		},
		PageSize: 100,
	}

	tasks, err := t.frappeClient.GetDocumentList(ctx, taskReq)
	if err != nil {
		slog.Warn("Failed to get project tasks", "error", err)
	}

	// Build project status
	status := map[string]interface{}{
		"project": project,
		"tasks":   tasks,
		"summary": map[string]interface{}{
			"total_tasks":  len(tasks.Data),
			"project_name": params.ProjectName,
			"last_updated": project["modified"],
		},
	}

	result, err := json.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Project Status for: %s", params.ProjectName),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// AnalyzeProjectTimeline analyzes project timeline and milestones
func (t *ToolRegistry) AnalyzeProjectTimeline(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		ProjectName string `json:"project_name"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	// Get project and tasks
	project, err := t.frappeClient.GetDocument(ctx, "Project", params.ProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	taskReq := types.SearchRequest{
		DocType: "Task",
		Filters: map[string]interface{}{
			"project": params.ProjectName,
		},
		Fields:   []string{"name", "subject", "status", "expected_start_date", "expected_end_date", "progress", "priority"},
		OrderBy:  "expected_start_date",
		PageSize: 100,
	}

	tasks, err := t.frappeClient.GetDocumentList(ctx, taskReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get project tasks: %w", err)
	}

	// Analyze timeline
	analysis := map[string]interface{}{
		"project": project,
		"timeline_analysis": map[string]interface{}{
			"total_tasks":         len(tasks.Data),
			"project_start_date":  project["expected_start_date"],
			"project_end_date":    project["expected_end_date"],
			"project_progress":    project["percent_complete"],
			"critical_path_tasks": []interface{}{}, // TODO: Implement critical path analysis
			"milestones":          []interface{}{}, // TODO: Get milestones
			"timeline_health":     "analyzing...",
		},
		"tasks": tasks.Data,
	}

	result, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Timeline Analysis for Project: %s", params.ProjectName),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// CalculateProjectMetrics calculates various project metrics
func (t *ToolRegistry) CalculateProjectMetrics(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		ProjectName string `json:"project_name"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	// Get project data
	project, err := t.frappeClient.GetDocument(ctx, "Project", params.ProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Calculate basic metrics (simplified)
	metrics := types.ProjectMetrics{
		BurnRate:   0.0,     // TODO: Calculate from timesheets
		Velocity:   0.0,     // TODO: Calculate from task completion
		Efficiency: 0.0,     // TODO: Calculate from time vs estimates
		RiskScore:  0.0,     // TODO: Calculate risk assessment
		Health:     "Green", // TODO: Determine health based on metrics
	}

	// Build response
	response := map[string]interface{}{
		"project": project,
		"metrics": metrics,
		"calculations": map[string]string{
			"burn_rate":  "Total cost / elapsed time",
			"velocity":   "Completed tasks / time period",
			"efficiency": "Actual time / estimated time",
			"risk_score": "Based on delays, budget variance, and resource allocation",
			"health":     "Overall project health indicator",
		},
	}

	result, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Project Metrics for: %s", params.ProjectName),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// GetResourceAllocation analyzes resource allocation across projects
func (t *ToolRegistry) GetResourceAllocation(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	// Get all active projects
	projectReq := types.SearchRequest{
		DocType: "Project",
		Filters: map[string]interface{}{
			"status": "Open",
		},
		Fields:   []string{"name", "project_name", "users", "expected_start_date", "expected_end_date"},
		PageSize: 100,
	}

	projects, err := t.frappeClient.GetDocumentList(ctx, projectReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	// Analyze resource allocation
	analysis := map[string]interface{}{
		"total_active_projects": len(projects.Data),
		"projects":              projects.Data,
		"resource_summary": map[string]interface{}{
			"analysis_note":  "Resource allocation analysis based on active projects",
			"total_projects": len(projects.Data),
		},
	}

	result, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: "Resource Allocation Analysis",
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// ProjectRiskAssessment performs risk assessment for a project
func (t *ToolRegistry) ProjectRiskAssessment(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		ProjectName string `json:"project_name"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	// Get project data
	project, err := t.frappeClient.GetDocument(ctx, "Project", params.ProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Simplified risk assessment
	riskFactors := map[string]interface{}{
		"schedule_risk": "Low", // TODO: Calculate based on timeline
		"budget_risk":   "Low", // TODO: Calculate based on budget variance
		"resource_risk": "Low", // TODO: Calculate based on resource availability
		"scope_risk":    "Low", // TODO: Calculate based on scope changes
		"overall_risk":  "Low",
		"recommendations": []string{
			"Monitor project timeline closely",
			"Regular stakeholder communication",
			"Track budget variance weekly",
		},
	}

	response := map[string]interface{}{
		"project":         project,
		"risk_assessment": riskFactors,
	}

	result, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Risk Assessment for Project: %s", params.ProjectName),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// GenerateProjectReport generates a comprehensive project report
func (t *ToolRegistry) GenerateProjectReport(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		ProjectName string `json:"project_name"`
		ReportType  string `json:"report_type"` // "summary", "detailed", "executive"
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	if params.ReportType == "" {
		params.ReportType = "summary"
	}

	// Get project data
	project, err := t.frappeClient.GetDocument(ctx, "Project", params.ProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Generate report based on type
	var report map[string]interface{}

	switch params.ReportType {
	case "executive":
		report = map[string]interface{}{
			"report_type": "Executive Summary",
			"project":     project,
			"key_metrics": map[string]interface{}{
				"status":   project["status"],
				"progress": project["percent_complete"],
			},
			"executive_summary": fmt.Sprintf("Project %s is currently %s with %v%% completion.",
				params.ProjectName, project["status"], project["percent_complete"]),
		}
	case "detailed":
		report = map[string]interface{}{
			"report_type": "Detailed Report",
			"project":     project,
			"sections": []string{
				"Project Overview",
				"Task Breakdown",
				"Resource Allocation",
				"Timeline Analysis",
				"Budget Analysis",
				"Risk Assessment",
			},
		}
	default: // summary
		report = map[string]interface{}{
			"report_type": "Summary Report",
			"project":     project,
			"summary": map[string]interface{}{
				"name":     project["name"],
				"status":   project["status"],
				"progress": project["percent_complete"],
			},
		}
	}

	result, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("%s Report for Project: %s", params.ReportType, params.ProjectName),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// PortfolioDashboard provides portfolio-level insights
func (t *ToolRegistry) PortfolioDashboard(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	// Get all projects
	projectReq := types.SearchRequest{
		DocType:  "Project",
		Fields:   []string{"name", "project_name", "status", "percent_complete", "priority"},
		OrderBy:  "creation desc",
		PageSize: 50,
	}

	projects, err := t.frappeClient.GetDocumentList(ctx, projectReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	// Calculate portfolio metrics
	dashboard := map[string]interface{}{
		"portfolio_overview": map[string]interface{}{
			"total_projects":     len(projects.Data),
			"active_projects":    0, // TODO: Count by status
			"completed_projects": 0,
			"overdue_projects":   0,
		},
		"projects": projects.Data,
		"kpis": map[string]interface{}{
			"average_completion": 0.0, // TODO: Calculate
			"on_time_delivery":   0.0, // TODO: Calculate
			"budget_utilization": 0.0, // TODO: Calculate
		},
	}

	result, err := json.Marshal(dashboard)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: "Portfolio Dashboard",
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// ResourceUtilizationAnalysis analyzes resource utilization
func (t *ToolRegistry) ResourceUtilizationAnalysis(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	// Get all employees
	empReq := types.SearchRequest{
		DocType:  "Employee",
		Fields:   []string{"name", "employee_name", "status", "department"},
		PageSize: 100,
	}

	employees, err := t.frappeClient.GetDocumentList(ctx, empReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get employees: %w", err)
	}

	// Analyze utilization
	analysis := map[string]interface{}{
		"resource_analysis": map[string]interface{}{
			"total_resources":  len(employees.Data),
			"utilization_note": "Resource utilization analysis based on current assignments",
		},
		"employees": employees.Data,
	}

	result, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: "Resource Utilization Analysis",
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// BudgetVarianceAnalysis analyzes budget variance across projects
func (t *ToolRegistry) BudgetVarianceAnalysis(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	// Get projects with budget information
	projectReq := types.SearchRequest{
		DocType:  "Project",
		Fields:   []string{"name", "project_name", "total_budget", "actual_cost", "status"},
		PageSize: 50,
	}

	projects, err := t.frappeClient.GetDocumentList(ctx, projectReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	// Analyze budget variance
	analysis := map[string]interface{}{
		"budget_analysis": map[string]interface{}{
			"total_projects_analyzed": len(projects.Data),
			"analysis_note":           "Budget variance analysis for all projects",
		},
		"projects": projects.Data,
		"summary": map[string]interface{}{
			"total_budget":     0.0, // TODO: Calculate sum
			"total_actual":     0.0, // TODO: Calculate sum
			"overall_variance": 0.0, // TODO: Calculate
		},
	}

	result, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: "Budget Variance Analysis",
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// AnalyzeDocument is a generic document analyzer that works with ANY doctype
// It fetches the document and optionally related documents, letting AI handle analysis
func (t *ToolRegistry) AnalyzeDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType        string   `json:"doctype"`
		Name           string   `json:"name"`
		IncludeRelated bool     `json:"include_related,omitempty"`
		Fields         []string `json:"fields,omitempty"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" || params.Name == "" {
		return nil, fmt.Errorf("doctype and name are required")
	}

	// Get the main document - completely generic!
	doc, err := t.frappeClient.GetDocument(ctx, params.DocType, params.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	response := map[string]interface{}{
		"doctype":  params.DocType,
		"name":     params.Name,
		"document": doc,
	}

	// Optionally include related documents (generic approach)
	if params.IncludeRelated {
		// Look for common relationship fields
		relatedDocs := t.fetchRelatedDocuments(ctx, params.DocType, doc)
		if len(relatedDocs) > 0 {
			response["related_documents"] = relatedDocs
		}
	}

	result, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Document Analysis for %s: %s", params.DocType, params.Name),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

// fetchRelatedDocuments generically fetches related documents based on common patterns
func (t *ToolRegistry) fetchRelatedDocuments(ctx context.Context, doctype string, doc types.Document) map[string]interface{} {
	related := make(map[string]interface{})

	// Generic approach: look for common relationship patterns
	// Check for 'items' child table (common in transactions)
	if items, ok := doc["items"]; ok {
		related["items"] = items
	}

	// Check for 'tasks' child table (common in projects)
	if tasks, ok := doc["tasks"]; ok {
		related["tasks"] = tasks
	}

	// Extract and fetch linked documents based on common field patterns
	// This is generic - works for any doctype!
	linkedFields := []string{"customer", "supplier", "project", "task", "parent_project", "sales_order", "purchase_order"}
	
	for _, field := range linkedFields {
		if value, ok := doc[field]; ok {
			if strValue, ok := value.(string); ok && strValue != "" {
				// Infer doctype from field name (generic heuristic)
				linkedDocType := inferDocTypeFromField(field)
				if linkedDocType != "" {
					// Fetch the linked document
					linkedDoc, err := t.frappeClient.GetDocument(ctx, linkedDocType, strValue)
					if err == nil {
						related[field] = linkedDoc
					}
				}
			}
		}
	}

	return related
}

// inferDocTypeFromField infers doctype from field name using generic patterns
func inferDocTypeFromField(fieldName string) string {
	// Generic mapping based on common ERPNext naming conventions
	mapping := map[string]string{
		"customer":       "Customer",
		"supplier":       "Supplier",
		"project":        "Project",
		"task":           "Task",
		"sales_order":    "Sales Order",
		"purchase_order": "Purchase Order",
		"item":           "Item",
		"employee":       "Employee",
	}
	
	if doctype, ok := mapping[fieldName]; ok {
		return doctype
	}
	
	// Fallback: capitalize field name (often works in ERPNext)
	// e.g., "warehouse" -> "Warehouse"
	if len(fieldName) > 0 {
		return strings.ToUpper(string(fieldName[0])) + fieldName[1:]
	}
	
	return ""
}
