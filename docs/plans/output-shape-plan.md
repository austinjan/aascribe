# Output Shape Plan For `aascribe`

## Summary

Implement `OutputShape` as an independent reusable model layer for CLI applications so nearly every user-facing command can route its payload through the same shape contract before producing final output.

This is foundational work. `aascribe` already has:

- a shared JSON envelope
- checked-in JSON Schema files under `docs/shapes/`
- documented `OutputShape` names in `docs/USAGE.md`

What is still missing is the implementation seam that connects those pieces in code. Today, command handlers return arbitrary `data` payloads through `output.CommandResult`, but there is no independent runtime model for `OutputShape`, no reusable shape registry, and no shared path that turns raw command data into a shape-backed result.

The goal of this plan is to make `OutputShape` real in the implementation, not only in docs, and to shape it as a reusable CLI-oriented model rather than an `aascribe`-only internal helper.

Preferred architecture:

1. command logic produces raw output data
2. raw output data is attached to an `OutputShape` model
3. the shape-aware result is rendered as the real CLI output

In short:

- `output -> outputShape -> real output`

## Why This Matters

`OutputShape` is not a niche feature for one command. It is a core contract that almost every current and future command depends on:

- `init` returns `StoreInitResult`
- `logs` commands return log operation shapes
- `describe` should return `FileDescription`
- `index` should return `PathIndexTree`
- memory commands already rely on stable list/detail/result payloads

If this stays documentation-only, payload drift becomes likely as commands are implemented. If we build it as an independent model now, later features in this repo and other CLI projects can reuse one stable shape system instead of each command inventing its own output rules.

## Current State

The repo already has the specification pieces:

- [docs/reference/aascribe_ai_output_shapes.md](../reference/aascribe_ai_output_shapes.md)
- [docs/shapes/README.md](../shapes/README.md)
- [docs/USAGE.md](../USAGE.md)

The current implementation seam is:

- command handlers return `*output.CommandResult`
- `output.CommandResult` currently contains only:
  - `Data any`
  - `Text string`
- the JSON envelope writer serializes `data` and `meta`

Current gap:

- no independent runtime `OutputShape` model
- no reusable shape registry or resolver
- no central mapping from command -> output shape
- no validation that documented shapes and runtime payloads stay aligned

## Design Goals

- Make every concrete user-facing command declare one stable `OutputShape`
- Make `OutputShape` reusable as an independent model, not just a string field on command results
- Keep `OutputShape` scoped to the `data` field, not the top-level envelope
- Preserve the current JSON/text envelope model
- Reuse shared shapes across commands where the logical payload is the same
- Make drift detectable in tests before users or skills discover it
- Avoid forcing a one-shot rewrite of every command before progress can continue

## Non-Goals

- Changing the top-level `ok` / `data` / `error` / `meta` envelope
- Replacing text output mode with schema-driven rendering
- Adding dynamic runtime schema loading from disk in the first pass
- Generating JSON Schema automatically from Go types in the first pass
- Enforcing every future command on day one before incremental rollout is possible

## Implementation Changes

### 1. Add an independent `OutputShape` model layer

Create a dedicated reusable model for output shapes rather than treating them as loose string constants on command handlers.

Recommended location:

- `pkg/outputshape/`
- or a standalone module if we later want to share it across repositories

Recommended responsibilities:

- define `OutputShape` identifiers
- define shape specs and schema references
- provide command-to-shape resolution
- build a shape-aware output object from raw command data
- expose helpers that tests and renderers can reuse

Recommended core types:

```go
type Name string

type Spec struct {
    Name            Name
    SchemaRef       string
    Description     string
}

type Result struct {
    Shape Spec
    Data  any
    Text  string
}
```

This makes `OutputShape` its own model, not a passive annotation.

Design rule:

- do not place this package under `internal/`
- treat it as a reusable CLI library boundary from the start

### 2. Define the output flow explicitly

The implementation should follow this flow:

```go
type RawOutput struct {
    Data any
    Text string
}
```

Then:

1. command handler computes raw output
2. command layer or shape layer resolves the expected `OutputShape`
3. shape layer returns a shape-aware result
4. output renderer writes the final JSON/text envelope

This is the key architectural shift:

- commands should not own shape logic directly
- renderers should not have to guess shape identity
- shape resolution should live in one reusable layer
- the shape layer should be usable by other CLI projects, not only `aascribe`

### 3. Keep `output.CommandResult` thin or replace it with shape-aware output

There are two reasonable ways to integrate the new model:

Option A:

- keep `output.CommandResult`
- make it wrap an `outputshape.Result`

Option B:

- replace `output.CommandResult` with `outputshape.Result` in command execution paths

Recommended first choice:

- keep the current output package stable
- introduce `outputshape.Result`
- make `output.WriteSuccess` consume the shape-aware result

This reduces churn while still establishing the independent model.

Example direction:

```go
type CommandResult struct {
    Shape outputshape.Spec
    Data  any
    Text  string
}
```

The important point is not the exact field layout. The important point is that shape identity comes from the independent `outputshape` model, not from ad hoc string literals spread around command handlers.

This package should be designed the way we would design a general-purpose CLI library:

- small public surface
- no `aascribe`-specific business logic inside the model
- command-specific bindings supplied by the application layer

### 4. Add a central command-to-shape registry

Create one shared mapping layer for command output contracts that uses the independent shape model.

Recommended form:

```go
type CommandBinding struct {
    Path  string
    Shape Name
}
```

Responsibilities:

- define the expected `OutputShape` for each concrete command/subcommand
- avoid scattering string literals across handlers
- provide a single reusable lookup API
- make command contract coverage easy to test

Boundary rule:

- the generic `outputshape` package defines shape concepts and helpers
- `aascribe` provides its own command bindings on top of that package
- do not bake `aascribe` command names into the generic library if we want cross-CLI reuse

Examples:

- `init` -> `StoreInitResult`
- `logs path` -> `LogPathResult`
- `logs export` -> `LogExportResult`
- `logs clear` -> `LogClearResult`
- `describe` -> `FileDescription`
- `index` -> `PathIndexTree`

### 5. Make command handlers return raw output, then bind shape

Update implemented command handlers so they focus on building the actual payload, while shape binding happens through the shared `outputshape` model.

Recommended direction:

- handler returns raw `data` + `text`
- command execution path asks `outputshape` for the expected shape
- shape layer constructs the final shape-aware result

This keeps handlers simpler and prevents every command from duplicating shape boilerplate.

Initial adoption target:

- `runInit`
- `runLogsPath`
- `runLogsExport`
- `runLogsClear`
- `runSummarize` if it remains a supported command

Future commands such as `describe`, `index`, `remember`, `recall`, `list`, and `show` should follow the same convention as they are implemented.

### 6. Decide how `summarize` fits the shape model

The repo currently has a debug-oriented `summarize` command, but the output-shape reference doc maps the product-facing single-file summary contract to `describe` via `FileDescription`.

This plan should force an explicit decision:

- Option A: keep `summarize` as a debug command with its own shape
- Option B: make `summarize` internal-only and stop treating it as a stable public contract
- Option C: make `summarize` return the same logical shape as `describe`

Recommended direction:

- `describe` should be the stable public single-file contract
- `summarize` should either:
  - be treated as a debug-only command outside the stable shape catalog, or
  - be aligned to the same shape if we intend to keep it public

Do not leave this ambiguous during implementation.

### 7. Add contract coverage tests

Add tests that verify:

- every implemented user-facing command resolves to a non-empty `OutputShape`
- `OutputShape` names match the documented contract
- command bindings do not drift from the code path

Recommended first tests:

- unit tests for shape binding that assert resolved shape identity
- one table-driven test for command-to-shape bindings
- one doc-alignment test covering the currently implemented commands

