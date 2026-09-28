package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mhingston/skillrouter/internal/search"
)

type Case struct {
	ID       string   `json:"id"`
	Query    string   `json:"query"`
	Relevant []string `json:"relevant"`
	Tags     []string `json:"tags,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}

type Searcher interface {
	Search(query string, limit int) ([]search.Candidate, string, string, error)
}

type CaseResult struct {
	ID                string   `json:"id"`
	Query             string   `json:"query"`
	Relevant          []string `json:"relevant"`
	Top               []string `json:"top"`
	FirstRelevantRank int      `json:"first_relevant_rank,omitempty"`
	PredictedNoMatch  bool     `json:"predicted_no_match"`
	LatencyMS         float64  `json:"latency_ms"`
}

type Metrics struct {
	TotalCases       int     `json:"total_cases"`
	PositiveCases    int     `json:"positive_cases"`
	NoMatchCases     int     `json:"no_match_cases"`
	RecallAt1        float64 `json:"recall_at_1"`
	RecallAt3        float64 `json:"recall_at_3"`
	RecallAt5        float64 `json:"recall_at_5"`
	MRR              float64 `json:"mrr"`
	NoMatchPrecision float64 `json:"no_match_precision"`
	NoMatchRecall    float64 `json:"no_match_recall"`
	Abstentions      int     `json:"abstentions"`
	LatencyP50MS     float64 `json:"latency_p50_ms"`
	LatencyP95MS     float64 `json:"latency_p95_ms"`
	MeanCandidates   float64 `json:"mean_candidates"`
}

type Report struct {
	Metrics Metrics      `json:"metrics"`
	Cases   []CaseResult `json:"cases"`
}

func LoadJSONL(r io.Reader) ([]Case, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var cases []Case
	line := 0
	seen := map[string]bool{}
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var c Case
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("corpus line %d: %w", line, err)
		}
		if c.ID == "" || c.Query == "" {
			return nil, fmt.Errorf("corpus line %d: id and query are required", line)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("corpus line %d: duplicate id %q", line, c.ID)
		}
		seen[c.ID] = true
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("corpus is empty")
	}
	return cases, nil
}

func Run(s Searcher, cases []Case, limit int) (Report, error) {
	if limit < 5 {
		limit = 5
	}
	var report Report
	report.Cases = make([]CaseResult, 0, len(cases))
	var positive, noMatch int
	var hit1, hit3, hit5 int
	var reciprocalRank float64
	var trueNoMatch, falseNoMatch int
	var totalCandidates int
	latencies := make([]float64, 0, len(cases))

	for _, c := range cases {
		start := time.Now()
		candidates, _, _, err := s.Search(c.Query, limit)
		elapsed := time.Since(start)
		if err != nil {
			return Report{}, fmt.Errorf("case %s: %w", c.ID, err)
		}

		top := make([]string, len(candidates))
		for i, candidate := range candidates {
			top[i] = candidate.Name
		}
		totalCandidates += len(top)
		latencyMS := float64(elapsed.Microseconds()) / 1000
		latencies = append(latencies, latencyMS)

		result := CaseResult{
			ID:               c.ID,
			Query:            c.Query,
			Relevant:         append([]string(nil), c.Relevant...),
			Top:              top,
			PredictedNoMatch: len(top) == 0,
			LatencyMS:        latencyMS,
		}

		if len(c.Relevant) == 0 {
			noMatch++
			if len(top) == 0 {
				trueNoMatch++
			}
		} else {
			positive++
			rank := firstRelevantRank(top, c.Relevant)
			result.FirstRelevantRank = rank
			if rank > 0 {
				reciprocalRank += 1 / float64(rank)
				if rank <= 1 {
					hit1++
				}
				if rank <= 3 {
					hit3++
				}
				if rank <= 5 {
					hit5++
				}
			}
			if len(top) == 0 {
				falseNoMatch++
			}
		}
		report.Cases = append(report.Cases, result)
	}

	sort.Float64s(latencies)
	abstentions := trueNoMatch + falseNoMatch
	report.Metrics = Metrics{
		TotalCases:       len(cases),
		PositiveCases:    positive,
		NoMatchCases:     noMatch,
		RecallAt1:        ratio(hit1, positive),
		RecallAt3:        ratio(hit3, positive),
		RecallAt5:        ratio(hit5, positive),
		MRR:              ratioFloat(reciprocalRank, positive),
		NoMatchPrecision: ratio(trueNoMatch, abstentions),
		NoMatchRecall:    ratio(trueNoMatch, noMatch),
		Abstentions:      abstentions,
		LatencyP50MS:     percentile(latencies, 0.50),
		LatencyP95MS:     percentile(latencies, 0.95),
		MeanCandidates:   ratioFloat(float64(totalCandidates), len(cases)),
	}
	return report, nil
}

func firstRelevantRank(top, relevant []string) int {
	allowed := map[string]bool{}
	for _, name := range relevant {
		allowed[name] = true
	}
	for i, name := range top {
		if allowed[name] {
			return i + 1
		}
	}
	return 0
}

func ratio(num, denom int) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

func ratioFloat(num float64, denom int) float64 {
	if denom == 0 {
		return 0
	}
	return num / float64(denom)
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 1 {
		return values[len(values)-1]
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}
