package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/mcp"
)

func rowsParams(t *testing.T, doctype string, n int) json.RawMessage {
	t.Helper()
	rows := make([]map[string]interface{}, n)
	for i := range rows {
		rows[i] = map[string]interface{}{"subject": fmt.Sprintf("Examplia task %d", i+1)}
	}
	raw, err := json.Marshal(map[string]interface{}{"doctype": doctype, "rows": rows})
	require.NoError(t, err)
	return raw
}

func TestCreateDocumentsInsertsTheBatchInOneCall(t *testing.T) {
	registry, ctx := confirmedRegistry(t)

	response, err := registry.CreateDocuments(ctx, mcp.ToolRequest{
		ID: "b-1", Tool: "create_documents", Params: rowsParams(t, "Task", 3),
	})

	require.NoError(t, err)
	require.Len(t, response.Content, 2)
	assert.Equal(t, "Successfully created 3 Task documents: TASK-1, TASK-2, TASK-3", response.Content[0].Text)
	assert.JSONEq(t, `["TASK-1","TASK-2","TASK-3"]`, response.Content[1].Text)
}

func TestCreateDocumentsNeedsTheUsersConfirmation(t *testing.T) {
	registry := NewRegistry(createTestClient(t))

	response, err := registry.CreateDocuments(context.Background(), mcp.ToolRequest{
		ID: "b-1", Tool: "create_documents", Params: rowsParams(t, "Task", 2),
	})

	require.Error(t, err)
	assert.Equal(t, RefusalNotConfirmed, err.Error())
	assert.Nil(t, response)
}

func TestCreateDocumentsRefusesWhatCannotBeOneBatch(t *testing.T) {
	registry, ctx := confirmedRegistry(t)
	cases := map[string]json.RawMessage{
		"no rows":         json.RawMessage(`{"doctype":"Task","rows":[]}`),
		"no doctype":      json.RawMessage(`{"rows":[{"subject":"x"}]}`),
		"an empty row":    json.RawMessage(`{"doctype":"Task","rows":[{"subject":"x"},{}]}`),
		"over the cap":    rowsParams(t, "Task", maxRows+1),
		"not an object":   json.RawMessage(`{"doctype":"Task","rows":["x"]}`),
		"rows not a list": json.RawMessage(`{"doctype":"Task","rows":{"subject":"x"}}`),
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			response, err := registry.CreateDocuments(ctx, mcp.ToolRequest{ID: "b-1", Params: params})
			require.Error(t, err)
			assert.Nil(t, response)
		})
	}
}

func TestTheBatchCapIsNamedInTheRefusal(t *testing.T) {
	registry, ctx := confirmedRegistry(t)
	_, err := registry.CreateDocuments(ctx, mcp.ToolRequest{ID: "b-1", Params: rowsParams(t, "Task", maxRows+1)})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "split the batch"), err.Error())
}

func TestABatchTokenIsMintedForItsSize(t *testing.T) {
	assert.Equal(t, "3 rows", BatchConfirmationName(3))
	assert.Equal(t, "100 rows", BatchConfirmationName(100))
}

func TestCreateDocumentsIsAWriteInTheCatalogue(t *testing.T) {
	for _, tool := range NewRegistry(nil).Catalog(false) {
		if tool.Name == "create_documents" {
			require.NotNil(t, tool.ReadOnly)
			assert.False(t, *tool.ReadOnly, "a write must never be declared read-only, or it skips the confirmation")
			return
		}
	}
	t.Fatal("create_documents is not in the catalogue")
}
