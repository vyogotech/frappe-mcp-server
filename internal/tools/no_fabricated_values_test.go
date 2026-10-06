package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"frappe-mcp-server/internal/mcp"
)

// A tool answers with what Frappe returned. A field no code computes is not reported at all, because a model reads
// "timeline_health": "analyzing..." or "health": "Green" as a finding about the project. See docs/adr/ADR-018.
func TestAnalyzeProjectTimelineReportsNoUncomputedField(t *testing.T) {
	registry := NewRegistry(createTestClient(t))

	request := mcp.ToolRequest{
		ID:     "test-1",
		Tool:   "analyze_project_timeline",
		Params: []byte(`{"project_name":"TEST-PROJ-001"}`),
	}

	response, err := registry.AnalyzeProjectTimeline(context.Background(), request)
	if err != nil {
		t.Fatalf("AnalyzeProjectTimeline: %v", err)
	}

	var analysis struct {
		TimelineAnalysis map[string]json.RawMessage `json:"timeline_analysis"`
	}
	if err := json.Unmarshal([]byte(response.Content[1].Text), &analysis); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}

	for _, field := range []string{"critical_path_tasks", "milestones", "timeline_health"} {
		if raw, ok := analysis.TimelineAnalysis[field]; ok {
			t.Errorf("timeline_analysis reports %q as %s; nothing computes it", field, raw)
		}
	}
	if got := string(analysis.TimelineAnalysis["total_tasks"]); got != "2" {
		t.Errorf("total_tasks = %s, want 2 (the tasks the mock returned)", got)
	}
}

func TestFabricatedProjectToolsStayRemoved(t *testing.T) {
	registry := reflect.TypeOf(NewRegistry(nil))
	for _, method := range []string{"CalculateProjectMetrics", "ProjectRiskAssessment", "PortfolioDashboard"} {
		if _, found := registry.MethodByName(method); found {
			t.Errorf("%s is back; it answered Green, Low and zero whatever the data said (docs/adr/ADR-018)", method)
		}
	}
}
