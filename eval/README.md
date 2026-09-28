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

## Corpus design

The initial corpus is manually curated against the real `mhingston/agent-skills` catalogue and emphasises boundaries that are easy to confuse:

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

Do not tune retrieval against this corpus indefinitely. Once it starts driving implementation decisions, split out a protected holdout set or add newly observed production queries before changing thresholds/weights.
