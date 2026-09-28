# SkillRouter

[![CI](https://github.com/mhingston/skillrouter/actions/workflows/ci.yml/badge.svg)](https://github.com/mhingston/skillrouter/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Keep large Agent Skill catalogues out of recurring prompt context. Retrieve only the skills a task actually needs.**

SkillRouter is a small, harness-agnostic MCP server for on-demand [Agent Skills](https://agentskills.io/) retrieval. Point it at a directory containing tens or hundreds of `SKILL.md` files, expose SkillRouter to your MCP client, and let the agent search the catalogue before loading one selected skill.

It is designed for Claude Code, Codex, Pi, Copilot, OpenCode, and other MCP-capable agent harnesses without making any of them part of SkillRouter's core.

```text
Without SkillRouter                         With SkillRouter

native skill discovery                      external skill catalogue
┌──────────────────────────┐                ┌──────────────────────────┐
│ skill A name/description │                │ A  B  C  ...  100s more │
│ skill B name/description │                └────────────┬─────────────┘
│ skill C name/description │                             │
│ ... every turn ...       │                       search_skills
└────────────┬─────────────┘                             │
             │                                    3–5 candidates
        model context                                      │
                                                     read_skill
                                                           │
                                                     one SKILL.md
                                                           │
                                                   read_resource
                                                           │
                                              only when actually needed
```

## Why SkillRouter?

Agent Skills already support progressive disclosure of a skill's full instructions, but a harness normally needs enough metadata to discover every installed skill. As the catalogue grows, those names and descriptions become recurring context overhead.

SkillRouter moves the catalogue behind MCP instead.

- **Constant native footprint.** Keep the wider catalogue outside harness-native skill directories.
- **Progressive disclosure.** Search first, load one skill second, load referenced files only when needed.
- **Harness agnostic.** No Claude/Codex/Pi/Copilot directory detection or host-specific adapters in the core.
- **Local by default.** Fast weighted lexical retrieval with no external service.
- **Semantic when useful.** Optional hybrid retrieval through any OpenAI-compatible embeddings endpoint.
- **Read-only.** SkillRouter does not execute skill scripts, modify skills, or install them into a harness.
- **Measurable.** A retrieval evaluation corpus is included so changes can be judged by evidence rather than intuition.

SkillRouter is intentionally a **runtime retrieval layer**, not a skill marketplace or governance platform.

## Quick start

### 1. Put your catalogue outside native skill discovery

This is the important part.

If the same 100 skills remain installed in a harness's native skill directory, that harness may still advertise all of their metadata and you will not get the full context-saving benefit.

A simple catalogue can look like:

```text
~/agent-skills/
  code-review/
    SKILL.md
    references/
      checklist.md
  plan/
    SKILL.md
  incident-investigation/
    SKILL.md
```

### 2. Build SkillRouter

Requires **Go 1.25+**.

```bash
git clone https://github.com/mhingston/skillrouter.git
cd skillrouter
go build -o skillrouter ./cmd/skillrouter
```

Or install directly with Go:

```bash
go install github.com/mhingston/skillrouter/cmd/skillrouter@latest
```

### 3. Add it as a stdio MCP server

The command is simply:

```bash
/path/to/skillrouter --skills-dir /path/to/agent-skills
```

Use that command in your MCP client's normal server configuration.

Many clients use a configuration shaped roughly like this:

```json
{
  "mcpServers": {
    "skillrouter": {
      "command": "/absolute/path/to/skillrouter",
      "args": [
        "--skills-dir",
        "/absolute/path/to/agent-skills"
      ]
    }
  }
}
```

The exact MCP configuration file or UI is owned by the client. SkillRouter itself does not need to know which harness launched it.

### 4. Optionally install one bootstrap skill

Some MCP clients surface server instructions/resources to the model more reliably than others. If your agent does not naturally discover SkillRouter, install only the bundled [`find-skills`](skills/find-skills/SKILL.md) skill using the harness's normal Agent Skills mechanism.

That gives you a constant native discovery footprint:

```text
native harness
     │
     └── find-skills              ← one small bootstrap skill
             │
             ▼
        SkillRouter MCP
             │
             ▼
      external catalogue          ← everything else
```

Do **not** copy dynamically retrieved skills back into the native skills directory unless you deliberately want them permanently installed.

### 5. Try it

Ask the agent for a task that should have a matching skill, for example:

> Review this pull request for correctness, security risks and missing tests.

The intended flow is:

```text
task
  ↓
search_skills("review this pull request...")
  ↓
compact candidates
  ↓
read_skill("review")
  ↓
skill instructions
  ↓
read_resource(...) only if required
```

You can also call the MCP tools directly from a client when debugging.

## MCP surface

| Tool | Purpose |
| --- | --- |
| `search_skills(query, limit?)` | Search the external catalogue and return compact ranked candidates. |
| `read_skill(name)` | Load one selected `SKILL.md` body and its resource index. |
| `read_resource(name, path)` | Read one exact resource belonging to the selected skill. |
| `health()` | Report catalogue size, collisions, retrieval mode, degradation and index drift. |
| `reindex()` | Rescan configured roots and rebuild the in-memory index. |

SkillRouter also exposes the small MCP resource:

```text
skillrouter://find-skills
```

It describes the same search → select → read workflow as the optional bootstrap skill.

## Multiple catalogues

Pass `--skills-dir` more than once:

```bash
skillrouter \
  --skills-dir ~/skills/company \
  --skills-dir ~/skills/personal
```

When two roots contain the same skill name, **the first configured root wins**. `health()` reports shadowed duplicates.

You can also use `SKILLROUTER_SKILLS_DIRS` with your platform's path separator:

```bash
export SKILLROUTER_SKILLS_DIRS="$HOME/skills/company:$HOME/skills/personal"
skillrouter
```

## Retrieval modes

### Local lexical retrieval

This is the default and requires no model or external service.

SkillRouter indexes:

- skill name;
- description;
- `when_to_use`, when present;
- a bounded excerpt of the skill body.

The index is built once and queried in memory.

```bash
skillrouter --skills-dir ~/agent-skills
```

### Hybrid semantic retrieval

For tasks whose wording differs substantially from the skill description, add any endpoint that implements the OpenAI embeddings request/response shape:

```bash
export SKILLROUTER_EMBEDDING_URL=http://localhost:11434/v1/embeddings
export SKILLROUTER_EMBEDDING_MODEL=nomic-embed-text

# Optional for authenticated endpoints:
export SKILLROUTER_EMBEDDING_API_KEY=...

skillrouter --skills-dir ~/agent-skills
```

This can be a local gateway or a hosted provider. SkillRouter has no provider-specific SDK dependency.

If semantic retrieval becomes unavailable, the runtime can fall back to lexical search and reports the degradation through its result/health metadata.

## Agent Skill format

SkillRouter recursively discovers standard `SKILL.md` files:

```markdown
---
name: code-review
description: Review code changes for correctness, maintainability and risk.
---

# Code review

Instructions...
```

Supporting files stay next to the skill:

```text
code-review/
  SKILL.md
  references/
    checklist.md
  scripts/
    inspect.py
```

`read_skill` returns the selected instructions plus a resource index. The agent can then request individual text resources with `read_resource`.

SkillRouter does **not** execute scripts. Skills can still instruct the host agent to use scripts through whatever filesystem/tool capabilities that host already provides.

## What SkillRouter does not do

SkillRouter deliberately avoids becoming another agent platform.

It does not:

- install skills into Claude, Codex, Pi, Copilot, OpenCode, or another harness;
- execute skill scripts or workflows;
- publish or host a marketplace;
- manage organisational approval/governance;
- orchestrate agents;
- score developers or collect developer telemetry.

Those concerns can sit above or beside SkillRouter without coupling the retrieval layer to one environment.

## Evaluation

Retrieval quality is exercised against a pinned real-world skill catalogue rather than a synthetic handful of examples.

The current development corpus contains **74 cases**:

- 62 positive routing cases, including deliberately lexically displaced prompts;
- 12 explicit no-match cases;
- difficult boundaries such as `review` vs `review-calibration`, the three memory skills, code research vs fault isolation, and ELI5 vs tutoring vs technical writing.

Current lexical development baseline:

| Metric | Result |
| --- | ---: |
| Recall@1 | 0.887 |
| Recall@3 | 0.919 |
| Recall@5 | 0.952 |
| MRR | 0.916 |

The evaluation also exposed an important current limitation: **the baseline retriever does not yet have a calibrated abstention mechanism**, so unrelated tasks can still receive candidates. This is tracked as a retrieval problem rather than hidden by the API.

See [`eval/README.md`](eval/README.md) for methodology, no-match metrics, latency measurements and how to run the benchmark.

```bash
go run ./cmd/skillrouter-eval \
  --skills-dir /path/to/agent-skills \
  --corpus eval/corpus.jsonl \
  --json-out eval-results.json
```

## Security model

SkillRouter treats the catalogue as data to retrieve, not code to execute.

- Skill roots are read-only.
- SkillRouter never executes scripts.
- Skill and resource reads are size bounded (`--max-file-bytes`, 1 MiB by default).
- Symlinked skill/resource files are ignored during discovery.
- Resource paths must already belong to the selected skill.
- Resource reads re-check containment and file identity after opening.
- Duplicate skill names are deterministic and observable through `health()`.
- Search metadata is explicitly treated as discovery metadata, not trusted instructions.

## How it differs from native skills

| | Native skill discovery | SkillRouter |
| --- | --- | --- |
| Catalogue metadata | Typically visible to the harness for discovery | Kept behind MCP |
| Full skill body | Loaded when selected | Loaded when selected |
| Resources | Host-specific / skill-relative | Explicit on-demand reads |
| Catalogue size cost | Usually grows with installed skills | Search result stays compact |
| Harness coupling | Installed into that harness | Generic stdio MCP server |
| External service required | No | No; semantic retrieval is optional |

Native skills are still useful. SkillRouter is for the point where **discovering all installed skills becomes the scaling problem**.

## How it differs from a registry such as Skillet

SkillRouter answers:

> Which skill is relevant right now, and what is the smallest amount of it the model needs?

A registry/control plane can additionally answer questions such as:

- Which revisions are approved?
- Who published this skill?
- Which teams may access it?
- How should skills be distributed or governed?

Those are complementary concerns. SkillRouter intentionally keeps the runtime retrieval path small.

## Development

Run the normal checks:

```bash
go mod tidy
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
```

Run the retrieval corpus:

```bash
go run ./cmd/skillrouter-eval \
  --skills-dir /path/to/agent-skills \
  --corpus eval/corpus.jsonl
```

## Project direction

The core contract stays intentionally small:

```text
search → read → selectively disclose resources
```

Likely future work should be driven by the evaluation corpus, particularly:

- calibrated abstention/no-match handling;
- semantic-retrieval comparison on a protected holdout set;
- persistent content-hash-keyed embedding cache if startup/reindex cost justifies it;
- configurable fusion/reranking only where it produces measured retrieval lift;
- incremental drift detection if explicit reindexing becomes a practical problem;
- standards-compatible skill discovery surfaces as the ecosystem stabilises.

The project should avoid adding harness-specific adapters or broad registry/orchestration responsibilities unless evidence shows they are necessary.

## Acknowledgements

SkillRouter is informed by:

- [`sowhan/skill-search`](https://github.com/sowhan/skill-search) — especially externalising richer skill descriptions and detecting index drift;
- [`tech-leads-club/agent-skills`](https://github.com/tech-leads-club/agent-skills) — especially the clean search → read → resource progressive-disclosure shape.

## License

[MIT](LICENSE)
