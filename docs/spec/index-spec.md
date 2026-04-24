# Index Command Specification

Specification for the `aascribe index` command and the supporting `internal/index` package. Covers CLI surface, traversal behavior, summarization, persisted metadata, failure handling, and the `index clean` subcommand.

## 1. Purpose

`aascribe index` walks one directory tree and produces:

1. A `PathIndexTree` returned to the caller (rendered as JSON or text).
2. A `.aascribe_index_meta.json` sidecar written into the indexed root, recording every file the indexer saw, every file it declined to summarize, and every file that failed mid-run.

The output is designed to be consumed either by a human via the CLI or by another agent reading structured JSON.

## 2. CLI surface

### 2.1 `aascribe index <path>`

```
aascribe index <path>
  [--depth <n>]
  [--include <glob>]...
  [--exclude <glob>]...
  [--refresh]
  [--no-summary]
  [--max-file-size <bytes>]
```

| Flag              | Type       | Default     | Meaning                                                                 |
| ----------------- | ---------- | ----------- | ----------------------------------------------------------------------- |
| `<path>`          | positional | required    | Directory to index. Must exist and must be a directory.                  |
| `--depth`         | int        | `3`         | Maximum directory depth. `0` = root only; negative = unlimited.          |
| `--include`       | glob (×N)  | `[]`        | Whitelist applied to files only. Empty list means "include all files."   |
| `--exclude`       | glob (×N)  | `[]`        | Blacklist applied to both files and directories.                         |
| `--refresh`       | bool       | `false`     | Reserved for incremental re-indexing. Accepted but not yet consumed.     |
| `--no-summary`    | bool       | `false`     | Skip all summarization. Files are still walked, hashed, and recorded.    |
| `--max-file-size` | int64      | `1_048_576` | Byte ceiling for summarization. `0` is valid; negative is rejected.      |

Exactly one positional path argument is required. Zero or multiple positional args return a parse error.

### 2.2 `aascribe index clean <path>`

```
aascribe index clean <path> [--dry-run] --force
```

| Flag        | Type       | Default  | Meaning                                                    |
| ----------- | ---------- | -------- | ---------------------------------------------------------- |
| `<path>`    | positional | required | Directory to sweep for `.aascribe_index_meta.json` files.  |
| `--dry-run` | bool       | `false`  | Report what would be removed; do not delete.               |
| `--force`   | bool       | required | Required because deletion is destructive. Absent → error.   |

## 3. Traversal behavior

### 3.1 Root resolution

- The `<path>` argument is resolved to an absolute path.
- Must exist (`PathNotFound` if not).
- Must be a directory (`InvalidArguments` if it is a file).

### 3.2 Default exclusions

These names are always excluded before any user exclude rules are applied:

- `.git`
- `node_modules`
- `target`
- `dist`
- `.venv`
- `.aascribe_index_meta.json`

### 3.3 Ignore files

During the single `Build` call the indexer reads, if present at the root:

1. `.gitignore`
2. `.aaignore`

From each file it extracts non-empty, non-comment lines. A leading `/` is stripped so patterns become repo-relative globs. Patterns ending in `/` match the directory and everything beneath it.

**First-pass limitations (intentional):**

- Only blacklist entries are supported. Negation (`!pattern`) lines are parsed and discarded.
- Patterns are only read from the top-level root — nested `.gitignore` files in subdirectories are not consulted.

### 3.4 Include / exclude matching

`matcher.skip(name, displayPath, isDir)` decides whether an entry is skipped:

1. If any exclude pattern matches → skip.
2. For directories: never filtered by `--include`.
3. For files with a non-empty `--include` list: include only when at least one include pattern matches; otherwise skip.
4. With an empty `--include` list: all non-excluded files are kept.

A pattern matches when any of these is true:

- The pattern (trimmed of a trailing `/`) equals the display path, prefixes it with `/`, or appears as a `/X/` segment.
- `filepath.Match(pattern, name)` matches the basename.
- `filepath.Match(pattern, displayPath)` matches the relative display path.
- If the pattern has no `/`, it matches any path segment.

### 3.5 Depth enforcement

- `depth == 0` keeps only immediate children of the root.
- `depth > 0` allows descent up to that many levels below the root.
- `depth < 0` imposes no limit.

