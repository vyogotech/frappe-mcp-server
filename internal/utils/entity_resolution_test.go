package utils

import (
	"testing"
)

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		a, b   string
		expect bool
	}{
		{"Project Alpha", "project alpha", true},
		{"Alpha Project", "Project Alpha", true},
		{"Alpha", "Alfa", true},
		{"Alpha", "Beta", false},
		{"Alpha123", "Alpha124", true},
		{"Alpha", "Alphabeta", true},
		{"Alpha", "Gamma", false},
	}
	for _, c := range cases {
		if got := FuzzyMatch(c.a, c.b); got != c.expect {
			t.Errorf("FuzzyMatch(%q, %q) = %v, want %v", c.a, c.b, got, c.expect)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	if LevenshteinDistance("kitten", "sitting") != 3 {
		t.Error("LevenshteinDistance('kitten', 'sitting') != 3")
	}
	if LevenshteinDistance("flaw", "lawn") != 2 {
		t.Error("LevenshteinDistance('flaw', 'lawn') != 2")
	}
}

func TestResolveEntity(t *testing.T) {
	candidates := []string{"Project Alpha", "Project Beta", "Project Gamma"}
	if id, ok := ResolveEntity("alpha", candidates); !ok || id != "Project Alpha" {
		t.Errorf("ResolveEntity failed for 'alpha'")
	}
	if id, ok := ResolveEntity("beta", candidates); !ok || id != "Project Beta" {
		t.Errorf("ResolveEntity failed for 'beta'")
	}
	if _, ok := ResolveEntity("delta", candidates); ok {
		t.Errorf("ResolveEntity should fail for 'delta'")
	}
}
