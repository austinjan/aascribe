---
name: indexing-folder
description: Use this skill as the default repo-scoped question-answering and file-routing tool whenever the user asks how something is handled in the current repository or folder, wants related files, asks whether a folder mentions a concept, or needs exact line-level confirmation. Route by intent: read known files directly, search known exact terms first, and use aascribe map for uncertain terminology or broad semantic routing. Also use it when the user explicitly asks to index a folder, inspect an index map, refresh summaries, or check async indexing operations.
---

# aascribe Index Skill

Use this skill when you need to answer a question from evidence in a repository or folder, find likely related files, confirm exact mentions, build or refresh an `aascribe` index, inspect the folder map, or monitor an async indexing operation.

Typical trigger examples:

- The user asks how a topic is handled in this project, such as "how is zsh configured here?"
- The user asks which files are related to a feature, concept, command, error, or setup flow.
- The user asks whether this folder mentions a term, file path, setting, or API.
- The user asks for line numbers, snippets, exact mentions, or an exhaustive mention audit inside a repo scope.
- The user asks to understand, orient around, or inspect a local folder before answering.

## What aascribe Gives You

`aascribe index` scans a directory, summarizes direct files per folder, and writes local metadata files named `.aascribe_index_meta.json` beside indexed directories.

`aascribe map` reads those metadata files and returns a routing overview. Use the map to decide which folders or files to inspect next. The map is not a substitute for reading the target source file when precision matters. Keep the first map bounded; use bare `--tag` to list available tags, `--tag <tag>` (repeatable with AND semantics) to return only related files, or `--recent-modified` to return the 20 newest current files visible to the map.

`aascribe search` performs exact text search with matching-line results. Use it first when the query already supplies an exact term; use it after `map` only when semantic routing was needed. Fixed-string auto mode tries `rg`, `git grep`, `grep`, then builtin with one shared scope. Regex auto mode uses compatible `rg`, then builtin.

Default to running with summaries. Use `--no-summary` only when you need a structural overview and will inspect files manually anyway. Content questions, such as "what does X cover?" or "where is Y discussed?", require summaries.

The default store is `./data/memory` in the current working directory, unless `AASCRIBE_STORE` or `--store <path>` is set.

## Binary Location

This skill assumes the `aascribe` binary is bundled with the skill at:

```text
skills/indexing-folder/bin/aascribe
```

On Windows, the binary may be:

```text
skills/indexing-folder/bin/aascribe.exe
```

When running commands from a repository root, prefer that bundled binary instead of assuming `aascribe` is on `PATH`.

```bash
AASCRIBE=./skills/indexing-folder/bin/aascribe
"$AASCRIBE" --version
```

If the bundled binary is unavailable, fall back to `aascribe` on `PATH` only after checking it exists.

```bash
command -v aascribe
```

## Intent-Based Routing

Choose evidence according to the question before selecting a command:

- **Known file or location:** read it directly. Consult a map only if broader context is useful.
- **Known literal, symbol, path, or error:** search first with fixed-string mode, then read the matching source.
- **Unknown terminology or broad concept:** use an existing, trustworthy map for semantic routing; otherwise use read-only filesystem discovery. Search candidate scopes when exact confirmation helps.

Source files, tests, configuration, and generated artifacts are final evidence as relevant. Map summaries are routing hypotheses, not proof.

## Read-Only Repository Questions (Default)

Ordinary repository questions are read-only. Locate the bundled binary, inspect whether `.aascribe_index_meta.json` already exists, and use existing metadata only when it is useful. If metadata is absent or cannot be trusted, use read-only filesystem discovery and inspect source files directly.

Do not run `init`, indexing, refresh, dirty-marking, or cleanup commands unless the user explicitly requested indexing, refresh, or another repository mutation. Missing metadata is not permission to create it.

Existing metadata can be read without modifying the repository:

```bash
"$AASCRIBE" map <folder>
"$AASCRIBE" map <folder> --tag
"$AASCRIBE" map <folder> --tag <tag>
"$AASCRIBE" map <folder> --recent-modified
"$AASCRIBE" --format json map <folder>
```

Treat map summaries as routing hints. Read the selected source files before making final claims.

## Explicitly Mutating Index Workflow

Use this workflow only when the user explicitly asks to initialize, index, or refresh a folder.

1. Initialize the project store when requested and needed.

```bash
"$AASCRIBE" init
```

This creates `./data/config/config.toml` with a Gemini config template. For LLM-backed summaries, use the configured secret-handling mechanism; do not place a real secret in commands or skill prose.

