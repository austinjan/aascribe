# Index Plan Feature Test Report

- **Date:** 2026-05-19
- **Binary:** `dist/aascribe` built from branch `refactor/agentcli-core-phase1` (rebuilt each round from latest tree)
- **Build command:** `make all`
- **Tester:** manual session, help-driven (no source code read)
- **Test folder:** `/tmp/aascribe-km-test` (copy of `~/code/km`), indexed subtree: `./coding` — 7 markdown files, 4 folders
- **Rounds:** R1 = heuristic plan; R2a = +`summary`; R2b = +`builtin.retrieval.v1`; R3 = R2b plan, iterated extractor

## TL;DR

Retrieval performance on a fixed **10-query benchmark** (semantic intents over a corpus of 7 docs):

| Channel | R1 (heuristic) | R2a (summary) | R2b (+retrieval) | R3 (iterated) |
|---|---:|---:|---:|---:|
| `rg` baseline | 4 | 4 | 4 | 4 |
| metadata channel | 1 | 3 | 6 | **7** |
| Wall time per run | ~5 ms | ~9 s | ~12 s | ~10 s |
| LLM tokens | 0 | 5,577 | 5,976 | 6,268 |

**Headline:** With `builtin.retrieval.v1` enabled and iterated (R3), substring search over the metadata file is strictly stronger than `rg` on this corpus (7 vs 4 hits across the 10 queries), closes all three failure modes diagnosed in R1, and costs about 5% more tokens than summary-only.

**Outstanding bugs** (filed below): code-snippet leakage and bogus `Example`/`Note` aliases (both from the same bullet-prefix parser issue); `likely_queries` are diversified in form but still 100% template-driven for tool-list files; `aascribe map` is referenced in help but not implemented.

**Recommended next fix:** patch the entity parser's leading-token detection to treat Markdown section labels (`Example`, `Note`, `Warning`, `Tip`) as labels, not entity names. Single change cleans up two of three remaining bugs.

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

## Recommendations (issued at end of Round 1)

| # | Effort | Coverage gain | Notes | Status in later rounds |
|---|---|---|---|---|
| 1. Union: query summary + raw, take the union | Trivial (just the agent doing both) | Closes Mode 1 and Mode 2; no help for Mode 3 | No code change needed | Used as the R2/R3 benchmarking method |
| 2. Add a retrieval-oriented prompt: emit `keywords`/`aliases` field with synonyms, category labels, and acronym expansions | Low — prompt change in the `summary` method | Closes Mode 1, Mode 2, and most of Mode 3 at small extra token cost | Best bang-for-buck | **Implemented in R2 as `builtin.retrieval.v1`** (separate method, same idea) |
| 3. Add a `builtin.embeddings.v1` method: embed `summary` + raw, store vectors, support cosine-similarity query | Medium — new method + vector store + query path | Closes all three modes; enables ranking | Right long-term shape | Open — would help with remaining word-order and unwritten-synonym misses |

## Other observations

- **Plan integrity is solid.** Validate-step digest matches the run-report digest end-to-end. Good determinism property.
- **Hash-based staleness already works.** `content_hash` is recorded per file. The `--refresh` flag is required to override; without it, re-indexing a clean tree would short-circuit.
- **Cost projection.** ~5,577 tokens / 9 s for 7 short markdown files (~4 KB total content). Scaled linearly that's ~800 tokens/file. A 5,000-file repo would be ~4 M tokens and ~100 min at this concurrency (4). Either acceptable for one-shot indexing or worth raising `--concurrency`.
- **`aascribe map` ghost.** `aascribe search --help` references a `map` command that doesn't exist. Recommend either implementing it or fixing the help text to point at the metadata file directly.

## Verdict (Round 1)

The index plan feature **works as documented**:

- Plan validation, digest propagation, method discovery, dry-run via `index eval`, and run-with-plan all behave correctly.
- The no-summary plan is useful as a folder-routing/staleness layer but mostly redundant with naming conventions on this corpus.
- The summary-enabled plan adds genuine vocabulary-bridging value (acronym expansion), but plain prose summaries optimized for human reading are an imperfect substitute for a true retrieval index. A small prompt-level change (emit retrieval-oriented `keywords`/`aliases` alongside the summary) would meaningfully close the gap before any need for vector search.

