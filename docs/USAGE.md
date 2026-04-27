# aascribe Usage

Related docs:
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
  - [logs](#logs)
  - [operation](#operation)
  - [index](#index)
  - [map](#map)
  - [describe](#describe)
  - [remember](#remember)
  - [consolidate](#consolidate)
  - [recall](#recall)
  - [list](#list)
  - [show](#show)
  - [forget](#forget)
- [Recipes](#recipes)

---

## Global flags

Available on every subcommand:

| Flag | Default | Description |
|---|---|---|
| `--store <path>` | `~/.aascribe` or `$AASCRIBE_STORE` | Path to the memory store root |
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
    "store": "/Users/you/.aascribe"
  }
}
```

On error:

```json
{
  "ok": false,
  "error": {
    "code": "STORE_NOT_FOUND",
    "message": "No store at /Users/you/.aascribe. Run `aascribe init`."
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

```
aascribe init [--store <path>] [--force]
```

| Flag | Description |
|---|---|
| `--store <path>` | Where to create the store (default: `~/.aascribe`) |
| `--force` | Reinitialize even if a store already exists (destructive) |

**Example**

```bash
aascribe init
aascribe init --store ./project-mem
```

**OutputShape:** `StoreInitResult`

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
    "store": "/Users/you/.aascribe",
    "created": true,
    "reinitialized": false,
    "layout_version": "bootstrap-v1"
  },
  "meta": {
    "command": "init",
    "duration_ms": 4,
    "store": "/Users/you/.aascribe"
  }
}
```

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

**OutputShape:** `LogPathResult` for `logs path`

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

**OutputShape:** `LogExportResult` for `logs export`

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

**OutputShape:** `LogClearResult` for `logs clear`

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

Behavior notes:

- Operation lifecycle state is stored under `<store>/operations/<operation-id>/`.
- `operation.json` is the latest compact status snapshot.
- `events.jsonl` is append-only event history.
- `result.json` is written only when a final result is available. Small results may include `data`; oversized results store data through the managed `output` transport and return `output_id`.
- `operation result` returns a typed error if the operation has not completed yet.
- When `operation result` shows `output_id`, use `aascribe output show <output-id>` or `aascribe output slice <output-id> --offset 0 --limit 4000` to inspect the stored payload.
- `operation cancel` is idempotent for already canceled operations and rejects operations that already succeeded or failed.

Examples:

```bash
aascribe operation list
aascribe operation status op_20260424T120000Z_ab12cd34
aascribe operation events op_20260424T120000Z_ab12cd34
aascribe operation result op_20260424T120000Z_ab12cd34
aascribe operation cancel op_20260424T120000Z_ab12cd34
```

Output shapes:

- `operation list`: `OperationList`
- `operation status`: `OperationStatus`
- `operation events`: `OperationEventList`
- `operation result`: `OperationResult`
- `operation cancel`: `OperationCancelResult`

---

### `index`

Walk a directory, summarize direct files per folder, and persist local metadata.

```
aascribe index <path> [flags]
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
| `--max-file-size <bytes>` | `1048576` | Files over this size get metadata only, no summary |

Behavior notes:

- `index` treats `.gitignore` and `.aaignore` as folder-scoped blacklist exclude files during traversal.
- Text-file detection is content-based, not extension-based.
- `index` writes `.aascribe_index_meta.json` files in indexed directories.
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
aascribe index --refresh .              # force re-summarize
aascribe index dirty ./src              # mark existing metadata stale
aascribe index eval ./src               # preview changed vs unchanged work
```

**OutputShape:** `PathIndexTree` for synchronous `index`

**OutputShape:** `OperationAccepted` for `index --async`

**Output**

```json
{
  "ok": true,
  "data": {
    "root": "/abs/path/to/src",
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
aascribe map <path>
```

Behavior notes:

- `map` is a routing overview for agents. Use it to choose the folder or file to inspect next; for precise answers, inspect the target file directly or re-run `index` without `--no-summary`.
- `map` reads metadata files; it does not re-summarize source files.
- `map` applies `.gitignore` and `.aaignore` while traversing child directories.
- `text` is the default output and returns a compact tree intended to be easier for LLMs to read.
- `--format json` returns a machine-friendly assembled projection.
- `index map <path>` is supported as an alias, but `map <path>` is the preferred surface.
- map node states are simple on purpose:
  - `ready`: local metadata exists
  - `dirty`: local metadata exists but was explicitly marked stale
  - `unindexed`: no local metadata file exists yet

**Example**

```bash
aascribe map ./tests
aascribe --format text map ./tests
aascribe --format json map ./docs
```

**OutputShape:** `PathIndexMap`

**JSON Output**

```json
{
  "ok": true,
  "data": {
    "root": "/abs/path/to/tests",
    "state_guide": {
      "dirty": "Metadata exists but is marked stale. Re-run index before trusting it.",
      "ready": "Metadata exists for this directory. Use the summary and files shown here first.",
      "unindexed": "No metadata file exists for this directory yet. If needed, inspect the directory directly or run index on it."
    },
    "map": {
      "path": "/abs/path/to/tests",
      "state": "ready",
      "folder_description": "Contains 1 subdirectory.",
      "brief_summary": "Contains 1 subdirectory.",
      "stats": {
        "direct_dir_count": 1
      },
      "children": [
        {
          "path": "/abs/path/to/tests/index-fixtures",
          "state": "ready",
          "brief_summary": "Contains 4 files and 2 subdirectories."
        }
      ]
    }
  }
}
```

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

**OutputShape:** `FileDescription`

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

**OutputShape:** `RememberResult`

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

**OutputShape:** `ConsolidationResult`

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

**OutputShape:** `MemoryRecallResult`

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

**OutputShape:** `MemoryEntryList`

---

### `show`

Show a single memory entry in full. `recall` and `list` output may be truncated for display; `show` gives you everything.

```
aascribe show <id>
```

| Flag | Description |
|---|---|
| `<id>` | Memory id (positional, required). Accepts `stm_...` or `ltm_...` |

**OutputShape:** `MemoryEntryDetail`

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

**OutputShape:** `ForgetResult`

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
