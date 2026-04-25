package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"frappe-mcp-server/internal/mcp"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// neo4jUnavailableResponse returns a graceful MCP response when the graph is not configured.
func (t *ToolRegistry) neo4jUnavailableResponse(reqID, toolName string) *mcp.ToolResponse {
	slog.Warn("Graph tool called but Neo4j is unavailable", "tool", toolName)
	return &mcp.ToolResponse{
		ID: reqID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("FrappeForge graph database is unavailable or not configured. Cannot execute %s.", toolName),
			},
		},
	}
}

// executeGraphQuery is a helper to run a Cypher query and return a standard JSON MCP response.
func (t *ToolRegistry) executeGraphQuery(ctx context.Context, reqID, toolName, cypher string, params map[string]any, title string) (*mcp.ToolResponse, error) {
	if t.neo4jClient == nil {
		return t.neo4jUnavailableResponse(reqID, toolName), nil
	}

	rows, err := t.neo4jClient.Query(ctx, cypher, params)
	if err != nil {
		// If the query fails due to connectivity (or syntax), return a graceful message rather than a hard MCP error
		// so the agent can understand what happened.
		return &mcp.ToolResponse{
			ID: reqID,
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("FrappeForge graph query failed: %v", err),
				},
			},
		}, nil
	}

	resultJSON, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal graph results: %w", err)
	}

	return &mcp.ToolResponse{
		ID: reqID,
		Content: []mcp.Content{
			{
				Type: "text",
				Text: title,
			},
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}

// ── Tools ────────────────────────────────────────────────────────────────────

// FfGraphStats returns total node and relationship counts in the knowledge graph.
func (t *ToolRegistry) FfGraphStats(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	if t.neo4jClient == nil {
		return t.neo4jUnavailableResponse(request.ID, request.Tool), nil
	}

	cypher := `
		MATCH (n)
		WITH count(n) as nodes
		OPTIONAL MATCH ()-[r]->()
		RETURN nodes, count(r) as relationships
	`
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, nil, "FrappeForge Knowledge Graph Statistics")
}

// FfListIngestedProjects lists all projects currently indexed in the graph.
func (t *ToolRegistry) FfListIngestedProjects(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	if t.neo4jClient == nil {
		return t.neo4jUnavailableResponse(request.ID, request.Tool), nil
	}

	cypher := `
		MATCH (p:Project)
		RETURN p.name as project, p.repo as repository, p.version as version
		ORDER BY p.name
	`
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, nil, "Ingested Projects in FrappeForge Graph")
}

// FfSearchDoctype finds doctypes matching a partial string (e.g. "Purchase").
func (t *ToolRegistry) FfSearchDoctype(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	cypher := `
		MATCH (d:DocType)-[:BELONGS_TO]->(p:Project)
		WHERE toLower(d.name) CONTAINS toLower($query) OR toLower(d.module) CONTAINS toLower($query)
		RETURN d.name as name, d.module as module, p.name as project
		ORDER BY d.name
		LIMIT 20
	`
	p := map[string]any{"query": params.Query}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("DocTypes matching '%s'", params.Query))
}

// FfGetDoctypeDetail returns the full field schema of a doctype.
func (t *ToolRegistry) FfGetDoctypeDetail(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType string `json:"doctype"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	cypher := `
		MATCH (d:DocType {name: $doctype})-[:BELONGS_TO]->(p:Project)
		OPTIONAL MATCH (d)-[:HAS_FIELD]->(f:Field)
		RETURN d.name as name, d.module as module, p.name as project,
		       collect({
		           fieldname: f.fieldname,
		           fieldtype: f.fieldtype,
		           label: f.label,
		           options: f.options,
		           mandatory: f.reqd,
		           hidden: f.hidden
		       }) as fields
	`
	p := map[string]any{"doctype": params.DocType}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("Schema for DocType '%s'", params.DocType))
}

// FfGetDoctypeControllers returns the Python controller methods for a doctype.
func (t *ToolRegistry) FfGetDoctypeControllers(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType string `json:"doctype"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	cypher := `
		MATCH (d:DocType {name: $doctype})-[:HAS_CONTROLLER]->(c:PythonClass)-[:HAS_METHOD]->(m:PythonMethod)
		RETURN c.name as class_name, c.filepath as file,
		       collect({
		           name: m.name,
		           args: m.args
		       }) as methods
	`
	p := map[string]any{"doctype": params.DocType}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("Python Controllers for '%s'", params.DocType))
}

