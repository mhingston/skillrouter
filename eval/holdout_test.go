package eval_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	sreval "github.com/mhingston/skillrouter/internal/eval"
)

type holdoutManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Owner         string `json:"owner"`
	CatalogCommit string `json:"catalog_commit"`
	CaseCount     int    `json:"case_count"`
	PositiveCases int    `json:"positive_cases"`
	NoMatchCases  int    `json:"no_match_cases"`
	CorpusSHA256  string `json:"corpus_sha256"`
	Reporting     string `json:"reporting"`
}

func TestProtectedHoldoutContract(t *testing.T) {
	manifestRaw, err := os.ReadFile("holdout-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest holdoutManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 {
		t.Fatalf("schema_version=%d", manifest.SchemaVersion)
	}
	if !regexp.MustCompile(`^@[A-Za-z0-9-]+$`).MatchString(manifest.Owner) {
		t.Fatalf("owner must be an explicit GitHub handle, got %q", manifest.Owner)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(manifest.CatalogCommit) {
		t.Fatalf("catalog_commit must be a full commit SHA, got %q", manifest.CatalogCommit)
	}
	if manifest.Reporting != "aggregate-only" {
		t.Fatalf("reporting=%q", manifest.Reporting)
	}

	holdoutRaw, err := os.ReadFile("holdout.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(holdoutRaw)
	if got := hex.EncodeToString(digest[:]); got != manifest.CorpusSHA256 {
		t.Fatalf("holdout digest changed: got %s, update the corpus and manifest together", got)
	}
	holdout, err := sreval.LoadJSONL(bytes.NewReader(holdoutRaw))
	if err != nil {
		t.Fatal(err)
	}
	if len(holdout) != manifest.CaseCount {
		t.Fatalf("case count=%d, manifest=%d", len(holdout), manifest.CaseCount)
	}
	if len(holdout) < 10 || len(holdout) > 20 {
		t.Fatalf("protected holdout must remain small (10-20 cases), got %d", len(holdout))
	}
	if manifest.PositiveCases != 8 || manifest.NoMatchCases != 4 {
		t.Fatalf("holdout balance must remain 8 positive / 4 no-match, manifest=%d/%d", manifest.PositiveCases, manifest.NoMatchCases)
	}
	if manifest.PositiveCases+manifest.NoMatchCases != manifest.CaseCount {
		t.Fatalf("manifest class counts do not sum to case_count")
	}

	developmentRaw, err := os.ReadFile("corpus.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	development, err := sreval.LoadJSONL(bytes.NewReader(developmentRaw))
	if err != nil {
		t.Fatal(err)
	}
	developmentIDs := make(map[string]bool, len(development))
	developmentQueries := make(map[string]bool, len(development))
	for _, testCase := range development {
		developmentIDs[testCase.ID] = true
		developmentQueries[testCase.Query] = true
	}

	idPattern := regexp.MustCompile(`^holdout-[0-9]{3}$`)
	noMatch := 0
	for _, testCase := range holdout {
		if !idPattern.MatchString(testCase.ID) {
			t.Fatalf("holdout ID must be opaque, got %q", testCase.ID)
		}
		if developmentIDs[testCase.ID] || developmentQueries[testCase.Query] {
			t.Fatalf("holdout case %q duplicates development data", testCase.ID)
		}
		if len(testCase.Relevant) == 0 {
			noMatch++
		}
	}
	positive := len(holdout) - noMatch
	if positive != manifest.PositiveCases || noMatch != manifest.NoMatchCases {
		t.Fatalf("holdout balance=%d positive/%d no-match, manifest=%d/%d", positive, noMatch, manifest.PositiveCases, manifest.NoMatchCases)
	}
}
