package tools

import "testing"

func TestCatalogIsOneWellFormedTable(t *testing.T) {
	registry := NewRegistry(nil)
	seen := map[string]bool{}
	for _, tool := range registry.Catalog(true) {
		if tool.Name == "" || tool.Handler == nil {
			t.Errorf("catalogue row %+v has no name or no handler", tool)
		}
		if seen[tool.Name] {
			t.Errorf("%s appears twice", tool.Name)
		}
		seen[tool.Name] = true
	}
	// These three answered with placeholder numbers and were deleted; nothing may publish them again.
	for _, gone := range []string{"calculate_project_metrics", "project_risk_assessment", "portfolio_dashboard"} {
		if seen[gone] {
			t.Errorf("%s is back in the catalogue", gone)
		}
	}
}

func TestCatalogDropsTheKnowledgeBaseWhenItIsOff(t *testing.T) {
	registry := NewRegistry(nil)
	on, off := registry.Catalog(true), registry.Catalog(false)
	if len(on) != len(off)+1 {
		t.Fatalf("knowledge base on: %d tools, off: %d; want one more", len(on), len(off))
	}
	for _, tool := range off {
		if tool.Name == "search_knowledge_base" {
			t.Error("search_knowledge_base is offered where the rag app is not installed")
		}
	}
}

// A count decoded into a Go int must be declared "integer": clients that follow the schema send a "number" as
// 3.0 (OpenHands does), and encoding/json refuses 3.0 for an int field, so the call fails before it runs.
func TestCountArgumentsAreDeclaredIntegers(t *testing.T) {
	counts := map[string]bool{"page_length": true, "limit": true, "start": true, "top_n": true}
	for _, tool := range NewRegistry(nil).Catalog(true) {
		props, _ := tool.InputSchema["properties"].(map[string]interface{})
		for name, raw := range props {
			prop, _ := raw.(map[string]interface{})
			if counts[name] && prop["type"] != "integer" {
				t.Errorf("%s.%s is %v; declare it integer", tool.Name, name, prop["type"])
			}
		}
	}
}
