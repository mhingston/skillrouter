package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mhingston/skillrouter/internal/catalog"
	sreval "github.com/mhingston/skillrouter/internal/eval"
)

func TestAggregateOnlySelectsRedactedReport(t *testing.T) {
	report := sreval.Report{
		Metrics: sreval.Metrics{TotalCases: 1},
		Cases: []sreval.CaseResult{{
			ID:       "holdout-001",
			Query:    "protected query",
			Relevant: []string{"protected-label"},
			Top:      []string{"protected-ranking"},
		}},
	}

	encoded, err := json.Marshal(reportForOutput(report, true))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"\"cases\":", "holdout-001", "protected query", "protected-label", "protected-ranking"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("aggregate-only output leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestDefaultReportKeepsDevelopmentDiagnostics(t *testing.T) {
	report := sreval.Report{Cases: []sreval.CaseResult{{ID: "development-case"}}}
	encoded, err := json.Marshal(reportForOutput(report, false))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "development-case") {
		t.Fatalf("development output lost case diagnostics: %s", encoded)
	}
}

func TestAggregateOnlySummaryOmitsCaseDiagnostics(t *testing.T) {
	report := sreval.Report{
		Metrics: sreval.Metrics{TotalCases: 1},
		Cases: []sreval.CaseResult{{
			ID:                "holdout-001",
			Relevant:          []string{"protected-label"},
			Top:               []string{"protected-ranking"},
			FirstRelevantRank: 0,
		}},
	}

	var output bytes.Buffer
	writeSummary(&output, report, true)
	for _, forbidden := range []string{"MISS", "holdout-001", "protected-label", "protected-ranking"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("aggregate-only summary leaked %q: %s", forbidden, output.String())
		}
	}
}

func TestAggregateOnlyRedactsValidationFailure(t *testing.T) {
	detailed := validateRelevantSkills(
		[]sreval.Case{{ID: "holdout-001", Relevant: []string{"protected-label"}}},
		map[string]catalog.Skill{},
	)
	if detailed == nil {
		t.Fatal("expected validation failure")
	}

	got := outputError(detailed, true, "protected corpus validation failed")
	for _, forbidden := range []string{"holdout-001", "protected-label"} {
		if strings.Contains(got.Error(), forbidden) {
			t.Fatalf("aggregate-only error leaked %q: %s", forbidden, got)
		}
	}
	if got.Error() != "protected corpus validation failed" {
		t.Fatalf("unexpected protected error: %s", got)
	}
	if unredacted := outputError(detailed, false, "protected corpus validation failed"); !errors.Is(unredacted, detailed) {
		t.Fatalf("default mode did not retain detailed error: %s", unredacted)
	}
}