### 8. Add schema-alignment checks

In the first pass, do lightweight verification rather than full runtime JSON Schema validation.

Recommended staged approach:

Stage 1:

- ensure each declared `OutputShape` in the registry has a corresponding schema file under `docs/shapes/`

Stage 2:

- add fixture-based tests that marshal representative `data` payloads and validate key required fields

Stage 3:

- optionally add full JSON Schema validation in tests using the checked-in schema files

This keeps the first implementation practical while still moving toward stronger guarantees.

### 9. Keep documentation and implementation aligned

For every command that becomes runtime shape-aware:

- `docs/USAGE.md` must name the same `OutputShape`
- `docs/shapes/` must contain the schema for that shape
- the shape registry must resolve that same shape name
- the runtime output path must carry that resolved shape through to rendering/tests

This gives the repo one three-part contract:

1. docs say what shape a command returns
2. schemas define that shape
3. the runtime shape model resolves and carries that shape

## Recommended Delivery Phases

### Phase 1: Independent shape model

- add `pkg/outputshape`
- define shape names/specs
- add command-to-shape bindings
- add unit tests for shape resolution

### Phase 2: Integrate the output flow

- adapt command execution to use:
  - raw output
  - shape binding
  - final output rendering
- update implemented commands to use the new path
- keep the external envelope unchanged

### Phase 3: Schema presence checks

- add tests that ensure bound shapes exist in `docs/shapes/`
- reconcile any missing or ambiguous command mappings
- make an explicit `summarize` decision

### Phase 4: Stronger payload validation

- add representative payload conformance tests
- optionally validate marshaled JSON payloads against checked-in JSON Schemas

### Phase 5: Feature rollout

- require new command implementations such as `describe` and `index` to ship with:
  - a registered `OutputShape`
  - a schema
  - tests proving contract alignment

## Public Contract Decisions

These decisions should be settled during implementation:

### Decision 1: Is `OutputShape` part of the runtime JSON envelope?

Options:

- internal-only metadata for now
- exposed in `meta`
- exposed as a top-level success field

Recommended first choice:

- keep it internal to the shape model and code/tests in the first pass
- avoid changing the public envelope until there is a clear consumer need

### Decision 2: Are debug commands part of the stable shape catalog?

Recommended first choice:

- no, unless they are documented as stable user-facing commands in `USAGE.md`

### Decision 3: What counts as an implemented command for enforcement?

Recommended first choice:

- only commands that currently return success payloads
- stubbed `NOT_IMPLEMENTED` commands can be brought under enforcement as they land

## Test Plan

- command result tests:
  - `init` resolves `StoreInitResult`
  - `logs path` resolves `LogPathResult`
  - `logs export` resolves `LogExportResult`
  - `logs clear` resolves `LogClearResult`
- binding coverage tests:
  - every implemented command has a declared shape
  - declared shapes are non-empty
- schema presence tests:
  - every declared shape resolves to a file in `docs/shapes/`
- doc alignment tests:
  - implemented command mappings match `docs/USAGE.md` for the currently supported commands

Later optional tests:

- validate marshaled payloads against JSON Schema
- reject commands whose payloads remove required fields without review

## Follow-On Work Enabled By This Plan

- implement `describe` with a real `FileDescription` contract
- implement `index` with a real `PathIndexTree` contract
- keep memory command implementations aligned with their documented shapes
- give skills and other AI clients a reliable contract for parsing `data`
- reduce accidental payload drift during iterative feature work

## Assumptions

- `docs/shapes/` remains the checked-in source of truth for output shapes
- `docs/USAGE.md` remains the command contract source of truth
- the first implementation should be incremental and low-risk
- an independent runtime shape model is more reusable than embedding output-shape strings directly in handlers
- this model should be designed as a reusable package, not an `internal/` implementation detail
- internal runtime shape plumbing is valuable even if the public envelope does not expose `OutputShape` immediately