2. Index or refresh the requested target.

```bash
"$AASCRIBE" index <folder> --depth 2
"$AASCRIBE" index <folder> --depth 2 --no-summary
"$AASCRIBE" index <folder> --depth 2 --refresh
```

Indexing writes `.aascribe_index_meta.json` in each indexed directory. `init` writes store configuration and data directories. Refresh rewrites the affected metadata.

3. Inspect the resulting map and source files.

```bash
"$AASCRIBE" map <folder>
```

## Async Indexing

Use async indexing for large folders or slower summarization runs.

```bash
"$AASCRIBE" index --async <folder>
```

The command returns an `operation_id`. Use it to inspect progress and results.

```bash
"$AASCRIBE" operation status <operation-id>
"$AASCRIBE" operation events <operation-id>
"$AASCRIBE" operation result <operation-id>
```

If `operation result` includes an `output_id`, inspect the stored payload with:

```bash
"$AASCRIBE" output show <output-id>
"$AASCRIBE" output slice <output-id> --offset 0 --limit 4000
```

To list known operations:

```bash
"$AASCRIBE" operation list
```

To cancel a pending or running operation:

```bash
"$AASCRIBE" operation cancel <operation-id>
```

Do not cancel completed, failed, or already useful operations unless the user asks.

## Indexing To Find Related Files (Explicit Request)

Use this section only when the user explicitly requested indexing. For an ordinary read-only question, follow Intent-Based Routing instead.

Use this pattern when asked to find relevant implementation files:

```bash
"$AASCRIBE" index . --depth 2
"$AASCRIBE" map .
```

Then inspect the folders that look relevant. If a folder is `unindexed` or too broad, index it more directly:

```bash
"$AASCRIBE" index ./internal/index --depth 2
"$AASCRIBE" map ./internal/index
```

For content-specific searches, use repeated fixed-string searches so fallback engines do not reinterpret regex syntax:

```bash
"$AASCRIBE" search "operation_id" . --fixed-strings
"$AASCRIBE" search "PathIndexTree" . --fixed-strings
"$AASCRIBE" search ".aascribe_index_meta" . --fixed-strings
```

The map helps choose where to look; `search` and file reads confirm the exact code.

## Samples

### Structural-Only Orientation

```bash
"$AASCRIBE" index . --depth 2 --no-summary
"$AASCRIBE" map .
```

Use this when you only need a fast folder structure overview and will inspect files manually.

### Summarized Index For A Subtree

```bash
"$AASCRIBE" index ./internal --depth 2 --include '*.go'
"$AASCRIBE" map ./internal
```

Use this when you need summaries for a specific implementation area.

### Large Folder With Operation Tracking

```bash
"$AASCRIBE" index --async ./docs
"$AASCRIBE" operation list
"$AASCRIBE" operation status <operation-id>
"$AASCRIBE" operation events <operation-id>
"$AASCRIBE" operation result <operation-id>
```

Use this when indexing might take long enough that progress and restart-safe state matter.

### Refresh Stale Metadata

```bash
"$AASCRIBE" index eval ./internal
"$AASCRIBE" index ./internal --refresh
"$AASCRIBE" map ./internal
```

Use this when the map looks old or the code has changed substantially.

### Exact Confirmation After Routing

```bash
"$AASCRIBE" map .
"$AASCRIBE" search "zprofile" ./os-config --fixed-strings
```

Use this when the map identifies likely folders but the user needs exact mentions, snippets, or line numbers.

## Absence Claims

A zero-result search supports an absence claim only when the result covers an adequate declared scope, reports `complete: true`, and reports `truncated: false`.

Do not claim absence when metadata freshness is unknown, traversal skipped or could not read content, the selected engine covers a narrower scope, or output was truncated. Narrow the scope, use a reliable compatible engine, raise the limit, or inspect the relevant files directly before making the claim.

## Safety Notes

- `"$AASCRIBE" map` is a routing overview, not proof. Always inspect source files for exact behavior.
- Do not use `"$AASCRIBE" map` as proof of absence. Search returns one result per matching line, not per individual occurrence; validate scope and completeness, then read matched files as needed.
- `"$AASCRIBE" index` writes `.aascribe_index_meta.json` files into indexed directories.
- `"$AASCRIBE" index clean <folder> --dry-run --force` shows generated metadata files that would be removed.
- `"$AASCRIBE" index clean <folder> --force` removes generated metadata files. Use it only when cleanup is intended.
- Keep `--concurrency` modest if the LLM backend or local machine is the bottleneck.
