package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhingston/skillrouter/internal/catalog"
	"github.com/mhingston/skillrouter/internal/embed"
	sreval "github.com/mhingston/skillrouter/internal/eval"
	"github.com/mhingston/skillrouter/internal/search"
)

type dirsFlag []string

func (d *dirsFlag) String() string { return strings.Join(*d, string(os.PathListSeparator)) }
func (d *dirsFlag) Set(value string) error {
	*d = append(*d, value)
	return nil
}

func main() {
	log.SetFlags(0)
	var dirs dirsFlag
	flag.Var(&dirs, "skills-dir", "directory containing Agent Skills; repeat for multiple roots")
	corpusPath := flag.String("corpus", "eval/corpus.jsonl", "JSONL evaluation corpus")
	limit := flag.Int("limit", 10, "number of candidates to retrieve per case")
	jsonOut := flag.String("json-out", "", "optional path for the JSON report")
	aggregateOnly := flag.Bool("aggregate-only", false, "omit queries, labels, rankings, and per-case failures from output")
	embeddingURL := flag.String("embedding-url", env("SKILLROUTER_EMBEDDING_URL", ""), "OpenAI-compatible embeddings endpoint")
	embeddingModel := flag.String("embedding-model", env("SKILLROUTER_EMBEDDING_MODEL", ""), "embedding model")
	flag.Parse()

	if len(dirs) == 0 {
		if raw := os.Getenv("SKILLROUTER_SKILLS_DIRS"); raw != "" {
			for _, item := range filepath.SplitList(raw) {
				if strings.TrimSpace(item) != "" {
					dirs = append(dirs, item)
				}
			}
		}
	}
	if len(dirs) == 0 {
		log.Fatal("no skills roots configured; pass --skills-dir or set SKILLROUTER_SKILLS_DIRS")
	}

	file, err := os.Open(*corpusPath)
	if err != nil {
		log.Fatal(err)
	}
	cases, err := sreval.LoadJSONL(file)
	_ = file.Close()
	if err != nil {
		log.Fatal(outputError(err, *aggregateOnly, "protected corpus validation failed"))
	}

	snapshot, err := catalog.Load(dirs, catalog.DefaultMaxFileBytes)
	if err != nil {
		log.Fatal(err)
	}
	if err := validateRelevantSkills(cases, snapshot.Skills); err != nil {
		log.Fatal(outputError(err, *aggregateOnly, "protected corpus validation failed"))
	}
	var embedder search.Embedder
	if *embeddingURL != "" || *embeddingModel != "" {
		if *embeddingURL == "" || *embeddingModel == "" {
			log.Fatal("--embedding-url and --embedding-model must be supplied together")
		}
		embedder = &embed.OpenAICompatible{
			URL:    *embeddingURL,
			Model:  *embeddingModel,
			APIKey: os.Getenv("SKILLROUTER_EMBEDDING_API_KEY"),
		}
	}
	index, err := search.New(snapshot, embedder)
	if err != nil {
		log.Fatal(err)
	}
	report, err := sreval.Run(index, cases, *limit)
	if err != nil {
		log.Fatal(outputError(err, *aggregateOnly, "protected evaluation failed"))
	}

	printSummary(report, *aggregateOnly)
	if *jsonOut != "" {
		encoded, err := json.MarshalIndent(reportForOutput(report, *aggregateOnly), "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*jsonOut, append(encoded, '\n'), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

func validateRelevantSkills(cases []sreval.Case, skills map[string]catalog.Skill) error {
	for _, testCase := range cases {
		for _, name := range testCase.Relevant {
			if _, ok := skills[name]; !ok {
				return fmt.Errorf("corpus case %s references unknown skill %q", testCase.ID, name)
			}
		}
	}
	return nil
}

func outputError(err error, aggregateOnly bool, publicMessage string) error {
	if aggregateOnly {
		return errors.New(publicMessage)
	}
	return err
}

func printSummary(report sreval.Report, aggregateOnly bool) {
	writeSummary(os.Stdout, report, aggregateOnly)
}

func writeSummary(w io.Writer, report sreval.Report, aggregateOnly bool) {
	m := report.Metrics
	fmt.Fprintf(w, "cases: %d (%d positive, %d no-match)\n", m.TotalCases, m.PositiveCases, m.NoMatchCases)
	fmt.Fprintf(w, "recall@1: %.3f  recall@3: %.3f  recall@5: %.3f  MRR: %.3f\n", m.RecallAt1, m.RecallAt3, m.RecallAt5, m.MRR)
	fmt.Fprintf(w, "no-match precision: %.3f  no-match recall: %.3f  abstentions: %d\n", m.NoMatchPrecision, m.NoMatchRecall, m.Abstentions)
	fmt.Fprintf(w, "latency p50: %.3fms  p95: %.3fms  mean candidates: %.2f\n", m.LatencyP50MS, m.LatencyP95MS, m.MeanCandidates)
	if aggregateOnly {
		return
	}

	var misses int
	for _, c := range report.Cases {
		if len(c.Relevant) > 0 && (c.FirstRelevantRank == 0 || c.FirstRelevantRank > 5) {
			misses++
			fmt.Fprintf(w, "MISS %-28s expected=%v top=%v\n", c.ID, c.Relevant, trim(c.Top, 5))
		}
	}
	if misses == 0 {
		fmt.Fprintln(w, "recall@5 misses: none")
	}
}

func reportForOutput(report sreval.Report, aggregateOnly bool) any {
	if aggregateOnly {
		return sreval.Aggregate(report)
	}
	return report
}

func trim(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
