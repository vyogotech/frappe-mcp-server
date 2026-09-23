package tools

import (
	"fmt"
	"slices"
	"strings"

	"frappe-mcp-server/internal/mcp"
)

// Tool is one row of the catalogue: what tools/list publishes for a tool and what runs when it is called.
type Tool struct {
	Name    string
	Handler mcp.ToolHandler
	mcp.ToolMeta
}

// Catalog is every tool a server offers, in the order the listings publish them. A row with no description is a legacy
// name kept callable but out of the REST listing. knowledgeBase is false where the rag app search_knowledge_base reads
// is not installed.
func (t *ToolRegistry) Catalog(knowledgeBase bool) []Tool {
	strProp := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	readOnly := func(b bool) *bool { return &b }
	objSchema := func(props map[string]interface{}, required ...string) map[string]interface{} {
		schema := map[string]interface{}{
			"type":       "object",
			"properties": props,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	catalog := []Tool{
		{Name: "get_document", Handler: t.GetDocument, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Retrieve a single ERPNext document by doctype and name",
			InputSchema: objSchema(map[string]interface{}{
				"doctype": strProp("ERPNext document type (e.g., Customer, Sales Order)"),
				"name":    strProp("Document name or ID"),
			}, "doctype", "name"),
		}},
		{Name: "list_documents", Handler: t.ListDocuments, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Fetch document rows to read their contents. Returns at most page_length rows (default 20), so it CANNOT be used to count records - use aggregate_documents for counts.",
			InputSchema: objSchema(map[string]interface{}{
				"doctype":     strProp("ERPNext document type"),
				"page_length": map[string]interface{}{"type": "number", "description": "Maximum results to return", "default": 20},
				"filters":     map[string]interface{}{"type": "object", "description": "Optional field-value filters"},
				"fields":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Fields to return"},
				"order_by":    strProp("Sort order (e.g., 'creation desc')"),
			}, "doctype"),
		}},
		{Name: "create_document", Handler: t.CreateDocument, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(false),
			Description: "Create a new ERPNext document. `data` is a flat object of fieldname→value pairs (NOT spread into top-level args).",
			InputSchema: objSchema(map[string]interface{}{
				"doctype": strProp("ERPNext document type (e.g., Customer, Sales Order)"),
				"data":    map[string]interface{}{"type": "object", "description": "Field values for the new document, as an object of fieldname→value"},
			}, "doctype", "data"),
		}},
		{Name: "update_document", Handler: t.UpdateDocument, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(false),
			Description: "Update an existing ERPNext document. `data` is a flat object of fieldname→value pairs for fields to change.",
			InputSchema: objSchema(map[string]interface{}{
				"doctype": strProp("ERPNext document type"),
				"name":    strProp("Document name or ID"),
				"data":    map[string]interface{}{"type": "object", "description": "Fields to update, as an object of fieldname→value"},
			}, "doctype", "name", "data"),
		}},
		{Name: "delete_document", Handler: t.DeleteDocument, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(false),
			Description: "Delete an ERPNext document",
			InputSchema: objSchema(map[string]interface{}{
				"doctype": strProp("ERPNext document type"),
				"name":    strProp("Document name or ID"),
			}, "doctype", "name"),
		}},
		{Name: "search_documents", Handler: t.SearchDocuments, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Search ERPNext documents of a given doctype using full-text search",
			InputSchema: objSchema(map[string]interface{}{
				"doctype":     strProp("Document type to search"),
				"search":      strProp("Search query string"),
				"page_length": map[string]interface{}{"type": "number", "description": "Maximum results to return", "default": 20},
				"filters":     map[string]interface{}{"type": "object", "description": "Optional field-value filters"},
			}, "doctype"),
		}},
		{Name: "aggregate_documents", Handler: t.AggregateDocuments, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Count, sum or average ERPNext records. Use this for any \"how many\" question: with metric=\"count\" it returns the exact total of all matching records, not just one page.",
			InputSchema: objSchema(map[string]interface{}{
				"doctype":  strProp("Document type to aggregate over"),
				"group_by": strProp("Field to group by (optional)"),
				"metric":   strProp("Aggregation: sum|count|avg|min|max"),
				"field":    strProp("Field to aggregate (required when metric != count)"),
				"filters":  map[string]interface{}{"type": "object", "description": "Optional field-value filters"},
				"top_n":    map[string]interface{}{"type": "integer", "description": "Return top N groups only"},
			}, "doctype", "metric"),
		}},
		{Name: "run_report", Handler: t.RunReport, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Execute a Frappe/ERPNext report (Sales Analytics, Purchase Register, etc.)",
			InputSchema: objSchema(map[string]interface{}{
				"report_name": strProp("Exact report name (e.g. 'Sales Analytics')"),
				"filters":     map[string]interface{}{"type": "object", "description": "Report filter values"},
			}, "report_name"),
		}},
		{Name: "global_search", Handler: t.GlobalSearch, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Full-text search across all indexed Frappe/ERPNext doctypes",
			InputSchema: objSchema(map[string]interface{}{
				"text":    strProp("Search keyword or phrase"),
				"doctype": strProp("Restrict results to a single doctype (optional)"),
				"scope":   map[string]interface{}{"description": "One doctype or list of doctypes to search within (optional)"},
				"limit":   map[string]interface{}{"type": "integer", "description": "Maximum number of results (default 20)"},
				"start":   map[string]interface{}{"type": "integer", "description": "Offset for pagination (default 0)"},
			}, "text"),
		}},
		{Name: "search_knowledge_base", Handler: t.SearchKnowledgeBase, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Search the user's uploaded documents (HR, expense, travel, security and vehicle policies) for a passage answering a question. Use for any policy, entitlement, limit or deadline question.",
			InputSchema: objSchema(map[string]interface{}{
				"query":   strProp("The user's question, in their own words"),
				"limit":   map[string]interface{}{"type": "integer", "description": "Maximum passages to return (default 5)"},
				"session": strProp("The chat the question comes from, whose attached files are searched too. Filled by the agent, not the model"),
			}, "query"),
			OutputSchema: objSchema(map[string]interface{}{
				"passages": map[string]interface{}{
					"type":        "array",
					"description": "The passages found, nearest the question first",
					"items": objSchema(map[string]interface{}{
						"file":       strProp("Name of the file the passage was read from"),
						"seq":        map[string]interface{}{"type": "integer", "description": "Position of the passage within that file"},
						"content":    strProp("The passage itself"),
						"distance":   map[string]interface{}{"type": "number", "description": "Distance from the question; smaller is nearer"},
						"file_name":  strProp("Readable name of a file attached to the chat, which is not in the user's Drive"),
						"attachment": map[string]interface{}{"type": "boolean", "description": "True when the passage comes from a file attached to the chat"},
					}, "file"),
				},
			}, "passages"),
		}},
		{Name: "analyze_document", Handler: t.AnalyzeDocument, ToolMeta: mcp.ToolMeta{
			ReadOnly:    readOnly(true),
			Description: "Analyze any ERPNext document with optional related data",
			InputSchema: objSchema(map[string]interface{}{
				"doctype":         strProp("ERPNext document type"),
				"name":            strProp("Document name or ID"),
				"include_related": map[string]interface{}{"type": "boolean", "description": "Include related documents", "default": false},
			}, "doctype", "name"),
		}},
		// Legacy tools, kept callable for backward compatibility; unlisted because they have no description.
		{Name: "get_project_status", Handler: t.GetProjectStatus, ToolMeta: mcp.ToolMeta{ReadOnly: readOnly(true)}},
		{Name: "analyze_project_timeline", Handler: t.AnalyzeProjectTimeline, ToolMeta: mcp.ToolMeta{ReadOnly: readOnly(true)}},
		{Name: "get_resource_allocation", Handler: t.GetResourceAllocation, ToolMeta: mcp.ToolMeta{ReadOnly: readOnly(true)}},
		{Name: "generate_project_report", Handler: t.GenerateProjectReport, ToolMeta: mcp.ToolMeta{ReadOnly: readOnly(true)}},
		{Name: "resource_utilization_analysis", Handler: t.ResourceUtilizationAnalysis, ToolMeta: mcp.ToolMeta{ReadOnly: readOnly(true)}},
		{Name: "budget_variance_analysis", Handler: t.BudgetVarianceAnalysis, ToolMeta: mcp.ToolMeta{ReadOnly: readOnly(true)}},
	}
	if !knowledgeBase {
		catalog = slices.DeleteFunc(catalog, func(tool Tool) bool { return tool.Name == "search_knowledge_base" })
	}
	return catalog
}

// Register publishes a catalogue on an MCP server. Both binaries register through here, so neither can ship a set the
// other does not, and neither can ship a tool that never declared whether it writes: the write gate keys on that
// declaration alone, so an undeclared tool stops the server rather than slipping through unguarded.
func Register(server *mcp.Server, catalog []Tool) error {
	var undeclared []string
	for _, tool := range catalog {
		if tool.ReadOnly == nil {
			undeclared = append(undeclared, tool.Name)
		}
	}
	if len(undeclared) > 0 {
		return fmt.Errorf("tools registered without a ReadOnly declaration: %s", strings.Join(undeclared, ", "))
	}

	for _, tool := range catalog {
		server.RegisterToolWithSchema(tool.Name, tool.ToolMeta, tool.Handler)
	}
	return nil
}
