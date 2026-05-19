# Index Plan Feature Test Report

- **Date:** 2026-05-19
- **Binary:** `dist/aascribe` built from branch `refactor/agentcli-core-phase1` (commit `1ec7464`)
- **Build command:** `make all`
- **Tester:** manual session, help-driven (no source code read)

## Goal

Exercise the `aascribe index` plan workflow end-to-end, then evaluate whether the generated `.aascribe_index_meta.json` actually helps retrieval — comparing against plain `grep`/`rg` in two regimes:

1. **No-summary plan** — `builtin.classify.v1` + `builtin.routing.v1` (heuristic only, no LLM).
2. **Summary-enabled plan** — adds `summary` (LLM-backed via Gemini).

## Test Folder

| Item | Value |
|---|---|
| Source | `~/code/km` |
| Test copy | `/tmp/aascribe-km-test` (copied via `cp -R`) |
| Indexed subfolder | `./coding` (7 markdown files across 4 folders) |
| Files | `coding/README.md`, `coding/claude-code/{CLAUDE,README}.md`, `coding/editor/README.md`, `coding/editor/zed/{README,install_wsl}.md`, `coding/tools/README.md` |
| Store reused from source | `data/memory/` (already initialized in km) |
| Gemini key | Already present in `data/config/.env` (carried by `cp -R`) |

## Test Steps

### Step 0 — Build

```
make all
```

Result: `dist/aascribe` produced cleanly.

### Step 1 — Discover the CLI surface from help only

```
./dist/aascribe --help
./dist/aascribe index --help
./dist/aascribe index plan --help
./dist/aascribe index plan validate --help
./dist/aascribe index methods
```

Findings:
- Help docs are agent-oriented and include a "Plan Verification Workflow For AI Agents" section under `index plan --help`. Easy to follow without reading source.
- `index methods` lists four methods: `base` (always on), `summary`, `builtin.classify.v1`, `builtin.routing.v1`.
- Minor doc nit: `aascribe search --help` instructs the agent to "Use `aascribe map` for semantic routing", but `map` is not a registered command. Probably a stale forward reference to either `index` (metadata-as-routing) or a planned command.

### Step 2 — Plan A: no-summary plan

Plan file `/tmp/aascribe-km-test/aascribe.index.toml`:

```toml
version = "index-plan-v1"
[methods]
enabled = [
  "builtin.classify.v1",
  "builtin.routing.v1"
]
```

```
aascribe index plan validate ./aascribe.index.toml
# -> index plan valid
# -> digest: sha256:bc2d862c...

aascribe index --index-plan ./aascribe.index.toml ./coding
```

Result:
- Run finished in **~5 ms**, no LLM calls, no tokens.
- 5 `.aascribe_index_meta.json` files written (one per folder in the tree).
- Indexing report: `data/report/2026-05-19-020614-indexing-report.md`. Report's `index_plan_digest` matched the validate step — good integrity property.
- Per-file `builtin.classify.v1` fields: `category="docs"`, `confidence=0.86`, tags from filename + path (e.g. `claude-code`, `claude.md`, `docs`, `md`).
- Per-folder `builtin.routing.v1` aggregates `category_counts`, `tag_counts`, `dominant_category`. Top-level `coding/` correctly reports `category_counts: {docs: 7}` even though `direct_file_count: 1` — descendant aggregation works.

### Step 3 — Retrieval Scenario A (known keyword)

Query: `"WSL"`.

```
rg -l "WSL" /tmp/aascribe-km-test/coding
# -> coding/editor/zed/install_wsl.md   (~21 ms)

# metadata-only via jq across all 5 meta files
# -> coding/editor/zed/install_wsl.md   (~43 ms)
```

Result: tie. Both find the same file. Metadata routing via filename-derived tag matches grep here.

### Step 4 — Retrieval Scenario B (semantic intent, no obvious keyword)

Query intent: *"set up a programming editor on Linux"*. The relevant file (`install_wsl.md`) never contains the literal word "Linux" — author used the jargon "WSL".

```
rg -li "linux"  -> 0 files
rg -li "ubuntu" -> 0 files
rg -li "install" -> 2 files (install_wsl.md + tools/README.md — false positive)
rg -li "editor" -> 3 files (broad)
```

Metadata routing (`category=docs AND tag=editor`) returned the 3-file shortlist including `install_wsl.md` — without any vocabulary guess.

**Observation:** narrowing-by-tag worked, but those tags (`editor`, `zed`) are folder/filename-derived, so `find -path '*editor*'` reproduces the same shortlist for free. The heuristic plan is largely redundant with naming conventions on this corpus.

### Step 5 — Plan B: enable `summary`

Updated plan:

```toml
version = "index-plan-v1"
[methods]
enabled = [
  "summary",
  "builtin.classify.v1",
  "builtin.routing.v1"
]
```

```
aascribe index plan validate ./aascribe.index.toml
# -> digest: sha256:124083be...

aascribe index --refresh --index-plan ./aascribe.index.toml ./coding
```

