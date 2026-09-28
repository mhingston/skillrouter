package eval

import (
	"strings"
	"testing"

	"github.com/mhingston/skillrouter/internal/search"
)

type fakeSearcher map[string][]string

func (f fakeSearcher) Search(query string, _ int) ([]search.Candidate, string, string, error) {
	names := f[query]
	out := make([]search.Candidate, len(names))
	for i, name := range names {
		out[i] = search.Candidate{Name: name}
	}
	return out, "fake", "", nil
}

func TestLoadJSONLRejectsDuplicateIDs(t *testing.T) {
	_, err := LoadJSONL(strings.NewReader("{\"id\":\"a\",\"query\":\"one\"}\n{\"id\":\"a\",\"query\":\"two\"}\n"))
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestRunComputesRetrievalAndNoMatchMetrics(t *testing.T) {
	cases := []Case{
		{ID: "a", Query: "q1", Relevant: []string{"review"}},
		{ID: "b", Query: "q2", Relevant: []string{"plan"}},
		{ID: "c", Query: "q3"},
	}
	report, err := Run(fakeSearcher{
		"q1": {"review", "plan"},
		"q2": {"other", "plan"},
		"q3": {},
	}, cases, 5)
	if err != nil {
		t.Fatal(err)
	}
	if report.Metrics.RecallAt1 != 0.5 {
		t.Fatalf("recall@1=%v", report.Metrics.RecallAt1)
	}
	if report.Metrics.RecallAt3 != 1 {
		t.Fatalf("recall@3=%v", report.Metrics.RecallAt3)
	}
	if report.Metrics.MRR != 0.75 {
		t.Fatalf("mrr=%v", report.Metrics.MRR)
	}
	if report.Metrics.NoMatchPrecision != 1 || report.Metrics.NoMatchRecall != 1 {
		t.Fatalf("no-match metrics=%+v", report.Metrics)
	}
}
