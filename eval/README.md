# Retrieval evaluation

This directory contains a harness-agnostic retrieval corpus for SkillRouter.

A case is deliberately small:

```json
{"id":"review-paraphrase-1","query":"Stress-test this pull request ...","relevant":["review"],"tags":["paraphrase"]}
```

- `query` is the task intent presented to the retriever.
- `relevant` contains all acceptable skill names. An empty array is an explicit no-match case.
- `tags` describe difficulty/boundary groups for later slicing; the core metrics do not depend on them.
- The corpus does not mention Claude, Codex, Pi, Copilot or another harness.

## What it measures

The evaluator reports:

- Recall@1 / Recall@3 / Recall@5 across positive cases.
- Mean reciprocal rank (MRR).
- No-match precision: among cases where the retriever abstains, how often abstention was correct.
- No-match recall: how many labelled no-match cases the retriever actually abstained on.
- p50 / p95 in-process query latency.
- Mean returned candidate count.
- Per-case Recall@5 misses.

No-match metrics intentionally use **zero returned candidates** as the current abstention contract. If SkillRouter later introduces an explicit threshold or abstention score, the corpus can stay unchanged while the evaluator adapts.

## Development and protected sets

The evaluation workflow uses two deliberately different sets:

| Set | Purpose | When it runs | Reporting |
| --- | --- | --- | --- |
| `corpus.jsonl` | Development and diagnosis | Pull requests and local iteration | Full per-case rankings and misses |
| `holdout.jsonl` | Final comparison after choosing a candidate | After merge to `main` or an explicitly approved dispatch | Aggregate metrics only |

The protected holdout contains 12 separately authored cases: eight positive routing cases and four no-match cases. Its IDs are opaque, it has no exact query or ID overlap with the development corpus, and its content hash, size, catalogue revision, reporting policy, and owner are pinned in `holdout-manifest.json`.

The holdout owner is **@mhingston**. `CODEOWNERS` routes changes to the corpus, manifest, reporting code, and workflow to that owner. Repository branch protection should require Code Owner approval, and the `protected-evaluation` GitHub environment should require owner approval before manual runs.

Use the holdout only after implementation choices have been made from development evidence. Do not use individual holdout failures to tune weights, queries, embeddings, or future abstention thresholds. If case-level data is exposed during debugging, rotate the affected cases and update the manifest hash.

This is leakage-resistant process protection, not secrecy: the repository is public, so the corpus is inspectable. The controls prevent routine PR output and artifacts from turning the holdout into another tuning set. If confidentiality is required later, keep the corpus in access-controlled storage and inject it into the same aggregate-only workflow.

## Corpus design

The corpus is manually curated against the real `mhingston/agent-skills` catalogue and emphasises boundaries that are easy to confuse:

- `review` vs `review-calibration`
- `codebase-walkthrough` vs `project-context` vs `repository-ontology`
- `fault-isolation` vs `code-research`
- `agent-workflow-design` vs `dynamic-workflows`
- `memory-capture` / `memory-recall` / `memory-maintenance`
- `wrap-up` vs `session-lessons`
- `eli5` vs `teach-me` vs `technical-plain-english` vs `technical-diagram`
- contributor/repository-history and engineering-attention/evidence pairs

It also includes unrelated no-match tasks so a router cannot score well merely by always returning something.

The CI workflow pins the evaluated skill catalogue to commit:

`26a0da162e69af259ecb22813a960b1188a4519f`

This prevents catalogue drift from being mistaken for a retrieval change.

## Current lexical baseline

Against the pinned catalogue, the 74-case corpus currently produces:

| Metric | Result |
| --- | ---: |
| Positive cases | 62 |
| No-match cases | 12 |
| Recall@1 | 0.887 |
| Recall@3 | 0.919 |
| Recall@5 | 0.952 |
| MRR | 0.916 |
| No-match precision | 0.000 |
| No-match recall | 0.000 |
| Abstentions | 0 |
| p50 search latency | 0.067 ms* |
| p95 search latency | 0.092 ms* |

\*Latency is an observed GitHub Actions sample, not a portability or release gate.

The three Recall@5 misses are deliberately lexically displaced cases for
`code-research`, `agent-readiness`, and `memory-recall`.

The baseline shows two distinct gaps:

1. **Semantic recall:** lexical retrieval is strong on direct/paraphrased tasks but misses some low-overlap intents.
2. **Abstention:** the current retriever always fills the requested candidate limit, even for unrelated tasks. A future abstention mechanism must be calibrated against both positive and no-match cases rather than added as an arbitrary score threshold.

Do not turn these numbers into release gates yet. In particular, the holdout records the current zero-abstention behavior; it does not define or implement an abstention threshold.

The current aggregate-only lexical holdout snapshot is:

| Metric | Result |
| --- | ---: |
| Positive cases | 8 |
| No-match cases | 4 |
| Recall@1 | 0.750 |
| Recall@3 | 0.875 |
| Recall@5 | 1.000 |
| MRR | 0.844 |
| No-match precision | 0.000 |
| No-match recall | 0.000 |
| Abstentions | 0 |

Only aggregate metrics are retained; case-level holdout diagnostics are not published.

## Run locally

```bash
go run ./cmd/skillrouter-eval \
  --skills-dir /path/to/agent-skills \
  --corpus eval/corpus.jsonl \
  --json-out eval-results.json
```

The same runner can exercise hybrid semantic retrieval:

```bash
SKILLROUTER_EMBEDDING_URL=http://localhost:11434/v1/embeddings \
SKILLROUTER_EMBEDDING_MODEL=nomic-embed-text \
go run ./cmd/skillrouter-eval \
  --skills-dir /path/to/agent-skills \
  --corpus eval/corpus.jsonl
```

Use the development corpus for iteration, and add newly observed production queries there before changing thresholds or weights. Reserve the protected set for final comparisons.

Run the holdout locally only for a final comparison, and keep its output aggregate-only:

```bash
go run ./cmd/skillrouter-eval \
  --skills-dir /path/to/agent-skills \
  --corpus eval/holdout.jsonl \
  --aggregate-only \
  --json-out holdout-results.json
```

## Harness-specific task text

The evaluator and routing contract are harness agnostic. Individual catalogue skills may still target a particular tool or harness (for example the `lsp-config` case mentions Copilot CLI because that is the skill's domain). Such task text does not make the evaluator or SkillRouter integration harness-specific; it is simply content being routed.