// FfGetDoctypeClientScripts returns the JavaScript client script events for a doctype.
func (t *ToolRegistry) FfGetDoctypeClientScripts(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType string `json:"doctype"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	cypher := `
		MATCH (d:DocType {name: $doctype})-[:HAS_CLIENT_SCRIPT]->(f:JSFile)-[:HAS_EVENT]->(e:JSEvent)
		RETURN f.filepath as file,
		       collect(e.name) as events
	`
	p := map[string]any{"doctype": params.DocType}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("Client Scripts for '%s'", params.DocType))
}

// FfFindDoctypesWithField finds any doctype that contains a specific fieldname.
func (t *ToolRegistry) FfFindDoctypesWithField(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		FieldName string `json:"fieldname"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.FieldName == "" {
		return nil, fmt.Errorf("fieldname is required")
	}

	cypher := `
		MATCH (d:DocType)-[:HAS_FIELD]->(f:Field)
		WHERE toLower(f.fieldname) = toLower($fieldname) OR toLower(f.label) CONTAINS toLower($fieldname)
		RETURN d.name as doctype, f.fieldname as fieldname, f.fieldtype as fieldtype, f.options as options
		ORDER BY d.name
		LIMIT 50
	`
	p := map[string]any{"fieldname": params.FieldName}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("DocTypes containing field '%s'", params.FieldName))
}

// FfGetDoctypeLinks finds other doctypes that link TO the specified doctype.
func (t *ToolRegistry) FfGetDoctypeLinks(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType string `json:"doctype"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	cypher := `
		MATCH (source:DocType)-[:HAS_FIELD]->(f:Field {fieldtype: 'Link', options: $doctype})
		RETURN source.name as doctype, f.fieldname as fieldname, f.label as label
		ORDER BY source.name
	`
	p := map[string]any{"doctype": params.DocType}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("DocTypes linking to '%s'", params.DocType))
}

// FfSearchMethods finds Python methods across the graph by name.
func (t *ToolRegistry) FfSearchMethods(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	cypher := `
		MATCH (m:PythonMethod)
		WHERE toLower(m.name) CONTAINS toLower($query)
		OPTIONAL MATCH (c:PythonClass)-[:HAS_METHOD]->(m)
		OPTIONAL MATCH (d:DocType)-[:HAS_CONTROLLER]->(c)
		RETURN m.name as method, m.args as args, c.name as class_name, c.filepath as filepath, d.name as doctype
		ORDER BY c.name, m.name
		LIMIT 50
	`
	p := map[string]any{"query": params.Query}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("Python methods matching '%s'", params.Query))
}

// FfGetHooks returns the Frappe hooks registered for a doctype.
func (t *ToolRegistry) FfGetHooks(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var params struct {
		DocType string `json:"doctype"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if params.DocType == "" {
		return nil, fmt.Errorf("doctype is required")
	}

	cypher := `
		MATCH (p:Project)-[:HAS_HOOKS]->(h:Hooks)-[:HAS_DOC_EVENT]->(e:DocEvent {doctype: $doctype})
		RETURN p.name as project, e.event_type as event, e.handler as handler
		ORDER BY p.name, e.event_type
	`
	p := map[string]any{"doctype": params.DocType}
	return t.executeGraphQuery(ctx, request.ID, request.Tool, cypher, p, fmt.Sprintf("Hooks for '%s'", params.DocType))
}