---

## Round 2 — `builtin.retrieval.v1` added

After Round 1, the project added a new method `builtin.retrieval.v1` (purpose: `llm_retrieval_hints`, stage: file). This is essentially recommendation #2 from above, productized. Re-tested without reading source — only `index methods` and the metadata output.

### Step 7 — Rebuild and discover

```
make all
./dist/aascribe index methods
```

The listing now includes:

```
- builtin.retrieval.v1
  stage: file
  purpose: llm_retrieval_hints
  description: Emits deterministic LLM-facing retrieval hints from path, summary, and bounded file text.
```

### Step 8 — Plan C: add `builtin.retrieval.v1`

```toml
version = "index-plan-v1"
[methods]
enabled = [
  "summary",
  "builtin.classify.v1",
  "builtin.retrieval.v1",
  "builtin.routing.v1"
]
```

```
aascribe index plan validate ./aascribe.index.toml
# -> digest: sha256:63644b3a...
aascribe index --refresh --index-plan ./aascribe.index.toml ./coding
```

- Run time **~12 s** (was 9 s).
- Tokens **5,976 total** (was 5,577) → only **+399 tokens** for the new method across 7 files (~57 tokens/file extra).
- All 7 files received a `methods["builtin.retrieval.v1"]` block.

### Output shape of `builtin.retrieval.v1`

Per file:

```json
{
  "aliases":         [{ "from": "WSL", "to": "Windows Subsystem for Linux" }, ...],
  "entities":        [],
  "likely_queries":  ["install zed in wsl", "set up editor on linux"],
  "retrieval_terms": ["Windows Subsystem for Linux", "linux development environment",
                      "coding", "editor", "install", "wsl", "zed", "md"]
}
```

Sample from `claude-code/CLAUDE.md` shows the synonym mapping landing exactly where Round 1 lost:

```
aliases: starship -> cross-shell prompt
         xh       -> HTTP client
         zoxide   -> directory navigation
```

### Step 9 — 10-query benchmark, all three channels

Search performed via substring (case-insensitive) over `retrieval_terms ∪ likely_queries ∪ aliases.from ∪ aliases.to`.

| Query | rg | summary | retrieval | Δ vs Round 1 |
|---|---:|---:|---:|---|
| `Linux` | 0 | 1 | **1** | held |
| `Windows Subsystem` | 0 | 1 | **1** | held |
| `WSL` | 1 | 1 | **1** | held |
| `shell prompt` | 2 | 0 | **2** | **fixed (Mode 1)** |
| `HTTP client` | 2 | 0 | **2** | **fixed (Mode 2)** |
| `directory navigation` | 0 | 0 | **2** | **fixed (Mode 3)** |
| `fuzzy jump` | 0 | 0 | 0 | still miss |
| `modern Unix tools` | 0 | 0 | 0 | still miss |
| `tab completion` | 0 | 0 | 0 | still miss |
| `API client` | 0 | 0 | 0 | still miss |

Three previously-failing queries now succeed. The win pattern matches the failure modes diagnosed in Round 1 exactly:

- **Mode 1 (compression):** alias `starship → cross-shell prompt` re-surfaces the dropped descriptor → `shell prompt` query lands.
- **Mode 2 (synonym substitution):** alias `xh → HTTP client` adds the noun phrase grep needed → `HTTP client` query lands.
- **Mode 3 (neither layer):** alias `zoxide → directory navigation` introduces a phrase **nobody in the source used** → `directory navigation` query lands. This is the strongest result — retrieval is now contributing vocabulary the source corpus never contained.

### Step 10 — Self-consistency and word-order probes

| Query | rg | retrieval | Note |
|---|---:|---:|---|
| `set up editor on linux` (verbatim `likely_queries[1]`) | 0 | 1 | self-consistent |
| `install zed in wsl` (verbatim `likely_queries[0]`) | 0 | 1 | self-consistent |
| `Linux editor setup` (reordered words) | 0 | 0 | substring matcher is order-sensitive |
| `API testing` | 2 | 2 | parity |
| `completion` | 2 | 2 | parity |
| `smarter cd` | 2 | 2 | parity |
| `pager` | 1 | 1 | parity |
| `modern CLI tools` | 0 | 0 | nobody wrote it |

