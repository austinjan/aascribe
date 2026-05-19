# Index Metadata Method Tasks

Status: v1-v2 implemented; v3-v4 planned

Task breakdown for extending `aascribe index` so scan-time metadata can grow through pluggable metadata methods without hardcoding every future field into the core index engine.

The durable reference for method concepts is [../reference/index-methods.md](../reference/index-methods.md).

## Goal

`index` should remain the main command for understanding repository content.

During scanning, `index` should be able to run metadata methods that add, omit, or refresh optional metadata fields such as category, tags, sensitivity, ownership, routing hints, or stale/active state.

The long-term design target is:

```text
run optional start hooks -> scan files -> run file methods -> run folder methods -> persist local metadata -> run optional end hooks -> report aggregate results
```

The implementation must be tiered. The first milestone should stabilize the in-process method envelope, registry, discovery output, and report shape before adding index plans, lifecycle hooks, LLM classification, or external subprocess methods.

## Tiers

### v1: In-Process Method Envelope

Implemented.

Scope:

- additive `methods` envelope in `.aascribe_index_meta.json`
- split method interfaces and registry
- deterministic built-in classifier, no LLM
- deterministic built-in folder routing method that rolls up classifier fields
- `--metadata-method <registered-id>` enables registered optional methods such as `builtin.classify.v1`
- `aascribe index methods` discovery command with text and JSON output
- method status enum
- binary/text eligibility rules
- built-in report aggregation for known typed fields
- metadata compatibility tests for old files without method fields
- default index contract unchanged

Out of scope:

- `--index-plan`
- lifecycle start/end hooks
- external/custom methods
- LLM-backed classifier

### v2: Index Plan Validation

Implemented.

Scope:

- `--index-plan <file>` parsing
- `index-plan-v1` TOML model
- `aascribe index plan validate`
- async operation snapshotting of the submitted plan file
- plan path in reports
- plan parsing and validation tests

Out of scope:

- `index plan template`, `index plan schema`, and `index plan explain`
- external method execution
- lifecycle hook execution

### v3: Lifecycle Hooks

Scope:

- start hooks for validation and run-level metadata only
- end hooks for final run-level aggregation and report metadata
- `run_methods` root metadata
- lifecycle failure handling

Decision: v3 start hooks do not feed downstream file/folder method input. They can validate and emit run-level metadata, but they do not mutate runtime context. Any future downstream dependency model needs a real dependency graph and is not part of this milestone.

Decision: run-level lifecycle output lives only in the root `.aascribe_index_meta.json`.

### v4: LLM And External Methods

Scope:

- LLM-backed classifier through the existing Gemini summarizer path
- explicit custom/external method definitions through `[[methods.custom]]`
- subprocess protocol, timeouts, output limits, and env allowlists
- tests proving method discovery does not execute external programs

`map` projection can ship earlier once v1 method fields are stable. It does not need to wait for v4.

## Design Principles

Keep these layers separate:

- required base metadata: cache correctness and existing caller compatibility
- file methods: per-file interpretation
- folder methods: aggregate folder interpretation
- lifecycle hooks: optional run-level validation and reporting

Default behavior must preserve today's `index` contract:

- base metadata always runs
- passing `--metadata-method base` is accepted but is a no-op because base always runs
- summary remains enabled by default unless `--no-summary` or `--no-metadata-methods` disables it
- summary remains the existing inline hot path in v1, not a `FileMethod` implementation
- summary-enabled indexing must use a real configured LLM summarizer
- missing LLM config or missing configured API-key environment variable must return an error, not a local fake summary
- `builtin.classify.v1` does not require a summary in v1; it should degrade gracefully to path and file-type signals when summary is unavailable
- built-in classification can become default only after the output/report contract is stable

## Metadata Shape

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

Each file metadata record should gain an additive `methods` object keyed by method id:

```json
{
  "path": "internal/index/index.go",
  "status": "ok",
  "methods": {
    "builtin.classify.v1": {
      "stage": "file",
      "generated_at": "2026-04-28T03:21:15Z",
      "input_hash": "sha256:...",
      "status": "ok",
      "fields": {
        "category": "source",
        "tags": ["index", "metadata", "go"],
        "confidence": 0.91,
        "reason": "Implements the index engine and metadata records."
      }
    }
  }
}
```

Folder metadata can use the same method envelope:

```json
{
  "version": "index-meta-v1",
  "folder_path": "/repo/internal/index",
  "methods": {
    "builtin.routing.v1": {
      "stage": "folder",
      "generated_at": "2026-04-28T03:21:15Z",
      "input_hash": "sha256:...",
      "status": "ok",
      "fields": {
        "category_counts": {
          "source": 4,
          "test": 1
        },
        "dominant_category": "source"
      }
    }
  },
  "files": []
}
```

v3 lifecycle output should live only in the root metadata file:

```json
{
  "version": "index-meta-v1",
  "folder_path": "/repo",
  "run_methods": {
    "plan.final-summary.v1": {
      "stage": "end",
      "generated_at": "2026-04-28T03:23:02Z",
      "input_hash": "sha256:...",
      "status": "ok",
      "fields": {
        "recommended_next_commands": [
          "aascribe map .",
          "aascribe search security ."
        ]
      }
    }
  },
  "files": []
}
```

This is an additive metadata extension. Do not bump `index-meta-v1` unless the implementation makes an incompatible persistence change. Existing metadata files must continue to read successfully, and new method fields should be omitted when absent.

## Method Interfaces

Do not use one interface with `Start`, `BuildFile`, `BuildFolder`, and `End` no-ops. The roles have different inputs, output destinations, and failure semantics.

Use a tiny shared core plus role interfaces:

```go
type Method interface {
    ID() string
    Fingerprint() string
    Describe() MethodDescriptor
}

type FileMethod interface {
    Method
    BuildFile(ctx context.Context, input FileMethodInput) (MethodFields, error)
}

type FolderMethod interface {
    Method
    BuildFolder(ctx context.Context, input FolderMethodInput) (MethodFields, error)
}

type LifecycleStage string

const (
    LifecycleStart LifecycleStage = "start"
    LifecycleEnd   LifecycleStage = "end"
)

type LifecycleMethod interface {
    Method
    Stage() LifecycleStage
    Run(ctx context.Context, input LifecycleInput) (MethodFields, error)
}
```

`MethodDescriptor` is the registry/discovery contract used by both engine logic and `aascribe index methods` output. It should include id, stage, scope, whether the method is required, whether it is always enabled, whether it is default-enabled, whether it requires a summary, whether it supports no-summary mode, a short `description`, a concise `purpose`, and config schema metadata.

`MethodFields` is the method field-map payload persisted under `fields`. v1 can define it as `map[string]any`; typed accessors or a struct with known soft-schema fields plus an `Extra` bag can be added later if report aggregation needs more structure.

`FileMethodInput` should include the stable file facts that can affect a file method: normalized path, file name, file type, content hash, summary text when available, no-summary state, and any method-selected text content. File methods receive only text/status-eligible files in v1.

`FolderMethodInput` should include normalized folder path plus the stable direct child file/folder records needed for aggregation, including their selected method envelopes. v1 folder methods may re-run every indexing pass if reuse is not implemented, but when folder reuse is added its `input_hash` must be based on this canonical folder input.

`LifecycleInput` is v3-only. Start lifecycle input should include run options, plan digest, and method config. End lifecycle input should include the final root tree, run stats, report aggregates, and plan digest.

The registry holds all methods and dispatches by role interface. `aascribe index methods --stage file|folder|start|end` is type filtering over the registry.

`Fingerprint()` is the method-owned config digest. It must fold in the method implementation version plus any config that can affect output, such as taxonomy path/content digest, prompt version, plan section, external command line, or declared external artifacts.

Input hashing rule:

```text
input_hash = sha256(canonical(stage-input) || ID() || Fingerprint())
```

This single rule applies to file, folder, and lifecycle methods.

`canonical(...)` means stable JSON serialization with lexically sorted object keys, no insignificant whitespace, and fixed timestamp and number formatting. Timestamp fields must use RFC 3339 nanosecond UTC formatting when present. Volatile fields such as `generated_at`, run timing, and transient run stats must be excluded. Aggregating methods should hash child method `input_hash` and stable `fields`, not child `generated_at`. The exact helper should be centralized so all method stages hash inputs the same way.

## Method Status

Method output status must use an explicit enum:

- `ok`: method ran and returned fields
- `failed`: method ran but returned an error
- `skipped`: method intentionally did not run, such as a summary-dependent method under `--no-summary`

Omitting a method key means the method was not selected for that run. Use `skipped` when the method was selected but gated by run options or file eligibility.

## File Eligibility

v1 file methods run only for text/status-eligible file records.

