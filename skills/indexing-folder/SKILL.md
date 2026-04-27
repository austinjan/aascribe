---
name: indexing-folder
description: Use aascribe to index folders, inspect folder index maps, find related files from map summaries, and monitor async indexing operations. Use this skill whenever the user asks to understand a repository or folder with aascribe, build or refresh an index, inspect an index map, locate relevant files, or check indexing operation status.
---

# aascribe Index Skill

Use this skill when you need to understand a folder, build or refresh its `aascribe` index, inspect the folder map, find likely related files, or monitor an async indexing operation.

## What aascribe Gives You

`aascribe index` scans a directory, summarizes direct files per folder, and writes local metadata files named `.aascribe_index_meta.json` beside indexed directories.

`aascribe map` reads those metadata files and returns a routing overview. Use the map to decide which folders or files to inspect next. The map is not a substitute for reading the target source file when precision matters.

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

## Standard Workflow

1. Index the target folder.

```bash
"$AASCRIBE" index <folder> --depth 2
```

Use `--no-summary` for a fast structural pass.

```bash
"$AASCRIBE" index <folder> --depth 2 --no-summary
```

Use `--refresh` when summaries may be stale and you want to regenerate them.

```bash
"$AASCRIBE" index <folder> --depth 2 --refresh
```

2. Read the folder map.

```bash
"$AASCRIBE" map <folder>
```

Use JSON when another tool or agent needs to parse the map.

```bash
"$AASCRIBE" --format json map <folder>
```

3. Use the map to pick likely related files.

Look for folder summaries, file summaries, and node state:

- `ready`: metadata exists and can be used for routing.
- `dirty`: metadata exists but was marked stale; re-run `"$AASCRIBE" index`.
- `unindexed`: metadata is missing; inspect directly or run `"$AASCRIBE" index` on that folder.

After choosing likely files, read the actual files with normal filesystem tools before making code changes or final claims.

4. Check what needs re-indexing when unsure.

```bash
"$AASCRIBE" index eval <folder>
```

If the result says files or folders need indexing, run:

```bash
"$AASCRIBE" index <folder>
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

## Finding Related Files

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

For content-specific searches, combine the map with regular search:

```bash
rg -n "operation_id|PathIndexTree|\\.aascribe_index_meta" .
```

The map helps choose where to look; `rg` and file reads confirm the exact code.

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

## Safety Notes

- `"$AASCRIBE" map` is a routing overview, not proof. Always inspect source files for exact behavior.
- `"$AASCRIBE" index` writes `.aascribe_index_meta.json` files into indexed directories.
- `"$AASCRIBE" index clean <folder> --dry-run --force` shows generated metadata files that would be removed.
- `"$AASCRIBE" index clean <folder> --force` removes generated metadata files. Use it only when cleanup is intended.
- Keep `--concurrency` modest if the LLM backend or local machine is the bottleneck.