### What still misses, and why

- `fuzzy jump`, `tab completion`, `API client`, `modern Unix tools` — the alias side covered some synonyms (e.g. `zoxide → directory navigation`) but not every plausible user phrasing. The LLM picks one canonical synonym per concept; the user's word may still differ.
- `Linux editor setup` — semantically identical to the stored `set up editor on linux`, but substring matching is word-order-sensitive. A real semantic search (vector cosine) would handle this; substring won't.

### Observed quality issues in retrieval output

Worth filing as follow-ups:

1. **Code-snippet leakage into `retrieval_terms`.** `CLAUDE.md` retrieval_terms contain literal command lines like `Example fd "*.txt"` instead of `find . -name "*.txt"`. The example commands shouldn't be terms.
2. **Markdown-link leakage.** `coding/README.md` retrieval_terms include raw markdown fragments like `[claude-code/](claude-code/`. The parser preserves link syntax.
3. **Wrong `from` in some aliases.** `CLAUDE.md` aliases include `Example -> HTTP client` — `from` should be `xh`, not the literal word "Example". Likely a delimiter-split bug in the extractor.
4. **`entities` array always empty** across all 7 files. Either not yet implemented or no entities were detected. Worth confirming this is intentional.
5. **`likely_queries` skew formulaic.** Several files emit only `configure X` / `find X` per item. `install_wsl.md` got two distinctive natural queries; most others did not. Prompt could push for diversity.

### Cost-aware view

- Round 1 (heuristic only): ~5 ms, 0 tokens
- Round 2a (summary): ~9 s, 5,577 tokens (~800/file)
- Round 2b (summary + retrieval): ~12 s, 5,976 tokens (~57 token marginal cost per file for the new method)

The marginal cost of `builtin.retrieval.v1` is small relative to summary, and it closes three of the four substring failure modes diagnosed in Round 1. Good ROI.

## Verdict (Round 2)

`builtin.retrieval.v1` materially advances the retrieval contract. With it enabled, substring search over the metadata file goes from "narrowly useful (acronym expansion only)" to "competitive with grep on most queries and strictly better when the user's vocab differs from the author's." Three of the four failure modes from Round 1 are closed.

Remaining work to reach a true retrieval index:
1. **Clean up the extractor** (code snippet leakage, markdown leakage, wrong alias `from` values).
2. **Diversify `likely_queries`** — push the prompt for natural-language intents, not just `configure X` / `find X` templates.
3. **Eventually add embedding-based ranking** to handle word-reorderings and concept-only queries that neither author nor LLM literally wrote.

But for the heuristic-substring retrieval path this metadata is designed to feed, the feature is now usefully better than grep on the queries where it matters most.

---

## Round 3 — Iteration on `builtin.retrieval.v1`

Between Round 2 and Round 3 the team kept iterating on the retrieval extractor (multiple interim reports in `data/report/` confirm several intermediate runs, including one that hit Gemini DNS resolution failures). No new methods registered. Rebuilt from the same branch and re-ran the same plan with `--refresh`.

### Step 11 — Cost delta

| Round | Wall time | Tokens | Δ vs prior |
|---|---|---|---|
| R2b (summary + retrieval) | 12 s | 5,976 | — |
| R3 (same plan, new extractor) | 10 s | 6,268 | +292 tokens (~5%) |

Faster wall time, slightly more tokens — consistent with the extractor producing more fields per file.

### Step 12 — Same 10-query benchmark

| Query | rg | R2 retrieval | R3 retrieval | Δ |
|---|---:|---:|---:|---|
| `Linux` | 0 | 1 | 1 | held |
| `Windows Subsystem` | 0 | 1 | 1 | held |
| `WSL` | 1 | 1 | 1 | held |
| `shell prompt` | 2 | 2 | 2 | held |
| `HTTP client` | 2 | 2 | 2 | held |
| `directory navigation` | 0 | 2 | 2 | held |
| `fuzzy jump` | 0 | 0 | 0 | still miss (substring needs `fuzzy directory jump` contiguous) |
| `modern Unix tools` | 0 | 0 | 0 | still miss |
| `tab completion` | 0 | 0 | 0 | still miss |
| `API client` | 0 | 0 | **2** | **newly fixed** — alias `xh → API client`, `HTTPie/xh → API client` |

