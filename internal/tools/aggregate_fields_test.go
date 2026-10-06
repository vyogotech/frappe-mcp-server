package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/mcp"
)

// The schema advertises metric, field and top_n; Frappe v16's get_list takes an aggregate only as {"SUM": field}.
func TestAggregateDocuments_UsesTheAdvertisedArguments(t *testing.T) {
	t.Run("sums the field per group", func(t *testing.T) {
		client, rec := newCountingFrappe(t, 50)
		aggregate(t, NewRegistry(client), `{"doctype":"Sales Invoice","metric":"sum","field":"grand_total","group_by":"customer"}`)
		assert.Equal(t, []interface{}{"customer", map[string]interface{}{"SUM": "grand_total", "as": "value"}}, rec.lastBody["fields"])
		assert.Equal(t, float64(0), rec.lastBody["limit_page_length"], "every group, not Frappe's default page of 20")
	})

	t.Run("top_n keeps the largest groups", func(t *testing.T) {
		client, rec := newCountingFrappe(t, 50)
		aggregate(t, NewRegistry(client), `{"doctype":"Sales Invoice","metric":"count","group_by":"customer","top_n":3}`)
		assert.Equal(t, float64(3), rec.lastBody["limit_page_length"])
		assert.Equal(t, "value desc", rec.lastBody["order_by"])
	})

	t.Run("a metric other than count needs a field, and an unknown metric is refused", func(t *testing.T) {
		client, _ := newCountingFrappe(t, 50)
		for _, params := range []string{`{"doctype":"Sales Invoice","metric":"sum"}`, `{"doctype":"Sales Invoice","metric":"median","field":"x"}`} {
			_, err := NewRegistry(client).AggregateDocuments(context.Background(), mcp.ToolRequest{ID: "a", Tool: "aggregate_documents", Params: []byte(params)})
			require.Error(t, err, params)
		}
	})
}
