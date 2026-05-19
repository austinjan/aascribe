# Index Methods

Status: Proposed design reference

This document describes the `aascribe index` method model: what index methods are, how file and folder methods differ, how lifecycle hooks work, and what an AI agent needs to know before generating an index plan.

The current implementation supports the built-in `base`, `summary`, `builtin.classify.v1`, and `builtin.routing.v1` methods plus the `aascribe index methods` discovery command. `--index-plan`, lifecycle hooks, LLM-backed classifiers, and custom/external methods remain planned work tracked in [../tasks/index-metadata-model-tasks.md](../tasks/index-metadata-model-tasks.md).

`aascribe --format json index methods` is the source of truth for currently available methods. This document is orientation and design guidance.

## Purpose

`aascribe index` prepares repository or folder content so future AI agents can navigate, search, summarize, and answer questions from local metadata.

Index methods are the extension points used during an index run. They let the index process add structured metadata without hardcoding every future field into the core traversal engine.

Examples of metadata a method might produce:

- file summaries
- categories
- tags
- sensitivity hints
- owner or module hints
- routing hints
- stale, active, or reference status
- folder-level category counts
- final run summaries and next-command recommendations

## Method Stages

Every method belongs to one explicit stage.

```text
start -> file -> folder -> end
```

### Start Methods

Start methods run once before traversal begins.

Use them for run-level preparation:

- validate the index plan
- load taxonomy or customer context
- check required config
- produce plan-level warnings

Start method output belongs to run-level metadata, not to individual file records.

In the first lifecycle implementation, start methods do not feed downstream file or folder method input. They validate and emit run-level metadata only.

### File Methods

File methods run for each indexed file.

Use them for per-file interpretation:

- record base file metadata
- summarize file content
- classify a file
- tag a file
- identify likely owner or module
- detect security-sensitive or generated files

File method output is stored under that file's `methods` object in `.aascribe_index_meta.json`.

### Folder Methods

Folder methods run for a folder-level view.

Use them for aggregate interpretation:

- dominant folder category
- category counts
- tag counts
- routing hints
- low-confidence warnings
- module-level summaries

Folder method output is stored on folder metadata, separate from per-file method output.

### End Methods

End methods run once after traversal and method execution complete.

Use them for final aggregation:

- summarize the index run
- produce customer-facing notes
- recommend next commands
- aggregate low-confidence or uncategorized items
- write report-only metadata

End method failures should not invalidate already-written file metadata. They should be recorded in run-level metadata and the completion report.

## Built-In Methods

The first implemented methods are:

| ID | Stage | Required | Default | Summary required | Purpose |
| --- | --- | --- | --- | --- | --- |
| `base` | `file` | yes | yes | no | Records path, size, mod time, content hash, file type, status, skip reason, and error state. |
| `summary` | `file` | no | yes | no | Produces file summaries through the configured LLM summarizer. |
| `builtin.classify.v1` | `file` | no | no | no | Classifies text/status-eligible files for agent routing using deterministic path, file type, and optional summary signals. |
| `builtin.routing.v1` | `folder` | no | no | no | Aggregates child classifier output into category counts, tag counts, dominant category, low-confidence counts, and uncategorized counts. |

`base` always runs. Passing `--metadata-method base` is accepted for compatibility but is a no-op.

`summary` is part of method selection, but it remains on the existing inline hot path that writes top-level `summary` and `summarized_at` fields. It is not a `FileMethod` implementation.

Planned lifecycle methods include:

| ID | Stage | Purpose |
| --- | --- | --- |
| `plan.customer-context.v1` | `start` | Captures plan/customer goal and prepares run context. |
| `plan.final-summary.v1` | `end` | Summarizes run results and recommends next commands. |

The discovery command lists the exact methods available in the current build:

```bash
aascribe index methods
aascribe index methods --stage start
aascribe index methods --stage file
aascribe index methods --stage folder
aascribe index methods --stage end
```

Listing methods must not execute external programs.

## Method Output

Method output is arbitrary structured data wrapped by `aascribe` with stable metadata.

Per-file example:

```json
{
  "path": "internal/index/index.go",
  "methods": {
    "builtin.classify.v1": {
      "generated_at": "2026-04-28T03:21:15Z",
      "input_hash": "sha256:...",
      "status": "ok",
      "fields": {
        "category": "source",
        "tags": ["index", "metadata"],
        "confidence": 0.91
      }
    }
  }
}
```

Run-level lifecycle example:

```json
{
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
  }
}
```

## Method Interfaces

Methods should not share one large interface with no-op `Start`, `BuildFile`, `BuildFolder`, and `End` methods.

Use a small method core plus role interfaces:

- `Method`: `ID()` and `Fingerprint()`
- `FileMethod`: per-file input and output
- `FolderMethod`: folder aggregate input and output
- `LifecycleMethod`: `start` or `end` run-level input and output

Each method should expose a descriptor used by both the engine and `aascribe --format json index methods`. The descriptor should include stage, scope, required/always-enabled/default-enabled flags, summary requirements, no-summary support, a short description, purpose, and config schema metadata.

`index methods` output should always include descriptions, not only method ids. The text form should be readable enough for a human or AI to choose a method quickly; the JSON form should expose the full descriptor.

Example text:

```text
available index methods:
- builtin.classify.v1
  stage: file
  default_enabled: false
  requires_summary: false
  description: Classifies each text file for agent routing using path, file type, and optional summary signals.

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

`Fingerprint()` is the method-owned digest of anything that can affect output, such as method version, taxonomy config, prompt version, plan section, external command line, or declared external artifacts.

The field-map payload persisted under `fields` can start as `map[string]any`. Built-in report aggregation should read only documented soft-schema fields unless a later method declares a richer aggregation contract.

File method input should include stable file facts such as normalized path, file name, file type, content hash, summary when available, no-summary state, and any method-selected text content.

Folder method input should include normalized folder path plus stable direct child records, including selected method envelopes. Folder methods may re-run in v1 if folder reuse is not implemented.

## Input Hashing And Reuse

Every method result should have an `input_hash`.

Use one rule for all stages:

```text
input_hash = sha256(canonical(stage-input) || ID() || Fingerprint())
```

`canonical(...)` means stable JSON serialization with lexically sorted object keys, no insignificant whitespace, and fixed timestamp/number formatting. Volatile fields such as `generated_at`, run timing, and transient stats must be excluded from aggregate hashes.

Unchanged file metadata can be reused only when the relevant method `input_hash` still matches.

## Method Status

Method output status should use a small enum:

- `ok`: method ran and returned fields
- `failed`: method ran but returned an error
- `skipped`: method was selected but intentionally did not run

Omitting a method key means the method was not selected. Use `skipped` when the method was selected but blocked by run options or file eligibility.

## File Eligibility

The first method implementation should run file methods only for text/status-eligible files.

Binary files keep the current behavior: they are recorded as not indexed. If a selected method cannot run on a binary file, its method envelope should be `skipped`.

Future methods that consume binary input must explicitly declare that support in `aascribe --format json index methods`.

## Failure Rules

Method failures should be local and explainable.

- File method failures are recorded on that file and should not fail the whole run.
- Folder method failures are recorded on that folder and should not fail unrelated folders.
- Optional start method failures become warnings and skipped context.
- Required start method failures may abort the run before traversal.
- End method failures are recorded in run-level metadata and the completion report.

Run-level lifecycle output should live only in the root `.aascribe_index_meta.json`.

## Built-In Aggregation

The first folder method should be `builtin.routing.v1`.

It should roll up stable child method output, especially:

- `category`
- `tags`
- `confidence`
- missing category state

It should not hash volatile child fields such as `generated_at`. Use child method `input_hash` and stable `fields` when building aggregate input.

## Index Plans

For non-trivial indexing behavior, an AI agent should prefer an index plan over many CLI flags.

Planned command:

```bash
aascribe index . --index-plan ./aascribe.index.toml
```

Example plan:

```toml
version = "index-plan-v1"

[run]
name = "agent-repo-understanding"
description = "Index plan for future agents to navigate and answer questions about this repo."

[methods]
enabled = [
  "plan.customer-context.v1",
  "summary",
  "builtin.classify.v1",
  "builtin.routing.v1",
  "plan.final-summary.v1"
]

[methods."plan.customer-context.v1"]
goal = "future agents can navigate and answer questions about this repo"

[methods."summary"]
style = "medium"

[methods."builtin.classify.v1"]
low_confidence_threshold = 0.6

[methods."builtin.routing.v1"]
include_tags = true

[methods."plan.final-summary.v1"]
include_recommended_commands = true
```

Built-in method stage and scope are registry facts. Plan files should not repeat them for built-in methods. Custom method descriptors still need explicit `stage` and `scope` because they define a new method.

## AI Caller Workflow

An AI agent should use this flow:

```text
list available methods -> read plan guidance -> inspect repo/customer goal -> draft index-plan-v1 TOML -> validate plan -> run index with --index-plan
```

Planned helper commands:

```bash
aascribe --format json index methods
aascribe index plan validate ./aascribe.index.toml
```

Defer `index plan template`, `index plan explain`, and `index plan schema` until the plan format stabilizes.

The AI should generate the plan file only. It should not run custom external programs while drafting a plan. External execution happens only during a validated `aascribe index --index-plan <file>` run.

## External Methods

Future custom methods may execute external programs, but only through explicit plan definitions.

Example:

```toml
[[methods.custom]]
id = "custom.security.v1"
stage = "file"
scope = "file"
requires_summary = true
command = ["./methods/security-check", "--format", "json"]
timeout = "5s"
```

External method requirements:

- opt-in only
- bounded timeout
- bounded stdout and stderr
- stable JSON input and output
- no source file mutation
- no inherited secrets unless explicitly allowed
- clear per-file, per-folder, or lifecycle failure recording
- method discovery must not execute the external program

Custom/external methods should be declared through `[[methods.custom]]` in an index plan. Avoid a separate `--metadata-method custom:...` code path; if a shortcut is ever added, it should synthesize a one-method plan internally.

## Current Versus Planned Behavior

Current behavior:

```bash
aascribe index .                         # base + summary
aascribe index . --no-summary            # base only
aascribe index . --no-metadata-methods   # base only
aascribe index . --metadata-method summary
aascribe index . --metadata-method base  # accepted no-op; base always runs
aascribe index . --metadata-method builtin.classify.v1
aascribe index . --metadata-method builtin.classify.v1 --metadata-method builtin.routing.v1
aascribe index methods
```

Currently accepted method ids:

- `base`
- `summary`
- `builtin.classify.v1`
- `builtin.routing.v1`

Planned behavior adds:

- `--index-plan <file>`
- `aascribe index plan validate`
- start and end lifecycle method stages
- explicit custom/external method definitions
