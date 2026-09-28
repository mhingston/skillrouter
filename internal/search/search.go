package search

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/mhingston/skillrouter/internal/catalog"
)

var tokenRE = regexp.MustCompile(`[a-z0-9][a-z0-9_+.#/-]*`)

type Candidate struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Lexical     float64 `json:"lexical_score,omitempty"`
	Semantic    float64 `json:"semantic_score,omitempty"`
}

type Embedder interface {
	Embed(texts []string) ([][]float64, error)
	ID() string
}

type Index struct {
	snapshot      *catalog.Snapshot
	docs          []doc
	df            map[string]int
	embedder      Embedder
	vectors       map[string][]float64
	semanticOK    bool
	embeddingID   string
	semanticError string
}

type doc struct {
	name   string
	terms  map[string]float64
	length float64
}

func New(snapshot *catalog.Snapshot, embedder Embedder) (*Index, error) {
	i := &Index{snapshot: snapshot, df: map[string]int{}, embedder: embedder, vectors: map[string][]float64{}}
	for _, name := range snapshot.Names {
		s := snapshot.Skills[name]
		terms := weightedTerms(s)
		seen := map[string]bool{}
		length := 0.0
		for term, weight := range terms {
			length += weight
			if !seen[term] {
				i.df[term]++
				seen[term] = true
			}
		}
		i.docs = append(i.docs, doc{name: name, terms: terms, length: length})
	}
	if embedder != nil && len(snapshot.Names) > 0 {
		texts := make([]string, 0, len(snapshot.Names))
		for _, name := range snapshot.Names {
			s := snapshot.Skills[name]
			body := s.Body
			if len(body) > 12000 {
				body = body[:12000]
			}
			texts = append(texts, s.Name+"\n"+s.Description+"\n"+body)
		}
		vectors, err := embedder.Embed(texts)
		if err != nil {
			i.semanticError = err.Error()
		} else if len(vectors) == len(snapshot.Names) {
			for n, vector := range vectors {
				i.vectors[snapshot.Names[n]] = vector
			}
			i.semanticOK = true
			i.embeddingID = embedder.ID()
		}
	}
	return i, nil
}

func (i *Index) Search(query string, limit int) ([]Candidate, string, string, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	qTerms := tokenize(query)
	avgLen := 1.0
	if len(i.docs) > 0 {
		total := 0.0
		for _, d := range i.docs {
			total += d.length
		}
		avgLen = math.Max(total/float64(len(i.docs)), 1)
	}
	lex := map[string]float64{}
	maxLex := 0.0
	for _, d := range i.docs {
		score := 0.0
		for _, term := range qTerms {
			tf := d.terms[term]
			if tf == 0 {
				continue
			}
			df := float64(i.df[term])
			idf := math.Log(1 + (float64(len(i.docs))-df+0.5)/(df+0.5))
			const k1, b = 1.2, 0.75
			score += idf * (tf * (k1 + 1)) / (tf + k1*(1-b+b*d.length/avgLen))
		}
		lex[d.name] = score
		if score > maxLex {
			maxLex = score
		}
	}

	semantic := map[string]float64{}
	mode := "lexical"
	degraded := i.semanticError
	if i.semanticOK && i.embedder != nil {
		vectors, err := i.embedder.Embed([]string{query})
		if err == nil && len(vectors) == 1 {
			for name, vec := range i.vectors {
				semantic[name] = math.Max(0, cosine(vectors[0], vec))
			}
			mode = "hybrid"
			degraded = ""
		} else if err != nil {
			degraded = err.Error()
		}
	}

	results := make([]Candidate, 0, len(i.docs))
	for _, d := range i.docs {
		l := lex[d.name]
		ln := 0.0
		if maxLex > 0 {
			ln = l / maxLex
		}
		s := semantic[d.name]
		final := ln
		if mode == "hybrid" {
			final = 0.4*ln + 0.6*s
		}
		if final <= 0 {
			continue
		}
		skill := i.snapshot.Skills[d.name]
		results = append(results, Candidate{Name: d.name, Description: skill.Description, Score: round(final), Lexical: round(ln), Semantic: round(s)})
	}
	sort.SliceStable(results, func(a, b int) bool {
		if results[a].Score == results[b].Score {
			return results[a].Name < results[b].Name
		}
		return results[a].Score > results[b].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, mode, degraded, nil
}

func weightedTerms(s catalog.Skill) map[string]float64 {
	out := map[string]float64{}
	add := func(text string, weight float64) {
		for _, term := range tokenize(text) {
			out[term] += weight
		}
	}
	add(strings.ReplaceAll(s.Name, "-", " "), 5)
	add(s.Description, 2.5)
	body := s.Body
	if len(body) > 12000 {
		body = body[:12000]
	}
	add(body, 0.35)
	return out
}

func tokenize(text string) []string {
	return tokenRE.FindAllString(strings.ToLower(text), -1)
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	dot, na, nb := 0.0, 0.0, 0.0
	for idx := range a {
		dot += a[idx] * b[idx]
		na += a[idx] * a[idx]
		nb += b[idx] * b[idx]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func round(v float64) float64 { return math.Round(v*10000) / 10000 }

func (i *Index) SemanticEnabled() bool { return i.semanticOK }
func (i *Index) EmbeddingID() string   { return i.embeddingID }
func (i *Index) SemanticError() string { return i.semanticError }