### 3.6 Child ordering

Children are sorted so directories appear before files; within the same type, by ascending `Path`.

## 4. Per-file analysis

For each file kept by the matcher, the indexer produces a `fileAnalysis` with:

- `path` — display path (forward slashes), relative to root display.
- `size` — `os.FileInfo.Size()`.
- `modTime` — RFC3339 UTC.
- `hash` — `"sha256:" + hex(sha256(content))`.
- `fileType` — see §4.2.
- `summary` / `generatedAt` — only populated when the file is summarizable.
- `notIndexed` — reason code when the file is skipped from summarization.

### 4.1 Binary vs text detection

A file is considered **binary** when:

- Content is non-empty, AND
- Content is not valid UTF-8, OR the first 1024 bytes contain a `0x00` byte.

Empty files are treated as text.

### 4.2 `file_type` values

- Binary content → `"binary"`.
- Text with no extension → `"text/plain"`.
- Text with extension `.ext` → `"text/<ext>"` (lowercased, dot stripped).

### 4.3 `not_indexed` reasons

A file passes traversal but is still not summarized when `notIndexedReason` returns one of:

| Reason                    | Trigger                                                             |
| ------------------------- | ------------------------------------------------------------------- |
| `binary_file`             | Binary content detected (see §4.1).                                  |
| `max_file_size_exceeded`  | `--max-file-size >= 0` and `len(content) > --max-file-size`.         |
| `no_summary_mode`         | `--no-summary` was passed. Hash/size still recorded.                 |

### 4.4 Summarization

When a file is summarizable:

- If `Options.Summarizer` is non-nil, it is called with `(path, content, length, focus)`. `length` is always `"medium"` and `focus` is always `""` during indexing; both exist for reuse by `DescribeFile*`. The returned summary is trimmed and used verbatim.
- If the summarizer is `nil`, a heuristic fallback in `summarizeFile` / `adjustSummaryLength` constructs a short summary from the first non-blank line and the extension.
- If the summarizer returns an error, the file is recorded as a **failed file** (§6), but indexing continues.

## 5. Output shapes

### 5.1 `PathIndexTree`

```json
{
  "root": "/abs/path/to/indexed/root",
  "tree": { IndexedPathNode }
}
```

### 5.2 `IndexedPathNode`

```json
{
  "path": "display/path",
  "type": "dir" | "file",
  "size": 1234,                      // files only
  "hash": "sha256:...",              // files only
  "summary": "...",                  // omitted when --no-summary or skipped
  "summarized_at": "RFC3339",        // paired with summary
  "children": [ IndexedPathNode ]    // dirs only
}
```

Directory nodes carry a generated summary from `summarizeDir`:

- Empty directory → `"Empty directory."`
- Otherwise counts of files and/or subdirectories (with singular/plural handling).

When `--no-summary` is active, directory `summary` and `summarized_at` are omitted as well.

## 6. Metadata file: `.aascribe_index_meta.json`

Written atomically at the indexed root (`.tmp` then `rename`). Schema:

```json
{
  "version": "index-meta-v1",
  "folder_path": "/abs/path/to/indexed/root",
  "last_updated": "RFC3339",
  "files":            [ MetadataFile ],
  "not_indexed_files":[ { "path": "...", "reason": "..." } ],
  "failed_files":     [ { "path": "...", "error": "..." } ],
  "warnings":         [ "..." ]
}
```

### 6.1 `MetadataFile`

```json
{
  "path": "display/path",
  "size": 1234,
  "mod_time": "RFC3339",
  "content_hash": "sha256:...",
  "file_type": "text/go" | "binary" | ...,
  "summary": "...",           // present when status == "ok" and --no-summary is off
  "summarized_at": "RFC3339", // paired with summary
  "status": "ok" | "not_indexed" | "failed",
  "not_indexed_reason": "binary_file" | "max_file_size_exceeded" | "no_summary_mode" | "",
  "error": "..."              // populated when status == "failed"
}
```

### 6.2 Status semantics

| `status`      | Meaning                                                                 |
| ------------- | ----------------------------------------------------------------------- |
| `ok`          | File was summarized. `summary` and `summarized_at` are populated.        |
| `not_indexed` | File walked but summarization skipped. `not_indexed_reason` is set.      |
| `failed`      | Summarizer (or directory walk) returned an error. `error` is populated.  |

