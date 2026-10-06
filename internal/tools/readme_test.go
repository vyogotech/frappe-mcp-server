package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	listStart = "<!-- tools:start -->"
	listEnd   = "<!-- tools:end -->"
)

func toolTable() string {
	rows := []string{"| Tool | What the model is told |", "| --- | --- |"}
	for _, tool := range NewRegistry(nil).Catalog(true) {
		description := tool.Description
		if description == "" {
			description = "Legacy name, still callable, described to nobody"
		}
		rows = append(rows, fmt.Sprintf("| `%s` | %s |", tool.Name, description))
	}
	return strings.Join(rows, "\n")
}

func TestReadmeToolTableMatchesTheCatalogue(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(readme), listStart)
	if !found {
		t.Fatalf("README.md has no %s marker", listStart)
	}
	table, _, found := strings.Cut(rest, listEnd)
	if !found {
		t.Fatalf("README.md has no %s marker", listEnd)
	}
	if got, want := strings.TrimSpace(table), toolTable(); got != want {
		t.Errorf("the README tool table has drifted from the catalogue; paste this between the markers:\n%s", want)
	}
}
