# aascribe Usage

Related docs:
- [installation.md](installation.md)
- [reference/aascribe_ai_output_shapes.md](reference/aascribe_ai_output_shapes.md)
- [shapes/README.md](shapes/README.md)
- [logging.md](logging.md)

Complete command and flag reference for `aascribe`.

- [Global flags](#global-flags)
- [Time format](#time-format)
- [Output envelope](#output-envelope)
- [Exit codes](#exit-codes)
- [Commands](#commands)
  - [init](#init)
  - [config](#config)
  - [logs](#logs)
  - [operation](#operation)
  - [index](#index)
  - [map](#map)
  - [search](#search)
  - [describe](#describe)
  - [remember](#remember)
  - [consolidate](#consolidate)
  - [recall](#recall)
  - [list](#list)
  - [show](#show)
  - [forget](#forget)
- [Recipes](#recipes)

---

## Repository evidence workflow

Repository questions are read-only unless the caller explicitly requests indexing, refresh, dirty-marking, or cleanup. Choose the shortest evidence path that fits the question:

- read a known file or location directly
- search a known literal, symbol, path, or error first
- use `map` for uncertain terminology or broad semantic routing, then inspect selected source files

Metadata presence is separate from verified freshness. `state: "ready"` means local metadata exists; trust it only when the same node reports `freshness: "fresh"` and `freshness_checked: true` without relevant partial issues.

Search returns matching lines, not individual occurrences. A zero-result response supports an absence claim only when its declared `scope` is adequate, `complete: true`, and `truncated: false`.

---

## Global flags

Available on every subcommand:

| Flag | Default | Description |
|---|---|---|
| `--store <path>` | `$AASCRIBE_STORE` or `./data/memory` | Path to the memory store root |
| `--format json\|text` | `text` | Output format. `text` is the compact default for LLM reading; `json` is for machine parsing |
| `--quiet`, `-q` | off | Suppress all logging; only emit the result |
| `--verbose`, `-v` | off | Emit debug logs to stderr |
| `--help`, `-h` | — | Show help for the command |
| `--version` | — | Show version |

---

## Time format

Any flag accepting a time value (`--since`, `--until`, `--ttl`) supports:

- **Relative durations**: `30s`, `15m`, `6h`, `2d`, `1w`
- **ISO 8601 timestamps**: `2026-04-20`, `2026-04-20T10:30:00Z`

Relative values are interpreted as "time ago" for `--since`/`--until` and as "from now" for `--ttl`.

---

## Output envelope

Default text output is optimized for LLM agents. Commands that manage state, inspect status, or return handles should include compact `next:` hints. Commands whose output is the requested content itself, such as `chat`, `summarize`, `describe`, and output chunk readers, keep text output focused on that content.

When you explicitly request `--format json`, every JSON response uses this shape:

```json
{
  "ok": true,
  "data": { ... },
  "meta": {
    "command": "recall",
    "duration_ms": 42,
    "store": "/path/to/project/data/memory"
  }
}
```

On error:

```json
{
  "ok": false,
  "error": {
    "code": "MISSING_SECRET",
    "message": "Environment variable GEMINI_API_KEY is not set or is empty."
  },
  "meta": { ... }
}
```

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General runtime error |
| `2` | Invalid arguments |
| `3` | Store not found or not initialized |
| `4` | Requested resource (memory id, file) not found |

---

## Commands

### `init`

Create a new store. Sets up a filesystem-first directory layout for short-term memory, long-term memory, index data, cache data, output transport data, and layout metadata.
Also creates a safe default `config.toml` template in the sibling `data/config/` directory when one does not already exist.
The template stores the API key environment variable name, not the secret itself.
Commands that need the store create the default store and config skeleton automatically when the store is missing. Run `init` explicitly when you want to inspect or repair setup before another command.

```
aascribe init [--store <path>] [--force]
```

| Flag | Description |
|---|---|
| `--store <path>` | Where to create the store (default: `./data/memory` in the current working directory, or `$AASCRIBE_STORE` when set) |
| `--force` | Reinitialize even if a store already exists (destructive) |

**Example**

```bash
aascribe init
aascribe init --store ./project-mem
```

OutputShape: StoreInitResult

**Bootstrap-managed layout**

- `short_term/`
- `long_term/`
- `index/`
- `cache/`
- `outputs/`
- `operations/`
- `layout.json`

**Output**

```json
{
  "ok": true,
  "data": {
    "store": "/path/to/project/data/memory",
    "created": true,
    "reinitialized": false,
    "layout_version": "bootstrap-v1",
    "config_path": "/path/to/project/data/config/config.toml",
    "config_created": true
  },
  "meta": {
    "command": "init",
    "duration_ms": 4,
    "store": "/path/to/project/data/memory"
  }
}
```

---

### `config`

Set required local configuration values for the active store. If the store does not exist yet, `config` creates the default store and config skeleton first.

```bash
aascribe config gemini_api_key=<key>
aascribe --store ./project-mem config gemini_api_key=<key>
```

The config skeleton only contains non-secret settings such as provider, model, `api_key_env`, and timeout. The Gemini API key is required for summary-backed `index` and other LLM-backed commands, has no default, and must be supplied by the user.

The key is written to `data/config/.env` as `GEMINI_API_KEY` and is not echoed back in command output.

---

### `logs`

Inspect and manage the active log file for the current store.

```
aascribe logs <subcommand> [flags]
```

Subcommands:

- `path`
  Return the resolved active log file path.
- `export --output <path>`
  Copy the current log file to another location.
- `clear --force`
  Truncate the current log file.

**Examples**

```bash
aascribe --store ./project-mem logs path
aascribe --store ./project-mem logs export --output ./aascribe-debug.log
aascribe --store ./project-mem logs clear --force
```

OutputShape: LogPathResult (logs path)

Example `logs path` output:

```json
{
  "ok": true,
  "data": {
    "path": "/Users/you/logs/aascribe.log"
  },
  "meta": {
    "command": "logs",
    "duration_ms": 1,
    "store": "/Users/you/project-mem"
  }
}
```

OutputShape: LogExportResult (logs export)

Example `logs export` output:

```json
{
  "ok": true,
  "data": {
    "source_path": "/Users/you/logs/aascribe.log",
    "output_path": "/Users/you/aascribe-debug.log"
  },
  "meta": {
    "command": "logs",
    "duration_ms": 2,
    "store": "/Users/you/project-mem"
  }
}
```

OutputShape: LogClearResult (logs clear)

Example `logs clear` output:

```json
{
  "ok": true,
  "data": {
    "path": "/Users/you/logs/aascribe.log",
    "cleared": true
  },
  "meta": {
    "command": "logs",
    "duration_ms": 1,
    "store": "/Users/you/project-mem"
  }
}
```

---

### `operation`

Inspect persisted long-running operation state for the active store.

```
aascribe operation list
aascribe operation status <operation-id>
aascribe operation events <operation-id>
aascribe operation result <operation-id>
aascribe operation cancel <operation-id>
aascribe operation clean [--dry-run] [--force]
```

Subcommands:

- `list`
  List known operations for the active store.
- `status <operation-id>`
  Show the latest lifecycle snapshot for one operation.
- `events <operation-id>`
  Show persisted event history for one operation.
- `result <operation-id>`
  Show the final result record for one completed operation.
- `cancel <operation-id>`
  Mark a pending or running operation as canceled.
- `clean`
  Remove completed, failed, and canceled operation records from the active store.

Behavior notes:

- Operation lifecycle state is stored under `<store>/operations/<operation-id>/`.
- `operation.json` is the latest compact status snapshot.
- `events.jsonl` is append-only event history.
- `result.json` is written only when a final result is available. Small results may include `data`; oversized results store data through the managed `output` transport and return `output_id`.
- `operation result` returns a typed error if the operation has not completed yet.
- When `operation result` shows `output_id`, use `aascribe output show <output-id>` or `aascribe output slice <output-id> --offset 0 --limit 4000` to inspect the stored payload.
- Operation-linked `output_id` payloads are not protected from normal output retention. If the referenced output has been pruned, `output show/slice/meta` returns the normal `OUTPUT_NOT_FOUND` guidance; the operation record itself is still valid historical state.
- `operation cancel` is idempotent for already canceled operations and rejects operations that already succeeded or failed.
- `operation clean` defaults to dry-run unless `--force` is provided. It never removes `pending` or `running` operations.
- `operation clean` removes operation lifecycle directories only; managed `outputs/` payloads follow the output transport retention policy.

Examples:

```bash
aascribe operation list
aascribe operation status op_20260424T120000Z_ab12cd34
aascribe operation events op_20260424T120000Z_ab12cd34
aascribe operation result op_20260424T120000Z_ab12cd34
aascribe operation cancel op_20260424T120000Z_ab12cd34
aascribe operation clean --dry-run
aascribe operation clean --force
```

OutputShape: OperationList (operation list)
OutputShape: OperationStatus (operation status)
OutputShape: OperationEventList (operation events)
OutputShape: OperationResult (operation result)
OutputShape: OperationCancelResult (operation cancel)
OutputShape: OperationCleanResult (operation clean)

---

### `index`

Walk a directory, summarize direct files per folder, and persist local metadata.

```
aascribe index <path> [flags]
aascribe index methods [--stage file|folder|start|end]
aascribe index dirty <path>
aascribe index eval <path>
```

| Flag | Default | Description |
|---|---|---|
| `<path>` | — | Directory to index (positional, required) |
| `--depth <N>` | `3` | Recursion depth. `0` = current level only. `-1` = unlimited |
| `--include <glob>` | — | Include only matching files. Repeatable |
| `--exclude <glob>` | `.git`, `node_modules`, `target`, `dist`, `.venv` | Exclude matching files/dirs. Repeatable |
| `--concurrency <N>` | `4` | Max concurrent direct-file processing jobs across the index run |
| `--async` | off | Start indexing as a persisted operation and return an `operation_id` immediately |
| `--refresh` | off | Ignore cache; regenerate summaries |
| `--no-summary` | off | Return structure only. Much faster |
| `--metadata-method <method>` | default set | Select explicit metadata methods. Repeatable. Current methods: `base`, `summary`, `builtin.classify.v1`, `builtin.retrieval.v1`, `builtin.routing.v1`, `builtin.run-context.v1`, `builtin.run-report.v1` |
| `--no-metadata-methods` | off | Disable optional/default methods beyond required base metadata |
| `--index-plan <file>` | off | Use an `index-plan-v1` TOML file to select built-in methods |
| `--max-file-size <bytes>` | `1048576` | Files over this size get metadata only, no summary |

Behavior notes:

- `index` treats `.gitignore` and `.aaignore` as folder-scoped blacklist exclude files during traversal.
- Text-file detection is content-based, not extension-based.
- `index` writes `.aascribe_index_meta.json` files in indexed directories.
- With no metadata method flags, `index` uses the default method set: required `base` metadata plus `summary`.
- If `--metadata-method` flags are provided, `index` uses those methods plus required `base`.
- `--no-metadata-methods` keeps only required `base` metadata.
- `--index-plan` is mutually exclusive with `--metadata-method` and `--no-metadata-methods`.
- `index plan validate <file>` validates an `index-plan-v1` TOML file without running index.
- `index plan --help` is written as an AI-agent contract for discovering methods, drafting TOML, validating, revising, and only then running the plan.
- `index review` provides a checkpointed prompt-improvement workflow for retrieval metadata: review current output, propose a prompt patch, patch, re-index, then accept or restore.
- Async `--index-plan` runs snapshot the submitted plan into the operation directory before starting the worker.
- `builtin.classify.v1` classifies text/status-eligible direct files using deterministic path, file type, and optional summary signals.
- `builtin.retrieval.v1` writes LLM-facing retrieval hints: retrieval terms, aliases, entities, and likely queries derived from bounded file text plus path/summary signals.
- `builtin.routing.v1` writes folder-level routing metadata by aggregating child classification output.
- `builtin.run-context.v1` writes root-only start lifecycle metadata under `run_methods`; it does not feed file or folder method input.
- `builtin.run-report.v1` writes root-only end lifecycle aggregates under `run_methods` after traversal.
- `index methods` lists available methods with descriptions; use `--format json` for machine-readable descriptors.
- `summary` is LLM-backed. If LLM config or the configured API-key environment variable is missing, summary-enabled indexing returns an error instead of using a local fallback summary.
- Successful sync and async `index` jobs also write a Markdown completion report under `data/report/` when using the default `data/memory` store.
- Each `.aascribe_index_meta.json` is local-only: it describes the current folder and its direct files, not a full descendant tree.
- Metadata records persisted file descriptions, non-indexed files, failures, and warnings.
- `index` uses bounded direct-file concurrency. Increase `--concurrency` to speed up summarize/hash work; keep it low if the LLM backend or machine is the bottleneck.
- `index --async <path>` creates a persisted operation, starts a background index worker, and returns an `OperationAccepted` payload. Inspect progress with `operation status`, inspect history with `operation events`, and fetch the final result with `operation result`.
- Re-running `index` reuses unchanged direct-file metadata when possible.
- `index dirty <path>` marks existing metadata stale so the next `index` run rebuilds it.
- `index eval <path>` previews which folders and direct files need indexing, plus which ones are unchanged.

**Example**

```bash
aascribe index --depth 2 --include '*.rs' --include '*.toml' ./src
aascribe index --concurrency 8 ./src
aascribe index --async ./src
aascribe index --no-summary .           # structure-only, fast
aascribe index --metadata-method summary .
aascribe index --metadata-method builtin.classify.v1 --metadata-method builtin.retrieval.v1 --metadata-method builtin.routing.v1 .
aascribe index --metadata-method builtin.run-context.v1 --metadata-method builtin.run-report.v1 .
aascribe index methods
aascribe --format json index methods --stage file
aascribe index plan validate ./aascribe.index.toml
aascribe index --index-plan ./aascribe.index.toml .
aascribe index review ./coding --method builtin.retrieval.v1
aascribe index review propose ./data/report/2026-05-19-retrieval-review.json
aascribe index review patch ./data/report/2026-05-19-retrieval-prompt-proposal.json
aascribe index review restore index-review-20260519T120000Z
aascribe index --refresh .              # force re-summarize
aascribe index dirty ./src              # mark existing metadata stale
aascribe index eval ./src               # preview changed vs unchanged work
```

OutputShape: PathIndexTree (synchronous index)

OutputShape: OperationAccepted (index --async)

**Output**

```json
{
  "ok": true,
  "data": {
    "root": "/abs/path/to/src",
    "report_path": "/abs/path/to/data/report/2026-04-03-101233-indexing-report.md",
    "tree": {
      "path": "src",
      "type": "dir",
      "children": [
        {
          "path": "src/main.rs",
          "type": "file",
          "size": 4821,
          "hash": "sha256:...",
          "summary": "CLI entry point; wires clap parser to command dispatch.",
          "summarized_at": "2026-04-23T09:15:00Z"
        }
      ]
    }
  }
}
```

---

### `map`

Assemble a hierarchy view by reading local `.aascribe_index_meta.json` files recursively.

```
aascribe map <path> [--depth <N>] [--summary-only] [--max-files-per-dir <N>] [--full]
aascribe map <path> --tag
aascribe map <path> --tag <tag> [--tag <tag> ...]
aascribe map <path> --recent-modified
```

| Flag | Default | Description |
|---|---|---|
| `--depth <N>` | `2` | Maximum child-directory depth in the returned projection |
| `--summary-only` | off | Omit direct-file detail while preserving routing summaries and continuations |
| `--max-files-per-dir <N>` | `20` | Maximum direct files returned per directory |
| `--full` | off | Disable semantic depth and file bounds; explicit partial errors still apply |
| `--tag` | off | With no value, list tags and file counts. With a value, list related files. Repeat values with AND semantics |
| `--recent-modified` | off | List up to 20 current files visible to the map by filesystem modification time, newest first |

Focused views:

- Bare `--tag` returns a compact, alphabetically sorted tag catalog with unique file counts.
- `--tag <tag>` performs case-insensitive exact tag matching. Repeating the flag requires every supplied tag.
- `--recent-modified` returns a compact global list instead of the hierarchy. Narrow `<path>` to inspect more than the first 20 files.
- Tag and recent-modified views traverse all accessible metadata below `<path>`, report whether the result is complete, and do not include file summaries.
- `--tag` and `--recent-modified` cannot be combined with each other or with hierarchy projection flags.

Behavior notes:

- `map` is a routing overview for agents. Use it to choose the folder or file to inspect next; for precise answers, inspect the target file directly. Re-index only when the caller explicitly requests repository mutation.
- `map` reads metadata and checks direct-file content fingerprints against the current filesystem; it does not re-summarize source files.
- `map` applies the current `.gitignore` and `.aaignore` blacklist subset while traversing. `!` re-inclusion is not supported and `ignore_policy` discloses this limitation.
- The default projection is bounded. `projection` declares the effective controls and each `omitted` entry provides its reason, path, and exact continuation command.
- `--full` restores complete traversal for migrating callers that depended on the previous unbounded behavior.
- `text` is the default output and returns a compact tree intended to be easier for LLMs to read.
- `--format json` returns a machine-friendly assembled projection.
- Oversized text or JSON is a final safety net: inline output contains a preview and `transport`; the complete payload is stored under `output_id` and can be read with `output slice`.
- `index map <path>` is supported as an alias, but `map <path>` is the preferred surface.
- map node state reports metadata access, while freshness reports filesystem drift:
  - `ready`: local metadata exists; consult `freshness`
  - `dirty`: local metadata exists but was explicitly marked stale
  - `unindexed`: no local metadata file exists yet
  - `corrupt`: metadata exists but cannot be parsed
  - `unreadable`: metadata, ignore policy, or directory content could not be read
- `freshness` is `fresh`, `stale`, or `unknown`; `freshness_checked` states whether direct-file fingerprints were compared.
- `partial` and structured `issues` expose missing metadata details, corruption, unreadable paths, symlink boundaries, and drift.
- In-root non-cyclic directory symlinks are followed. Cycles and out-of-root targets stop with structured issues.

**Example**

```bash
aascribe map ./tests
aascribe map ./tests --tag
aascribe map ./tests --tag docs --tag integration
aascribe map ./tests --recent-modified
aascribe --format text map ./tests
aascribe --format json map ./docs
```

OutputShape: PathIndexMap

**JSON Output**

```json
{
  "ok": true,
  "data": {
    "root": "/abs/path/to/tests",
    "ignore_policy": {
      "mode": "blacklist",
      "negation_supported": false,
      "sources": [".gitignore", ".aaignore"]
    },
    "state_guide": {
      "dirty": "Metadata exists and is explicitly marked stale.",
      "ready": "Metadata exists; consult freshness before trusting summaries.",
      "unindexed": "Metadata is missing; structural child stubs do not prove descendant absence."
    },
    "projection": {
      "mode": "bounded",
      "depth": 2,
      "summary_only": false,
      "max_files_per_dir": 20
    },
    "map": {
      "path": "/abs/path/to/tests",
      "state": "ready",
      "freshness": "fresh",
      "freshness_checked": true,
      "partial": false,
      "issues": [],
      "folder_description": "Contains 1 subdirectory.",
      "brief_summary": "Contains 1 subdirectory.",
      "stats": {
        "direct_dir_count": 1
      },
      "children": [
        {
          "path": "/abs/path/to/tests/index-fixtures",
          "state": "ready",
          "freshness": "fresh",
          "freshness_checked": true,
          "partial": false,
          "issues": [],
          "brief_summary": "Contains 4 files and 2 subdirectories."
        }
      ]
    }
  }
}
```

Focused JSON views use smaller result types instead of `PathIndexMap`:

- `--tag`: `{root, mode: "tag_catalog", tags: [{tag, file_count}], complete, issues}`
- `--tag <tag>`: `{root, mode: "tag_files", query, match: "all", files: [{path, matched_tags}], complete, issues}`
- `--recent-modified`: `{root, files: [{path, mod_time}], total_files, truncated, complete, issues}`

**Text Output**

```text
/abs/path/to/tests
  summary: Contains 1 subdirectory.
  index-fixtures
    summary: Contains 4 files and 2 subdirectories.
    README.md - fixture tree for recursive indexing tests
    notes.conf - localhost:8080 config
```

---

### `search`

Run exact text search and return matching lines. Use it first for a known exact term, or after `map` when semantic routing was needed.

```
aascribe search <query> [path] [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--engine auto\|rg\|git-grep\|grep\|builtin` | `auto` | Search engine. Fixed-string auto tries `rg`, `git grep`, `grep`, then builtin; regex auto uses `rg`, then builtin |
| `--ignore-case` | off | Case-insensitive matching |
| `--fixed-strings` | off | Treat query as literal text instead of a regular expression |
| `--glob <pattern>` | — | Include matching files. Repeatable |
| `--max-count <N>` | `100` | Maximum matches to return. `0` returns no matches, useful only for engine checks |

Behavior notes:

- `search` is for exact confirmation: matching lines, line numbers, and snippets. Repeated occurrences on one line are represented by one matching-line result.
- `search` is not semantic. If you do not know the keyword, use `map` first to route to likely files or folders.
- Fixed-string `auto` tries `rg`, `git grep`, `grep`, then builtin. Regex `auto` uses only compatible `rg`, then builtin.
- All engines consume the same default candidate scope: tracked and untracked files are included; hidden files and blacklist-matched `.gitignore`/`.aaignore` files are excluded.
- Regex mode is supported by `rg`, `builtin`, and `auto` (`rg` then `builtin`). Explicit `grep` or `git-grep` regex requests return `UNSUPPORTED_QUERY`; those engines remain available for fixed strings.
- A file path is an exact scope and searches only that file; it is never broadened to the parent directory.
- `complete` is false and `partial_reasons` describes unreadable scope when candidate discovery cannot inspect everything. Do not infer absence unless `complete: true`.
- Search supports matching lines up to 1 MiB. A larger builtin-scanner token produces `complete: false` with a `scanner_limit` reason.
- Binary files are deliberately excluded and listed in `exclusions`; this is distinct from an I/O failure and does not by itself make the result incomplete.
- When `auto` safely falls back after an external execution failure, `engine_failures` records the failed attempt. Explicit engine failures return a diagnostic error including engine stderr.
- `match_count` currently means returned matching lines, not total occurrences.
- `returned_match_lines` is the precise returned-line count; `match_count` is retained as a compatibility alias.
- External engines stream results and stop after limit-plus-one evidence, so `--max-count` bounds production as well as rendering.
- `query_mode` reports `fixed` or `regex`; use it to verify how metacharacters were interpreted.
- `column_unit: byte` means every reported column is a one-based byte offset.
- `--format json` returns a stable `SearchResult` payload for agents.

**Example**

```bash
aascribe search "zprofile" .
aascribe search "GEMINI_API_KEY" ./internal --fixed-strings
aascribe search "func .*Search" . --glob "*.go"
aascribe search "zsh" ./docs --engine builtin --ignore-case
```

OutputShape: SearchResult

**JSON Output**

```json
{
  "ok": true,
  "data": {
    "query": "zprofile",
    "root": "/abs/path/to/project",
    "engine": "rg",
    "query_mode": "fixed",
    "match_unit": "line",
    "column_unit": "byte",
    "scope": {
      "root": "/abs/path/to/project",
      "tracked_only": false,
      "hidden": false,
      "ignored": false,
      "ignore_sources": [".gitignore", ".aaignore"]
    },
    "match_count": 1,
    "returned_match_lines": 1,
    "truncated": false,
    "complete": true,
    "partial_reasons": [],
    "exclusions": [],
    "engine_failures": [],
    "matches": [
      {
        "path": "docs/setup.md",
        "line": 25,
        "column": 18,
        "text": "Add Homebrew to ~/.zprofile."
      }
    ]
  }
}
```

---

### `index dirty`

Mark existing `.aascribe_index_meta.json` files under one folder tree as stale.

```
aascribe index dirty <path>
```

Behavior notes:

- Updates existing metadata files only.
- Does not create missing metadata files.
- Does not summarize source files.

**Example**

```bash
aascribe index dirty ./tests
```

---

### `index eval`

Preview which folders and direct files need indexing, plus which ones are unchanged.

```
aascribe index eval <path>
```

Behavior notes:

- Reads the current filesystem and existing local metadata files.
- Folder state is local-only: child folder changes do not make the parent folder changed unless the parent's own direct files or metadata changed.
- Uses `needs_index` and `unchanged` states for both folders and files.

**Example**

```bash
aascribe index eval ./tests
aascribe --format json index eval ./tests
```

---

### `index clean`

Remove generated index metadata artifacts under one directory tree.

```
aascribe index clean <path> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `<path>` | — | Directory root to clean recursively |
| `--dry-run` | off | Show which `.aascribe_index_meta.json` files would be removed without deleting them |
| `--force` | required | Required because this command removes generated files |

**Example**

```bash
aascribe index clean ./tests --dry-run --force
aascribe index clean ./docs --force
```

**Output**

```json
{
  "ok": true,
  "data": {
    "root": "/abs/path/to/tests",
    "removed_paths": [
      "/abs/path/to/tests/.aascribe_index_meta.json",
      "/abs/path/to/tests/index-fixtures/.aascribe_index_meta.json"
    ],
    "removed_count": 2,
    "dry_run": true
  },
  "meta": {
    "command": "index",
    "duration_ms": 1,
    "store": "/Users/you/project-mem"
  }
}
```

---

### `describe`

Summarize a single file.

```
aascribe describe <file> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `<file>` | — | File to describe (positional, required) |
| `--refresh` | off | Ignore cache; regenerate |
| `--length short\|medium\|long` | `medium` | Summary length |
| `--focus <topic>` | — | Bias the summary toward a specific topic (e.g. `error handling`, `public API`) |

Behavior notes:

- `describe` uses the same file-analysis / summarization path as `index`.
- When store config and secrets resolve successfully, `describe` uses the configured Gemini path.
- When LLM config is unavailable, `describe` falls back to a deterministic local summary.

**Example**

```bash
aascribe describe ./src/poll.rs
aascribe describe --length long --focus "FOCAS retry logic" ./src/poll.rs
```

OutputShape: FileDescription

---

### `remember`

Write a short-term memory entry. Content can be a positional argument or piped via stdin.

```
aascribe remember [<content>] [flags]
```

| Flag | Default | Description |
|---|---|---|
| `<content>` | — | The memory text. Omit to read from stdin |
| `--tag <t>` | — | Tag the entry. Repeatable |
| `--source <ref>` | — | Provenance marker (file path, URL, conversation id) |
| `--session <id>` | auto | Group entries under a session |
| `--ttl <duration>` | none | Auto-expire after this duration |
| `--importance <1-5>` | `3` | Priority signal for consolidation |
| `--stdin` | off | Force reading from stdin (disambiguates when content looks like a flag) |

**Examples**

```bash
aascribe remember "FOCAS timeouts need exponential backoff" --tag nimbl --tag focas

echo "bbpollsvc crashes on malformed DNP3 frames" \
  | aascribe remember --tag nimbl --tag bug --importance 5

aascribe remember --source ./src/poll.rs --tag refactor \
  "poll loop should be split into producer/consumer"
```

OutputShape: RememberResult

**Output**

```json
{
  "ok": true,
  "data": {
    "id": "stm_01HZX9K7...",
    "session": "sess_20260423_0915",
    "stored_at": "2026-04-23T09:15:12Z"
  }
}
```

---

### `consolidate`

Analyze short-term memories and produce long-term memory entries. Typically run at the end of a session or on a schedule.

```
aascribe consolidate [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--since <time>` | `7d` | Include short-term entries newer than this |
| `--until <time>` | now | Include entries older than this |
| `--session <id>` | — | Limit to one session |
| `--tag <t>` | — | Limit to entries with this tag. Repeatable |
| `--topic <desc>` | — | Hint to bias consolidation toward a theme |
| `--dry-run` | off | Show what would be produced without writing |
| `--keep-short` | off | Keep consolidated short-term entries (default: delete them) |
| `--min-items <N>` | `3` | Skip consolidation if fewer than N candidates |

**Examples**

```bash
aascribe consolidate --since 2d --dry-run
aascribe consolidate --session sess_20260423_0915
aascribe consolidate --tag nimbl --topic "FOCAS protocol handling"
```

OutputShape: ConsolidationResult

**Output**

```json
{
  "ok": true,
  "data": {
    "created": [
      {
        "id": "ltm_01HZX...",
        "summary": "FOCAS integration requires retry + backoff; malformed DNP3 frames currently crash bbpollsvc.",
        "tags": ["nimbl", "focas", "bug"],
        "source_entries": ["stm_01HZX...", "stm_01HZY..."]
      }
    ],
    "consumed": 7,
    "skipped": 0
  }
}
```

---

### `recall`

Retrieve memories relevant to a query. Combines semantic similarity with tag, time, and tier filters.

```
aascribe recall <query> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `<query>` | — | Natural-language query (positional, required) |
| `--tier short\|long\|all` | `all` | Which memory tier to search |
| `--limit <N>` | `10` | Maximum results |
| `--tag <t>` | — | Filter by tag. Repeatable (AND semantics) |
| `--since <time>` | — | Only entries newer than this |
| `--until <time>` | — | Only entries older than this |
| `--min-score <0-1>` | `0.3` | Minimum similarity score |
| `--include-source` | off | Include `source` field in each result |

**Examples**

```bash
aascribe recall "FOCAS retry"
aascribe recall "deployment issues" --tier long --limit 5
aascribe recall "bug" --tag nimbl --since 7d --include-source
```

OutputShape: MemoryRecallResult

**Output**

```json
{
  "ok": true,
  "data": {
    "query": "FOCAS retry",
    "results": [
      {
        "id": "ltm_01HZX...",
        "tier": "long",
        "score": 0.87,
        "content": "FOCAS integration requires retry + backoff...",
        "tags": ["nimbl", "focas"],
        "created_at": "2026-04-23T09:30:00Z"
      }
    ]
  }
}
```

---

### `list`

List raw memory entries. Mostly for debugging or human review.

```
aascribe list [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--tier short\|long\|all` | `all` | Which tier |
| `--session <id>` | — | Filter by session (short-term only) |
| `--tag <t>` | — | Filter by tag. Repeatable |
| `--since <time>` | — | Newer than |
| `--until <time>` | — | Older than |
| `--limit <N>` | `50` | Maximum rows |
| `--order asc\|desc` | `desc` | Sort by creation time |

OutputShape: MemoryEntryList

---

### `show`

Show a single memory entry in full. `recall` and `list` output may be truncated for display; `show` gives you everything.

```
aascribe show <id>
```

| Flag | Description |
|---|---|
| `<id>` | Memory id (positional, required). Accepts `stm_...` or `ltm_...` |

OutputShape: MemoryEntryDetail

---

### `forget`

Delete a memory entry.

```
aascribe forget <id> [--force]
```

| Flag | Description |
|---|---|
| `<id>` | Memory id to delete |
| `--force` | Skip confirmation (required in non-interactive mode) |

OutputShape: ForgetResult

---

## Recipes

### Bootstrap context for a new task

```bash
# Get the lay of the land, then check if you've worked on this before
aascribe index . --depth 2 --no-summary
aascribe recall "$(basename $(pwd))" --tier long --limit 5
```

### Session loop

```bash
# Start of session
SESSION=$(date +sess_%Y%m%d_%H%M)

# During work — drop notes as you go
aascribe remember "refactored poll loop to use tokio::select" \
  --session "$SESSION" --tag refactor --tag nimbl

# End of session
aascribe consolidate --session "$SESSION"
```

### Scoped recall for a topic

```bash
aascribe recall "authentication" \
  --tag security \
  --since 30d \
  --min-score 0.5 \
  --include-source
```

### Re-index only what changed

The index cache keys on content hash, so just re-run — cached files are skipped automatically:

```bash
aascribe index .
```

Force a full rebuild with `--refresh`.

### Pipe into another tool

```bash
aascribe recall "FOCAS" --format json \
  | jq -r '.data.results[] | "\(.id)\t\(.content)"'
```

### Inspect what consolidation will do

```bash
aascribe consolidate --since 1d --dry-run | jq '.data.created[].summary'
```