Every file the indexer encountered appears once in `files`. `not_indexed_files` and `failed_files` are redundant projections of the `not_indexed` and `failed` entries, provided as convenience for consumers.

## 7. Failure handling

### 7.1 Hard failures (abort)

The command returns an error and writes no metadata when:

- `Root` is empty.
- `MaxFileSize` is negative.
- The resolved root does not exist or is not a directory.
- An ignore file exists but cannot be read.
- Metadata cannot be serialized or written.

### 7.2 Soft failures (continue + record)

During traversal, any error from `buildDir` or `buildFile` for a single entry:

1. Appends a `FailedFile` entry under `failed_files`.
2. Appends a metadata `files` row with `status: "failed"` and the error string.
3. Notifies the failure tracker (§7.3).
4. Traversal moves on to the next sibling.

### 7.3 Consecutive-failure warning

`failureTracker` counts failures since the last success:

- Default threshold: `3`. Callers can override via `Options.FailureThreshold` (any value `<= 0` reverts to `3`).
- On reaching the threshold, one warning is appended to `metadata.Warnings` and (if `FailureNoticeWriter` is set) written to that writer:
  `"Index encountered N consecutive failures. Please check the files, permissions, ignore rules, or LLM/config setup."`
- The warning is emitted **once** per streak. A successful entry resets the counter and re-arms the warning.

CLI wiring passes `os.Stderr` as `FailureNoticeWriter` so agents see the notice on stderr alongside structured output on stdout.

## 8. Package API

`internal/index` exports:

- `Build(opts Options) (*PathIndexTree, error)` — the indexer entry point.
- `CleanArtifacts(path string, dryRun bool) (*CleanResult, error)` — recursive metadata sweep used by `aascribe index clean`.
- `DescribeFile(path, length, focus string) (*FileDescription, error)` — single-file describe with the heuristic summarizer.
- `DescribeFileWithSummarizer(path, length, focus string, summarizer SummarizerFunc) (*FileDescription, error)` — same, but allows injecting an LLM summarizer (used by the `describe` CLI).

Types:

```go
type Options struct {
    Root                string
    Depth               int
    Include             []string
    Exclude             []string
    Refresh             bool
    NoSummary           bool
    MaxFileSize         int64
    Summarizer          SummarizerFunc
    FailureThreshold    int
    FailureNoticeWriter io.Writer
}

type SummarizerFunc func(path, content, length, focus string) (string, error)
```

`Summarizer` is called with `length="medium"`, `focus=""` from `Build`. Implementations must return a non-empty string on success or an `error` to signal a per-file failure.

## 9. `index clean` behavior

`CleanArtifacts(path, dryRun)`:

- Resolves and validates that `path` is an existing directory.
- Walks the tree. For every regular file named `.aascribe_index_meta.json`:
  - Always appended to `CleanResult.RemovedPaths`.
  - Deleted unless `dryRun` is true.
- Returns:

```json
{
  "root": "/abs/path",
  "removed_paths": ["..."],
  "removed_count": N,
  "dry_run": true | false
}
```

CLI text output:

- Dry run: `Would remove N index artifact(s) under <root>`
- Real run: `Removed N index artifact(s) under <root>`

`--force` is enforced at parse time and is required even for `--dry-run`.

## 10. Rendering

`renderIndexText` in `internal/command/command.go` turns `PathIndexTree` into an indented, depth-prefixed text tree. Directory rows get a trailing `/`; rows with a summary append ` - <summary>`. This is the text returned when the CLI is invoked with `--format text`; JSON output is the full `PathIndexTree` structure.

## 11. Error codes (via `internal/apperr`)

- `InvalidArguments` — missing path, negative `--max-file-size`, path is a file, describe called on a binary file, etc.
- `PathNotFound` — root does not exist.
- `IOError` — filesystem read/write failures (resolve, stat, read, rename, walk).
- `Serialization` — metadata JSON marshal failure.

## 12. Out of scope for v1

The current implementation deliberately defers:

- `--refresh` incremental semantics (flag accepted but unused).
- Respecting nested `.gitignore` files beneath the root.
- `!pattern` negation in ignore files.
- Parallel traversal or summarization.
- Emitting metadata for excluded or depth-capped entries.