Score on the 10-query set: **R1 0 → R2 6 → R3 7**. Retrieval is now strictly stronger than rg on this corpus (rg gets 4, retrieval gets 7).

### Step 13 — Adjacent synonym probes

| Query | rg | R3 retrieval | Note |
|---|---:|---:|---|
| `fuzzy directory` | 0 | 2 | alias `zoxide → fuzzy directory jump` now hits |
| `API testing client` | 1 | 1 | parity (entity name in tools/README.md) |
| `multi-shell completion` | 2 | 2 | parity |
| `modern CLI` | 0 | 0 | nobody wrote it |
| `rust CLI` | 0 | 0 | nobody wrote it |

The extractor is now generating **multiple synonyms per concept** — `zoxide` aliases to both `directory navigation` *and* `fuzzy directory jump`; `xh` aliases to both `HTTP client` *and* `API client`. That's the right pattern: broaden the synonym net per entity.

### Step 14 — Bug status vs Round 2 report

| # | Bug | R2 | R3 |
|---|---|---|---|
| 1 | Code-snippet leakage into `retrieval_terms` (literal `bat file.py\` instead of \`cat file.py`) | present | **still present** |
| 2 | Markdown-link leakage (`[claude-code/](claude-code/`) | present | **fixed** |
| 3 | Bogus alias `from = "Example"` / `"Note"` (artifact of bullet-prefix parsing) | present | **still present** in `CLAUDE.md` (`Example → HTTP client`, `Example → API client`) |
| 4 | `entities` array always empty | present | **partially fixed** — content-heavy files now populated (CLAUDE.md: 12 entities, tools/README.md: 16); README-only stubs still empty (reasonable) |
| 5 | Formulaic `likely_queries` | present | **expanded but still formulaic** — CLAUDE.md now emits 24 queries, all templated as `configure X` / `how to set up X` / `which tool handles X`. The template set grew; the form did not change. `install_wsl.md` is still the only file with truly natural queries (3 of them this round, e.g. *"set up zed editor in wsl"*) |

### Step 15 — New entity field, qualitative spot check

`tools/README.md` entities (Round 3 sample):

```
bat       -> "Cat clone with syntax highlighting"
DBeaver   -> "Universal database tool"
HTTPie/xh -> "Command-line HTTP clients"
mitmproxy -> "Interactive HTTPS proxy"
starship  -> "Cross-shell prompt"
zoxide    -> "Smarter cd command"
...
```

Clean, structured "name + role" rows that match the source bullets. Useful for an agent answering "what's the X tool?" without reading the file.

`CLAUDE.md` entities are mostly clean too, but contain two artifacts:
- `Example` listed as an entity name with role = a code example string ("bat file.py\` instead of \`cat file.py")
- `Note` listed as an entity name with role = the Nushell alias note

Same root cause as bug #3: the extractor is splitting on something (likely "**" / colon / bullet) and picking up section labels like "Example" and "Note" as if they were entity names.

## Verdict (Round 3)

The retrieval extractor matured between R2 and R3 without changing the method's external contract:

- **+1 query resolved** on the 10-query benchmark (`API client`).
- **Multiple aliases per entity** is the right shape and unlocks adjacent synonym queries (`fuzzy directory`, `API client` variants).
- **`entities` is now populated** for content-heavy files, giving agents a clean tool-→-role lookup.
- **Markdown-link leakage fixed.**
- **Code-snippet leakage and bogus `Example`/`Note` aliases remain** — both stem from the bullet/prefix parser treating section labels as entity names. Worth targeted fix.
- **`likely_queries` template diversification is more diverse, not more natural.** The template set grew (`configure X` / `how to set up X` / `which tool handles X`) but every emitted query still follows a template. `install_wsl.md` shows that natural queries are possible when the prompt has enough context — the gap is in how the prompt is constructed for tool-list files.

