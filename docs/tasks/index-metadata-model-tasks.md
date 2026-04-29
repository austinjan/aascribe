# Index Metadata Method Tasks

Status: Draft

Task breakdown for extending `aascribe index` so scan-time metadata can grow through pluggable metadata methods without hardcoding every future field into the core index engine.

## Goal

`index` should remain the main command for understanding repository content.

During scanning, `index` should be able to run one or more metadata methods that add, omit, or refresh optional metadata fields such as category, tags, sensitivity, ownership, routing hints, or stale/active state.

The design target is:

```text
scan file -> run default metadata methods -> run optional metadata methods -> persist local metadata -> report aggregate results
```

## Design Principle

Keep two layers separate:

- default methods: the current `index` behavior that is required for cache correctness and useful baseline summaries
- optional methods: extra agent-facing interpretation produced by rules or LLM calls

Default methods must preserve today's default `index` behavior:

- path
- size
- high-precision mod time
- content hash
- file type
- status
- summary
- summarized timestamp
- not-indexed reason
- error

Optional method metadata can vary by method and use case:

- category
- tags
- confidence
- reason
- sensitivity
- owner/module
- suggested use
- stale/active/reference state
- routing hints
- custom method-specific fields

## Proposed Metadata Shape

Default method output should continue to populate the existing top-level fields on each `MetadataFile` so old callers keep working:

```json
{
  "path": "internal/index/index.go",
  "size": 1234,
  "mod_time": "2026-04-28T03:21:14.123456789Z",
  "content_hash": "sha256:...",
  "file_type": "text/go",
  "summary": "Index engine and metadata persistence.",
  "summarized_at": "2026-04-28T03:21:15Z",
  "status": "ok"
}
```

Each file metadata record should gain a structured `methods` object keyed by method id:

```json
{
  "path": "internal/index/index.go",
  "size": 1234,
  "mod_time": "2026-04-28T03:21:14.123456789Z",
  "content_hash": "sha256:...",
  "file_type": "text/go",
  "summary": "Index engine and metadata persistence.",
  "summarized_at": "2026-04-28T03:21:15Z",
  "status": "ok",
  "methods": {
    "builtin.classify.v1": {
      "generated_at": "2026-04-28T03:21:15Z",
      "input_hash": "sha256:...",
      "status": "ok",
      "fields": {
        "category": "source",
        "tags": ["index", "metadata", "go"],
        "confidence": 0.91,
        "reason": "Implements the index engine and metadata records."
      }
    },
    "builtin.routing.v1": {
      "generated_at": "2026-04-28T03:21:15Z",
      "input_hash": "sha256:...",
      "status": "ok",
      "fields": {
        "suggested_use": "inspect_for_index_behavior",
        "routing_hints": ["metadata", "change-detection"]
      }
    }
  }
}
```

Folder metadata can use the same method namespace for aggregate interpretation:

```json
{
  "version": "index-meta-v1",
  "folder_path": "/repo/internal/index",
  "last_updated": "2026-04-28T03:21:15Z",
  "methods": {
    "builtin.folder-classify.v1": {
      "generated_at": "2026-04-28T03:21:15Z",
      "status": "ok",
      "fields": {
        "category_counts": {
          "source": 4,
          "test": 1
        },
        "dominant_category": "source"
      },
      "input_hash": "sha256:..."
    }
  },
  "files": []
}
```

The first pass should treat this as an additive metadata extension. Do not bump the metadata version unless the implementation makes an incompatible persistence change. Existing `index-meta-v1` files should continue to read successfully, and new method fields should be omitted when absent.

## Metadata Method Contract

In Go, the index engine should depend on a small method interface:

```go
type MetadataMethod interface {
    ID() string
    Scope() MetadataScope
    RequiresSummary() bool
    BuildFile(ctx context.Context, input MetadataMethodInput) (map[string]any, error)
    BuildFolder(ctx context.Context, input MetadataFolderInput) (map[string]any, error)
}

type MetadataMethodInput struct {
    Path        string
    FileName    string
    Content     string
    FileType    string
    ContentHash string
    Summary     string
    NoSummary   bool
}
```

