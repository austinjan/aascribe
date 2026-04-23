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
  - [index](#index)
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
| `--format json\|text` | `json` | Output format. `json` is agent-friendly; `text` is human-readable |
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

Every JSON response uses this shape:

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

Create a new store. Sets up a filesystem-first directory layout for short-term memory, long-term memory, index data, cache data, and layout metadata.

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
    "path": "/Users/you/project-mem/logs/aascribe.log"
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
    "source_path": "/Users/you/project-mem/logs/aascribe.log",
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
    "path": "/Users/you/project-mem/logs/aascribe.log",
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

### `index`

Walk a directory and return a tree of paths with per-file summaries.

```
aascribe index <path> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `<path>` | — | Directory to index (positional, required) |
| `--depth <N>` | `3` | Recursion depth. `0` = current level only. `-1` = unlimited |
| `--include <glob>` | — | Include only matching files. Repeatable |
| `--exclude <glob>` | `.git`, `node_modules`, `target`, `dist`, `.venv` | Exclude matching files/dirs. Repeatable |
| `--refresh` | off | Ignore cache; regenerate summaries |
| `--no-summary` | off | Return structure only. Much faster |
| `--max-file-size <bytes>` | `1048576` | Files over this size get metadata only, no summary |

**Example**

```bash
aascribe index ./src --depth 2 --include '*.rs' --include '*.toml'
aascribe index . --no-summary           # structure-only, fast
aascribe index . --refresh              # force re-summarize
```

**OutputShape:** `PathIndexTree`

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

**Example**

```bash
aascribe describe ./src/poll.rs
aascribe describe ./src/poll.rs --length long --focus "FOCAS retry logic"
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
