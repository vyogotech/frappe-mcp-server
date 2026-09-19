package frappe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/types"
)

// Frappe v16's query_report.run returns rows as objects; a prepared report from an older cache returns arrays.
func TestRunReportReadsObjectAndArrayRows(t *testing.T) {
	for name, body := range map[string]string{
		"objects": `{"message": {"columns": [{"fieldname": "table"}, {"fieldname": "size"}], "result": [{"table": "tabFile", "size": 1.5}]}}`,
		"arrays":  `{"message": {"columns": [{"fieldname": "table"}, {"fieldname": "size"}], "result": [["tabFile", 1.5]]}}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		c, _ := NewClient(config.ERPNextConfig{BaseURL: srv.URL, APIKey: "k", APISecret: "s", Timeout: time.Second,
			RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100}})
		got, err := c.RunReport(context.Background(), types.ReportRequest{ReportName: "Database Storage Usage By Tables"})
		srv.Close()
		if err != nil || len(got.Data) != 1 || got.Data[0]["table"] != "tabFile" || got.Data[0]["size"] != 1.5 {
			t.Errorf("%s: %v %+v", name, err, got)
		}
	}
}
