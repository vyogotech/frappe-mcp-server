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

type ToolRegistry struct {
	frappeClient *frappe.Client
	// ConfirmationRedeemMethod is the Frappe method a write's one-time token is redeemed against; empty means the default.
	ConfirmationRedeemMethod string
}

func NewRegistry(frappeClient *frappe.Client) *ToolRegistry {
	return &ToolRegistry{
		frappeClient: frappeClient,
	}
}

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

func (t *ToolRegistry) CreateDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.CreateDocumentRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" || params.Data == nil {
		return nil, fmt.Errorf("doctype and data are required")
	}

	if err := t.requireConfirmation(ctx, "create_document", params.DocType, ""); err != nil {
		return nil, err
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

func (t *ToolRegistry) UpdateDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.UpdateDocumentRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" || params.Name == "" || params.Data == nil {
		return nil, fmt.Errorf("doctype, name, and data are required")
	}

	if err := t.requireConfirmation(ctx, "update_document", params.DocType, params.Name); err != nil {
		return nil, err
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

func (t *ToolRegistry) DeleteDocument(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
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

	// never a `confirm` argument again: the caller who asks for the delete cannot be the one who confirms it
	if err := t.requireConfirmation(ctx, "delete_document", params.DocType, params.Name); err != nil {
		return nil, err
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
		// Task dates are exp_start_date/exp_end_date; the Project fields below are the expected_* ones
		Fields:   []string{"name", "subject", "status", "exp_start_date", "exp_end_date", "progress", "priority"},
		OrderBy:  "exp_start_date",
		PageSize: 100,
	}

	tasks, err := t.frappeClient.GetDocumentList(ctx, taskReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get project tasks: %w", err)
	}

	// Every value here is read from the project or counted from its tasks; a field nothing computes is not reported.
	analysis := map[string]interface{}{
		"project": project,
		"timeline_analysis": map[string]interface{}{
			"total_tasks":        len(tasks.Data),
			"project_start_date": project["expected_start_date"],
			"project_end_date":   project["expected_end_date"],
			"project_progress":   project["percent_complete"],
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

	// Missing or non-numeric amounts are skipped, so a partial dataset still sums instead of failing.
	var totalBudget, totalActual float64
	for _, p := range projects.Data {
		if v, ok := p["total_budget"].(float64); ok {
			totalBudget += v
		}
		if v, ok := p["actual_cost"].(float64); ok {
			totalActual += v
		}
	}

	analysis := map[string]interface{}{
		"budget_analysis": map[string]interface{}{
			"total_projects_analyzed": len(projects.Data),
			"analysis_note":           "Budget variance analysis for all projects",
		},
		"projects": projects.Data,
		"summary": map[string]interface{}{
			"total_budget":     totalBudget,
			"total_actual":     totalActual,
			"overall_variance": totalBudget - totalActual,
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

// AnalyzeDocument only fetches the document and, with include_related, its linked records; the model analyses them.
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
					if err != nil {
						// a refused or failed lookup is not the same as a field with nothing linked
						related[field] = map[string]interface{}{"error": err.Error()}
						continue
					}
					related[field] = linkedDoc
				}
			}
		}
	}

	return related
}

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

func (t *ToolRegistry) AggregateDocuments(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.AggregationRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	// get_list is paginated, so len(results) under-reports. A grouped count
	// still needs get_list: it returns one row per group.
	if strings.EqualFold(strings.TrimSpace(params.Metric), "count") && params.GroupBy == "" {
		total, err := t.frappeClient.GetCount(ctx, params.DocType, params.Filters)
		if err != nil {
			return nil, fmt.Errorf("failed to count documents: %w", err)
		}

		countJSON, err := json.Marshal(map[string]interface{}{
			"doctype": params.DocType,
			"metric":  "count",
			"count":   total,
			"filters": params.Filters,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal results: %w", err)
		}

		return &mcp.ToolResponse{
			ID: request.ID,
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("%s has %d matching record(s)", params.DocType, total),
				},
				{
					Type: "text",
					Text: string(countJSON),
				},
			},
		}, nil
	}

	switch strings.ToLower(strings.TrimSpace(params.Metric)) {
	case "count":
	case "sum", "avg", "min", "max":
		if params.Field == "" {
			return nil, fmt.Errorf("metric %q needs a field", params.Metric)
		}
	default:
		return nil, fmt.Errorf("metric must be count, sum, avg, min or max, not %q", params.Metric)
	}
	params.Metric = strings.TrimSpace(params.Metric)

	// Execute aggregation query
	results, err := t.frappeClient.RunAggregationQuery(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to run aggregation query: %w", err)
	}

	// Build response
	resultJSON, err := json.Marshal(map[string]interface{}{
		"doctype":  params.DocType,
		"group_by": params.GroupBy,
		"results":  results,
		"count":    len(results),
		"metric":   params.Metric,
		"field":    params.Field,
		"filters":  params.Filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Aggregation query on %s returned %d result(s)", params.DocType, len(results)),
			},
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}

func (t *ToolRegistry) RunReport(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params types.ReportRequest

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.ReportName == "" {
		return nil, fmt.Errorf("report_name is required")
	}

	// Execute report
	reportData, err := t.frappeClient.RunReport(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to run report: %w", err)
	}

	// Format the response
	resultJSON, err := json.Marshal(map[string]interface{}{
		"report_name": params.ReportName,
		"columns":     reportData.Columns,
		"data":        reportData.Data,
		"row_count":   len(reportData.Data),
		"filters":     params.Filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report data: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Report '%s' executed successfully with %d row(s)", params.ReportName, len(reportData.Data)),
			},
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}

// SearchKnowledgeBase searches the documents the user has uploaded to Drive.
func (t *ToolRegistry) SearchKnowledgeBase(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		Query   string `json:"query"`
		Limit   int    `json:"limit,omitempty"`
		Session string `json:"session,omitempty"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	passages, err := t.frappeClient.SearchKnowledgeBase(ctx, params.Query, params.Limit, params.Session)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(passages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Found %d passage(s) for %q", len(passages), params.Query),
			},
			{
				Type: "text",
				Text: string(result),
			},
		},
	}, nil
}

func (t *ToolRegistry) GlobalSearch(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		Text    string      `json:"text"`
		Doctype string      `json:"doctype"`
		Scope   interface{} `json:"scope"` // string or []string
		Limit   int         `json:"limit"`
		Start   int         `json:"start"`
	}

	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.Text == "" {
		return nil, fmt.Errorf("text is required")
	}

	results, err := t.frappeClient.GlobalSearch(ctx, frappe.GlobalSearchRequest{
		Text:    params.Text,
		Doctype: params.Doctype,
		Scope:   params.Scope,
		Limit:   params.Limit,
		Start:   params.Start,
	})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return &mcp.ToolResponse{
		ID: request.ID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Found %d result(s) for %q", len(results), params.Text),
			},
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}
