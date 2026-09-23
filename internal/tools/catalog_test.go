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