Important contract rules:

- method id must be stable and versioned, such as `builtin.classify.v1`
- method return values are arbitrary key/value fields, but the persisted wrapper must include `generated_at`, `input_hash`, `status`, and `fields`
- method failures should be recorded per file/folder without failing the whole index run
- unchanged files can reuse method metadata only when `input_hash` still matches
- `input_hash` must be derived from the exact inputs that affect the method result, including content hash, method id/version, method config or taxonomy digest, summary/no-summary state, and prompt/classifier version when applicable
- `--refresh` should regenerate method metadata
- dirty metadata should regenerate method metadata
- methods must not mutate source files
- methods that require summaries must not run when `--no-summary` is set unless the method explicitly supports an empty summary input

## CLI Surface

First pass:

```bash
aascribe index <path>
aascribe index <path> --metadata-method classify
aascribe index <path> --metadata-method classify --metadata-method routing
aascribe index <path> --no-metadata-methods
```

Later:

```bash
aascribe index <path> --taxonomy ./taxonomy.toml
aascribe index <path> --metadata-method builtin.classify.v1
aascribe index <path> --metadata-method custom:./methods/security.toml
```

Default behavior should preserve the current index contract:

- the default method set includes the current base metadata and summary behavior
- when no metadata method flags are provided, `index` uses the default method set
- when metadata method flags are provided, `index` uses the requested optional method set instead of auto-enabling optional defaults
- base metadata remains always available
- summary remains enabled by default, must use a real configured LLM summarizer, and is disabled only by `--no-summary`
- summary-enabled indexing must return an error when LLM config or the configured API-key environment variable is unavailable; it must not silently use a local fake summarizer
- built-in classification can become default only after the output/report contract is stable
- `--no-metadata-methods` disables optional methods, not the required base metadata method
- `--no-summary` should disable the default summary method and should not trigger extra summarization work for optional metadata methods; summary-dependent optional methods should be skipped or should run from non-summary inputs only

## Report Additions

The index completion report should add a metadata-method section:

- enabled method ids
- files classified
- folders classified
- method failures
- category counts
- low-confidence items
- uncategorized items
- token usage per method when available

## Map Behavior

`map` should project method metadata into the routing view, but keep it compact:

- show dominant folder category
- show category/tag counts
- include low-confidence warnings
- avoid dumping every method field by default

Detailed method metadata remains available in `.aascribe_index_meta.json`.

## Implementation Tasks

1. Model the current base metadata extraction as a required default method.
2. Model the current summary behavior as a default method that remains enabled unless `--no-summary` is set.
3. Add optional method metadata fields to `Metadata` and `MetadataFile`.
4. Introduce stable Go structs for method output, status, scope, and input hashing.
5. Add a metadata method registry in `internal/index`.
6. Implement a deterministic built-in classifier from path, file type, and summary.
7. Add optional LLM-backed classifier using the existing Gemini summarizer path.
8. Reuse method metadata for unchanged files by checking `input_hash`.
9. Add CLI flags for enabling/disabling optional metadata methods.
10. Thread metadata method flags through async indexing, including `IndexCommand`, `parseIndex`, `parseOperationRunIndex`, `indexArgs`, and async worker tests.
11. Define `--no-summary` behavior for summary-dependent optional methods.
12. Add metadata compatibility tests for reading old files without method fields and writing additive method fields.
13. Extend report generation with method aggregate counts.
14. Extend `map` projection with compact category/tag routing data.
15. Update schemas and docs.

## Open Questions

- Should built-in classification be enabled by default in the first implementation?
- Should method metadata live directly under each file record, or under a folder-level sidecar keyed by path?
- Should custom taxonomy files be TOML, JSON, or Markdown front matter?
- Should low-confidence classification produce warnings, or only report entries?
- Should method metadata affect `index eval`, or should eval remain base-cache-only for now?