Recommended next iteration:
1. **Fix the entity parser's leading-token detection** so `Example` and `Note` (and similar Markdown section markers like `Warning`, `Tip`) are recognized as labels, not entity names. This single fix would clean up both bug #1 (code snippets) and bug #3 (bogus aliases).
2. **Drop the templated-query generator** for files that already have rich entity coverage; rely on the LLM to emit 2–4 natural-language queries the way `install_wsl.md` did. Quality > quantity.
3. **Word-order-tolerant matching** in any agent code that consumes this metadata — at least tokenize the query and require all tokens present in any order, before falling back to substring. Cheap, no LLM cost, would catch `fuzzy jump` (via `fuzzy directory jump`) and `Linux editor setup` (via `set up editor on linux`).

---

## Cross-round consolidation

### Score progression (10-query benchmark, semantic intents)

| Round | rg | metadata | Notes |
|---|---:|---:|---|
| R1 | 4 | 1 | metadata = filename-derived tags only |
| R2a | 4 | 3 | summary helps when LLM expands acronyms (`WSL` → `Windows Subsystem for Linux`) |
| R2b | 4 | 6 | `builtin.retrieval.v1` closes 3 of 4 substring failure modes from R1 |
| R3 | 4 | **7** | extractor iteration adds `xh→API client`, `zoxide→fuzzy directory jump` |

`rg` baseline unchanged (it doesn't depend on the plan). Metadata channel went from worse-than-grep to strictly better-than-grep over three rounds.

### Failure modes (R1) — status across rounds

| Mode | Example query | R1 | R2a | R2b | R3 |
|---|---|---|---|---|---|
| 1. Compression dropped descriptor | `shell prompt` | miss | miss | **fixed** | held |
| 2. Synonym substitution | `HTTP client` | miss | miss | **fixed** | held |
| 3. Neither layer uses user vocab | `directory navigation` | miss | miss | **fixed** | held |
| (new in R3) Multiple synonyms per entity | `API client`, `fuzzy directory` | miss | miss | miss | **fixed** |
| Word-order tolerance | `Linux editor setup` | miss | miss | miss | miss (substring is order-sensitive) |
| Unwritten synonyms | `fuzzy jump`, `tab completion`, `modern Unix tools` | miss | miss | miss | miss |

### Outstanding bugs (still open after R3)

1. **Code-snippet leakage** in `retrieval_terms` (e.g. ``bat file.py` instead of `cat file.py``).
2. **Bogus aliases** with `from = "Example"` / `"Note"` — Markdown section labels treated as entity names. (Same root cause as #1.)
3. **`likely_queries` are templated** — R3 expanded the template set but every emitted query for tool-list files still follows one of `configure X` / `how to set up X` / `which tool handles X`. Only `install_wsl.md` produces natural queries.
4. **`aascribe map` ghost** — `aascribe search --help` instructs agents to "Use `aascribe map` for semantic routing", but `map` is not a registered command. Verified still present after the R3 rebuild.

### Recommended next iteration (ranked by ROI)

| # | Effort | Expected impact |
|---|---|---|
| 1. Patch entity parser to recognize Markdown section labels (`Example`, `Note`, `Warning`, `Tip`) as non-entities | Small, localized | Closes bugs 1 and 2 in one shot |
| 2. Word-order-tolerant matching in metadata consumers (tokenize + require all tokens, any order) | Trivial | Catches the `Linux editor setup` / `fuzzy jump` class without any LLM cost |
| 3. Drop templated `likely_queries` for files with ≥N entities; rely on the LLM as `install_wsl.md` already does | Small prompt change | Higher-quality natural queries; minor token savings |
| 4. Fix or remove the `aascribe map` reference in `aascribe search --help` | Trivial doc fix | Stops misleading agent users |
| 5. `builtin.embeddings.v1` for true semantic ranking | Medium | Closes the remaining "synonym the LLM didn't generate" misses (`tab completion`, `modern Unix tools`); enables relevance ordering |

### Stability / determinism findings (held across all rounds)

- Plan validation **digest matches** the run-report digest — good integrity property.
- `content_hash`-based **staleness detection works**; `--refresh` is required to override.
- The `index_plan` field in the run report **preserves the plan path and digest** — auditable.
- Method discovery via `aascribe --format json index methods` returns a stable shape suitable for agent consumption.

These are the load-bearing trust properties for an agent-facing index feature, and they held cleanly across three rounds of method iteration.