Current binary behavior remains: binary files are recorded as not indexed with the existing reason, and summary/classification-style methods should either be omitted when not selected or recorded as `skipped` when selected but ineligible.

If a future method wants binary input, it must explicitly declare binary support in its registry metadata and document its input contract.

## Report Aggregation

The report cannot aggregate arbitrary `map[string]any` fields.

v1 report aggregation should understand only built-in typed fields:

- `category`
- `tags`
- `confidence`
- `reason`

These are a documented soft schema for `builtin.classify.v1` and any method that opts into the same aggregation contract. Arbitrary custom fields remain persisted in metadata but are not counted by the generic report unless the method declares an aggregation contribution in a later milestone.

## CLI Surface

Current behavior:

```bash
aascribe index <path>
aascribe index <path> --metadata-method base
aascribe index <path> --metadata-method summary
aascribe index <path> --no-metadata-methods
```

v1 optional method enablement:

```bash
aascribe index <path> --metadata-method builtin.classify.v1
aascribe index <path> --metadata-method builtin.classify.v1 --metadata-method builtin.retrieval.v1 --metadata-method builtin.routing.v1
aascribe index <path> --metadata-method summary --metadata-method builtin.classify.v1
```

`--metadata-method` should validate against the method registry instead of a hardcoded `base`/`summary` allowlist. Unknown method ids remain parse errors with actionable suggestions.

v1 discovery:

```bash
aascribe index methods
aascribe index methods --stage file
aascribe index methods --stage folder
aascribe index methods --stage start
aascribe index methods --stage end
aascribe --format json index methods
```

`index methods` output must include descriptions, not just ids. Text output should show each method id, stage, default/required flags, and a short description. JSON output should expose the full `MethodDescriptor` so an AI can choose methods without reading hand-maintained docs.

Example text:

```text
available index methods:
- builtin.classify.v1
  stage: file
  default_enabled: false
  requires_summary: false
  description: Classifies each text file for agent routing using path, file type, and optional summary signals.

- builtin.retrieval.v1
  stage: file
  default_enabled: false
  requires_summary: false
  description: Emits deterministic LLM-facing retrieval hints from path, summary, and bounded file text.

- builtin.routing.v1
  stage: folder
  default_enabled: false
  requires_summary: false
  description: Aggregates child file classifications into folder-level category and routing hints.
```

Example JSON:

```json
{
  "methods": [
    {
      "id": "builtin.classify.v1",
      "stage": "file",
      "scope": "file",
      "required": false,
      "default_enabled": false,
      "always_enabled": false,
      "requires_summary": false,
      "supports_no_summary": true,
      "description": "Classifies each text file for agent routing using path, file type, and optional summary signals.",
      "purpose": "file_classification",
      "config_schema": {
        "low_confidence_threshold": "number"
      }
    }
  ]
}
```

v2 plan validation:

```bash
aascribe index <path> --index-plan ./aascribe.index.toml
aascribe index plan validate ./aascribe.index.toml
```

Defer these helper commands until the plan format stabilizes:

```bash
aascribe index plan template
aascribe index plan schema
aascribe index plan explain
```

## Index Plan Shape

Plan files should be TOML only in `index-plan-v1`.

Built-in method stage and scope are registry facts. Do not repeat them in built-in method config.

```toml
version = "index-plan-v1"

[methods]
enabled = [
  "summary",
  "builtin.classify.v1",
  "builtin.retrieval.v1",
  "builtin.routing.v1"
]

[methods."summary"]
style = "medium"

[methods."builtin.classify.v1"]
taxonomy = "./taxonomy.toml"
low_confidence_threshold = 0.6

[methods."builtin.routing.v1"]
include_tags = true
```

Custom/external methods should be declared only through `[[methods.custom]]` in a plan. Do not add a separate `--metadata-method custom:...` code path; if a CLI shortcut is ever needed, make it syntactic sugar that builds a one-method plan internally.

```toml
version = "index-plan-v1"

[[methods.custom]]
id = "custom.security.v1"
stage = "file"
scope = "file"
requires_summary = true
command = ["./methods/security-check", "--format", "json"]
timeout = "5s"
```

Relative paths inside a plan resolve from the plan file directory.

## Async Plan Snapshot

When `--async` is used with `--index-plan`, the submitter must snapshot the plan file into the operation directory before returning `OperationAccepted`.

The detached worker must read the snapshotted copy, not the original path. This avoids a race where the plan changes between submission and worker startup, and it records exactly what ran.

