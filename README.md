# SkillRouter

A small, harness-agnostic MCP server for **on-demand Agent Skill retrieval**.

SkillRouter is designed for the case where you have tens or hundreds of `SKILL.md` files but do **not** want every skill's frontmatter advertised in an agent's prompt on every turn. Keep the catalogue outside Claude Code, Codex, Pi, Copilot, OpenCode, or any other harness-native skill directory; expose one MCP server instead, then retrieve only what the task needs.

```text
                         external catalogue
                       100s of Agent Skills
                               │
                               ▼
                         ┌─────────────┐
                         │ SkillRouter │
                         └──────┬──────┘
                                │ search_skills
                                ▼
                         compact candidates
                                │ read_skill
                                ▼
                          one SKILL.md
                                │ read_resource
                                ▼
                    only referenced files needed
```

## Why

Native Agent Skills usually provide progressive disclosure for skill **bodies**, but the harness still needs enough metadata to discover installed skills. At catalogue scale that metadata becomes recurring context overhead.

SkillRouter moves discovery outside the harness:

- **No harness adapters.** Skill roots are explicit configuration, not `~/.claude`, `~/.codex`, `~/.pi`, etc.
- **No native installation.** Search/read operations never copy skills into a harness directory and therefore do not gradually recreate the metadata tax.
- **Progressive disclosure.** `search_skills` returns compact metadata, `read_skill` returns one selected body, and `read_resource` returns one exact resource.
- **Local by default.** Built-in weighted BM25-style retrieval has no service dependency.
- **Optional semantic retrieval.** Point SkillRouter at any OpenAI-compatible embeddings endpoint (local or remote) to enable hybrid lexical + semantic search.
- **Drift-aware.** `health` reports when the on-disk catalogue changed after indexing; `reindex` refreshes it explicitly.
- **Read-only and bounded.** Skill files are never modified or executed. Resource reads are restricted to discovered files within the selected skill directory.

This is intentionally narrower than a skill registry/control plane such as Skillet: no governance, publishing, installation, workflow execution, or lifecycle management.

## MCP tools

| Tool | Purpose |
| --- | --- |
| `search_skills(query, limit?)` | Return up to 20 compact candidates ranked for the task. |
| `read_skill(name)` | Return one selected skill's instructions and its resource index. |
| `read_resource(name, path)` | Read one exact resource previously discovered for that skill. |
| `health()` | Report skill count, collisions, retrieval mode, and index drift. |
| `reindex()` | Rescan configured roots and rebuild the in-memory index. |

SkillRouter also exposes `skillrouter://find-skills`, a small MCP resource describing the discovery workflow.

## Quick start

Requires Go 1.25+.

```bash
git clone https://github.com/mhingston/skillrouter.git
cd skillrouter
go build -o skillrouter ./cmd/skillrouter

./skillrouter --skills-dir /path/to/agent-skills
```

Multiple roots are supported and **root order defines precedence** when two skills have the same name:

```bash
./skillrouter \
  --skills-dir ~/skills/company \
  --skills-dir ~/skills/personal
```

You can also configure roots with the platform path separator:

```bash
export SKILLROUTER_SKILLS_DIRS="$HOME/skills/company:$HOME/skills/personal"
```

Then configure the binary as a normal **stdio MCP server** in any MCP-capable harness. The exact client config syntax differs, but the command is always equivalent to:

```text
/path/to/skillrouter --skills-dir /path/to/agent-skills
```

There is deliberately no Claude/Codex/Pi/Copilot-specific setup logic inside SkillRouter.

## Semantic retrieval

The default local retriever is deterministic weighted lexical search over skill name, description and a bounded body excerpt. To enable hybrid semantic retrieval, configure any endpoint implementing the OpenAI embeddings request/response shape:

```bash
export SKILLROUTER_EMBEDDING_URL=http://localhost:11434/v1/embeddings
export SKILLROUTER_EMBEDDING_MODEL=nomic-embed-text
# optional for authenticated endpoints:
export SKILLROUTER_EMBEDDING_API_KEY=...

./skillrouter --skills-dir ~/skills
```

This works with OpenAI-compatible local gateways as well as hosted providers; SkillRouter has no provider-specific client SDK.

If embedding initialization or a later query fails, retrieval falls back to lexical mode rather than making the catalogue unavailable. `health` reports whether hybrid search is active.

## Catalogue layout

SkillRouter recursively discovers standard Agent Skills:

```text
skills/
  code-review/
    SKILL.md
    references/
      checklist.md
    scripts/
      inspect.py
  plan/
    SKILL.md
```

A skill needs YAML frontmatter:

```markdown
---
name: code-review
description: Review code changes for correctness, maintainability and risk.
---

# Code review

Instructions...
```

`when_to_use` is also included in retrieval text when present.

## Retrieval design

The first slice deliberately avoids a heavyweight vector database. At the expected personal/team catalogue sizes, embeddings can remain in memory:

1. Parse and deduplicate skills once at startup.
2. Build a weighted lexical index locally.
3. If an embeddings endpoint is configured, embed each skill once and keep vectors in memory.
4. Search with lexical BM25-style scoring and optional cosine semantic scoring.
5. Combine scores as 40% lexical / 60% semantic.
6. Return only compact candidate metadata.

This keeps the runtime small and makes the retrieval provider replaceable. A persistent embedding cache can be added later if measurements show reindex latency warrants it.

## Security boundaries

- SkillRouter never executes skill scripts.
- It never writes to skill roots.
- Symlinked skill/resource files are ignored during discovery.
- `read_resource` only accepts a relative path that was already discovered under the selected skill.
- Skill and resource files are bounded by `--max-file-bytes` (1 MiB by default).
- Duplicate names are deterministic: the first configured root wins, and `health` reports all shadowed paths.

## Direction

Keep SkillRouter focused on the runtime retrieval problem:

```text
search -> read -> selectively disclose resources
```

Good future slices, if justified by evidence:

- persistent content-hash keyed embedding cache;
- retrieval evaluation corpus (`recall@k`, MRR, no-match precision);
- configurable hybrid weighting / reciprocal-rank fusion;
- incremental file watching rather than explicit reindex;
- optional reranking behind a provider-neutral interface;
- SEP-2640-compatible skill discovery surfaces as the MCP proposal stabilises.

Out of scope unless the project direction changes: skill marketplaces, publishing, installation into harness directories, workflow orchestration, governance, telemetry about individual developers, or harness-specific adapters.

## Acknowledgements

The design is informed by the retrieval-first ideas in [`sowhan/skill-search`](https://github.com/sowhan/skill-search), especially keeping full descriptions outside the recurring prompt and detecting index drift, and by the `search -> read -> resource` progressive-disclosure pattern used by [`tech-leads-club/agent-skills`](https://github.com/tech-leads-club/agent-skills).

## License

MIT