Result:
- Run took **~9 s** (vs. 5 ms).
- Provider `gemini`, model `gemini-2.5-flash`.
- Token usage: 1,786 prompt + 722 output + ~3,069 thinking = **5,577 total** across 7 files.
- All 7 files got prose `summary` fields. Folder routing recomputed with `summarized_files: 7`.

### Step 6 — Retrieval Scenario B revisited with summaries

Substring search over each file's `summary` field (jq) vs `rg` over raw files:

| Query | summary hits | rg hits | Winner |
|---|---:|---:|---|
| `Linux` | 1 | 0 | summary |
| `Windows Subsystem` | 1 | 0 | summary |
| `WSL` | 1 | 1 | tie |
| `shell prompt` | 0 | 2 | rg |
| `HTTP client` | 0 | 2 | rg |
| `directory navigation` | 0 | 0 | both miss |
| `fuzzy jump` | 0 | 0 | both miss |
| `modern Unix tools` | 0 | 0 | both miss |
| `tab completion` | 0 | 0 | both miss |
| `API client` | 0 | 0 | both miss |

Net: summary won 2 queries that grep failed on; lost 2 that grep handled. Five queries neither approach answered.

## Why summary loses in some cases (root causes)

Three distinct failure modes, each with concrete evidence:

### Mode 1 — Compression dropped the descriptor

| Field | Content |
|---|---|
| Raw (CLAUDE.md L39) | `**starship** - Cross-shell prompt` |
| Summary | "...lists other available utilities like `zoxide`, `starship`, and `carapace`..." |

The LLM kept the tool **name** but rolled four bullets up into a single categorical phrase, deleting each tool's role descriptor. Query "shell prompt" therefore matches the raw line but not the summary.

### Mode 2 — Synonym substitution

| Field | Content |
|---|---|
| Raw (CLAUDE.md L37) | `**xh** - HTTP client` |
| Summary | "...`xh` for HTTP requests instead of `curl`..." |

Same concept, different exact substring. Substring search on summary fails; grep on raw succeeds.

### Mode 3 — Neither layer uses the user's vocab

| Field | Content |
|---|---|
| Raw | `**zoxide** - Smarter cd command` |
| Summary | "...utilities like `zoxide`..." |
| User query | "directory navigation" / "fuzzy jump" |

Nobody on either side wrote those phrases. Substring search across either layer cannot bridge this. Only true semantic search (vector embeddings) would.

### Why summary wins on `Linux` / `Windows Subsystem`

| Field | Content |
|---|---|
| Raw (install_wsl.md) | "Install Zed on WSL" (acronym only) |
| Summary | "...within a **Windows Subsystem for Linux (WSL)** environment..." |

The LLM has world knowledge and **expanded the acronym**. That is the narrow class of wins: acronym/abbreviation expansion where the LLM knows the long form.

## Underlying mismatch

The `summary` method's prompt asks for **a summary**, not **a search index**. A summary optimizes for conciseness and readability; a retrieval index optimizes for vocabulary coverage (synonyms, expansions, category labels, alternative phrasings). Substring search on prose summaries is a poor proxy for semantic search.

## Recommendations

| # | Effort | Coverage gain | Notes |
|---|---|---|---|
| 1. Union: query summary + raw, take the union | Trivial (just the agent doing both) | Closes Mode 1 and Mode 2; no help for Mode 3 | No code change needed |
| 2. Add a retrieval-oriented prompt: have `summary` also emit a `keywords`/`aliases` field with synonyms, category labels, and acronym expansions | Low — prompt change in the `summary` method | Closes Mode 1, Mode 2, and most of Mode 3 at small extra token cost | Best bang-for-buck; recommended as the next iteration |
| 3. Add a `builtin.embeddings.v1` method: embed `summary` + raw, store vectors, support cosine-similarity query | Medium — new method + vector store + query path | Closes all three modes; enables ranking | Right long-term shape |

## Other observations

- **Plan integrity is solid.** Validate-step digest matches the run-report digest end-to-end. Good determinism property.
- **Hash-based staleness already works.** `content_hash` is recorded per file. The `--refresh` flag is required to override; without it, re-indexing a clean tree would short-circuit.
- **Cost projection.** ~5,577 tokens / 9 s for 7 short markdown files (~4 KB total content). Scaled linearly that's ~800 tokens/file. A 5,000-file repo would be ~4 M tokens and ~100 min at this concurrency (4). Either acceptable for one-shot indexing or worth raising `--concurrency`.
- **`aascribe map` ghost.** `aascribe search --help` references a `map` command that doesn't exist. Recommend either implementing it or fixing the help text to point at the metadata file directly.

## Verdict

The index plan feature **works as documented**:

- Plan validation, digest propagation, method discovery, dry-run via `index eval`, and run-with-plan all behave correctly.
- The no-summary plan is useful as a folder-routing/staleness layer but mostly redundant with naming conventions on this corpus.
- The summary-enabled plan adds genuine vocabulary-bridging value (acronym expansion), but plain prose summaries optimized for human reading are an imperfect substitute for a true retrieval index. A small prompt-level change (emit retrieval-oriented `keywords`/`aliases` alongside the summary) would meaningfully close the gap before any need for vector search.
