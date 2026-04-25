package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/mcp"
	ffneo4j "frappe-mcp-server/internal/neo4j"
)

// newNilNeo4jRegistry returns a ToolRegistry with a nil neo4j client,
// simulating the graph being unavailable or unconfigured.
func newNilNeo4jRegistry(t *testing.T) *ToolRegistry {
	t.Helper()
	var nc *ffneo4j.Client
	return &ToolRegistry{neo4jClient: nc}
}

// ── ff_graph_stats ────────────────────────────────────────────────────────────

func TestFfGraphStats_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	resp, err := r.FfGraphStats(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	require.NoError(t, err, "should not return Go error when graph unavailable")
	require.NotNil(t, resp)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_list_ingested_projects ─────────────────────────────────────────────────

func TestFfListIngestedProjects_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	resp, err := r.FfListIngestedProjects(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_search_doctype ─────────────────────────────────────────────────────────

func TestFfSearchDoctype_MissingQuery(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfSearchDoctype(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestFfSearchDoctype_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"query": "Sales"})
	resp, err := r.FfSearchDoctype(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_get_doctype_detail ─────────────────────────────────────────────────────

func TestFfGetDoctypeDetail_MissingDoctype(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfGetDoctypeDetail(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doctype")
}

func TestFfGetDoctypeDetail_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"doctype": "Sales Invoice"})
	resp, err := r.FfGetDoctypeDetail(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_get_doctype_controllers ────────────────────────────────────────────────

func TestFfGetDoctypeControllers_MissingDoctype(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfGetDoctypeControllers(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doctype")
}

func TestFfGetDoctypeControllers_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"doctype": "Sales Invoice"})
	resp, err := r.FfGetDoctypeControllers(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_get_doctype_client_scripts ─────────────────────────────────────────────

func TestFfGetDoctypeClientScripts_MissingDoctype(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfGetDoctypeClientScripts(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doctype")
}

func TestFfGetDoctypeClientScripts_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"doctype": "Sales Invoice"})
	resp, err := r.FfGetDoctypeClientScripts(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_find_doctypes_with_field ───────────────────────────────────────────────

func TestFfFindDoctypesWithField_MissingFieldname(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfFindDoctypesWithField(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fieldname")
}

func TestFfFindDoctypesWithField_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"fieldname": "customer"})
	resp, err := r.FfFindDoctypesWithField(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_get_doctype_links ──────────────────────────────────────────────────────

func TestFfGetDoctypeLinks_MissingDoctype(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfGetDoctypeLinks(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doctype")
}

func TestFfGetDoctypeLinks_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"doctype": "Customer"})
	resp, err := r.FfGetDoctypeLinks(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_search_methods ─────────────────────────────────────────────────────────

func TestFfSearchMethods_MissingQuery(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfSearchMethods(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestFfSearchMethods_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"query": "validate"})
	resp, err := r.FfSearchMethods(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}

// ── ff_get_hooks ──────────────────────────────────────────────────────────────

func TestFfGetHooks_MissingDoctype(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	_, err := r.FfGetHooks(context.Background(), mcp.ToolRequest{ID: "t1", Params: []byte(`{}`)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doctype")
}

func TestFfGetHooks_Neo4jUnavailable(t *testing.T) {
	r := newNilNeo4jRegistry(t)
	params, _ := json.Marshal(map[string]string{"doctype": "Sales Invoice"})
	resp, err := r.FfGetHooks(context.Background(), mcp.ToolRequest{ID: "t1", Params: params})
	require.NoError(t, err)
	assert.Contains(t, resp.Content[0].Text, "unavailable")
}