The external execution safety guarantee applies to async workers too: external programs may execute only during validated `aascribe index --index-plan <file>` execution, never during method listing or plan validation.

## AI-Generated Plans

Because `aascribe` is primarily called by AI agents, guidance must be discoverable from the CLI and not only from docs.

The agent workflow should be:

```text
list methods -> read plan guidance -> inspect customer/repo goal -> draft index-plan-v1 TOML -> validate plan -> run index with --index-plan
```

The source of truth for current method capabilities is JSON output from `aascribe index methods`, not the hand-maintained tables in [../reference/index-methods.md](../reference/index-methods.md).

## Implementation Tasks

### v1 Tasks

1. Add method envelope structs for file/folder metadata with `stage`, `generated_at`, `input_hash`, `status`, and `fields`.
2. Add method status enum: `ok`, `failed`, `skipped`.
3. Add `Method`, `FileMethod`, and `FolderMethod` role interfaces with `Fingerprint()`.
4. Add `MethodDescriptor` and use it for discovery output, `--no-summary` skip logic, and registry validation.
5. Define `MethodFields`, `FileMethodInput`, and `FolderMethodInput`.
6. Add input hashing helper using `canonical(stage-input) || ID() || Fingerprint()`.
7. Implement stable canonical serialization for method input hashing, excluding volatile fields from aggregate inputs.
8. Model current base extraction as required metadata outside optional method selection while preserving top-level fields.
9. Keep current summary behavior as the existing inline hot path while preserving selection semantics.
10. Add before/after regression tests proving unchanged summary-enabled indexing preserves the existing top-level metadata shape.
11. Add deterministic built-in classifier from path, file type, and optional summary; it must not require summaries.
12. Add deterministic `builtin.routing.v1` folder method that rolls up child `category`, `tags`, and `confidence` fields into category/tag counts, dominant category, low-confidence counts, and uncategorized counts.
13. Add method registry in `internal/index`.
14. Change `parseIndex` validation from the hardcoded `base`/`summary` allowlist to registered method ids, while keeping `base` as an accepted no-op.
15. Reuse file method metadata for unchanged files by checking method `input_hash`.
16. Punt folder method reuse in v1 unless `FolderMethodInput` canonical hashing is implemented; folder methods may re-run each pass.
17. Define binary/text file eligibility and summary-gated `skipped` behavior from method descriptors.
18. Add `aascribe index methods` text and JSON output.
19. Add `--stage` filtering for `index methods`.
20. Add metadata compatibility tests for reading old files without method fields and writing additive method fields.
21. Add tests for `index methods` output and the guarantee that listing methods does not execute external programs.
22. Extend report generation with built-in method counts using the documented classification/routing soft schema.
23. Update docs/spec/index-spec.md and docs/reference/index-methods.md for v1 behavior. No new `.schema.json` file is required for this task unless the command output shape changes.

### v2 Tasks

1. Add `--index-plan <file>` parsing and `index-plan-v1` TOML structs.
2. Add `aascribe index plan validate`.
3. Validate unknown plan version, unknown method id, invalid config, duplicate custom id, and invalid relative paths.
4. Validate that `--index-plan` is mutually exclusive with `--metadata-method` and `--no-metadata-methods`.
5. Thread plan path through `IndexCommand`, `parseIndex`, `parseOperationRunIndex`, `indexArgs`, and worker tests.
6. Snapshot plan files into the operation directory for async runs.
7. Add report output for plan path and plan digest.

### v3 Tasks

1. Add `LifecycleMethod` and root-only `run_methods` metadata.
2. Add start hooks for validation/run-level metadata only.
3. Add end hooks for final aggregation/report metadata.
4. Add required/optional lifecycle failure handling.
5. Add tests that lifecycle hooks do not mutate downstream file/folder method input.

### v4 Tasks

1. Add LLM-backed classifier using the existing Gemini summarizer path.
2. Add custom/external method descriptors through `[[methods.custom]]`.
3. Add subprocess protocol, timeout, stdout/stderr limits, and env allowlists.
4. Add tests for external method validation and execution failures.
5. Add optional `map` projection once method fields are stable, if not already shipped earlier.

## Open Questions

- Should built-in classification be enabled by default after v1 output/report behavior stabilizes?
- Should low-confidence classification produce warnings, or only report entries?
- Should method metadata affect `index eval`, or should eval remain base-cache-only for now?
