package search

import (
	"testing"

	"github.com/mhingston/skillrouter/internal/catalog"
)

type fakeEmbedder struct{}

func (fakeEmbedder) ID() string { return "fake" }
func (fakeEmbedder) Embed(texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, text := range texts {
		if contains(text, "db-safe") || contains(text, "database") || contains(text, "migration") {
			out[i] = []float64{1, 0}
		} else {
			out[i] = []float64{0, 1}
		}
	}
	return out, nil
}

func TestLexicalSearchFindsDescriptionMatch(t *testing.T) {
	s := &catalog.Snapshot{Skills: map[string]catalog.Skill{
		"db-safe": {Name: "db-safe", Description: "plan a safe database migration"},
		"ui":      {Name: "ui", Description: "review frontend accessibility"},
	}, Names: []string{"db-safe", "ui"}}
	idx, err := New(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	results, mode, _, err := idx.Search("database migration", 5)
	if err != nil {
		t.Fatal(err)
	}
	if mode != "lexical" || len(results) == 0 || results[0].Name != "db-safe" {
		t.Fatalf("mode=%s results=%v", mode, results)
	}
}

func TestHybridSearchUsesEmbeddings(t *testing.T) {
	s := &catalog.Snapshot{Skills: map[string]catalog.Skill{
		"db-safe": {Name: "db-safe", Description: "safe schema changes"},
		"ui":      {Name: "ui", Description: "frontend accessibility"},
	}, Names: []string{"db-safe", "ui"}}
	idx, err := New(s, fakeEmbedder{})
	if err != nil {
		t.Fatal(err)
	}
	results, mode, _, err := idx.Search("database migration", 5)
	if err != nil {
		t.Fatal(err)
	}
	if mode != "hybrid" || len(results) == 0 || results[0].Name != "db-safe" {
		t.Fatalf("mode=%s results=%v", mode, results)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
