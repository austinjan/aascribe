# LLM Output Tasks

Status: Planned

## Summary

Implement an LLM-oriented output transport layer for `aascribe`.

This task document is more important than `OutputShape` for the current phase.

The key problem is not "what shape is this data?" The key problem is:

- command output can become too large for an LLM context window
- large output must remain inspectable in bounded chunks
- the system must preserve enough metadata and hints so an LLM can continue reading the rest safely
- retained output files must be bounded so disk usage does not grow without limit

The design target is:

1. command produces output text
2. output layer decides whether to inline or spill to a persisted output file
3. output layer returns an LLM-friendly response with explicit continuation hints
4. the LLM can use dedicated commands to inspect more of the stored output

In short:

- `raw output -> output transport -> LLM-readable chunks`

## Task Breakdown

This document acts as the execution tracker for the LLM-oriented output system.

### Phase 0: Interface And Contract

#### Task 0.1: Define the LLM output contract

- decide the minimum inline output limit behavior
- define when output stays inline vs spills to file
- define the partial-output metadata contract
- define the browsing command surface for stored output

Status: Planned

#### Task 0.2: Define the stored-output navigation interface

- settle the first command group:
  - `output list`
  - `output meta`
  - `output show`
  - `output head`
  - `output tail`
  - `output slice`
- decide the unit used by `slice`
  - bytes
  - runes
  - or lines
- document deterministic behavior for out-of-range requests

Status: Planned

#### Task 0.3: Define the retention contract

- keep only the newest 50 stored outputs
- define eviction behavior
- define required manifest metadata

Status: Planned

### Phase 1: Core Transport Package

#### Task 1.1: Add `pkg/llmoutput`

- create a reusable package for LLM-oriented output transport
- keep it independent from `aascribe` business logic
- define core transport-facing types

Status: Planned

#### Task 1.2: Add inline-vs-stored delivery logic

- accept raw output text
- compare output size against the inline threshold
- return either:
  - full inline output
  - or partial inline output plus stored-output metadata

Status: Planned

#### Task 1.3: Add managed output persistence

- persist oversized outputs to a managed output directory
- assign stable output IDs
- write metadata needed for later browsing

Status: Planned

### Phase 2: Stored Output Browsing

#### Task 2.1: Add `output meta`

- return metadata for one stored output
- include total size, created time, and navigation hints

Status: Planned

#### Task 2.2: Add `output show`

- return metadata plus the default first chunk
- make the result useful as the first recovery step after truncation

Status: Planned

#### Task 2.3: Add `output head` and `output tail`

- support bounded line-based browsing
- return clear range metadata with every response

Status: Planned

#### Task 2.4: Add `output slice`

- support deterministic offset-based reads
- define offset unit explicitly
- reject or clamp invalid ranges consistently

Status: Planned

#### Task 2.5: Add `output list`

- list recent retained outputs
- include IDs and enough metadata for follow-up reads

Status: Planned

### Phase 3: Partial Output Hints

#### Task 3.1: Add partial-output metadata to oversized command responses

- include `output_id`
- include `truncated`
- include current inline range
- include total size
- include next-step command hints

Status: Planned

#### Task 3.2: Make partial-output hints explicit for LLMs

- clearly indicate that current output is incomplete
- clearly indicate where the full output lives
- clearly indicate how to query the next portion

Status: Planned

### Phase 4: Retention And Storage Safety

#### Task 4.1: Implement 50-item retention

- keep only the newest 50 stored outputs
- evict the oldest retained output after overflow
- keep manifest state in sync with file deletion

Status: Planned

#### Task 4.2: Handle corrupted or missing stored outputs

- return clear errors for missing output IDs
- return clear errors for missing files
- handle corrupted manifest state safely

Status: Planned

### Phase 5: Command Integration

#### Task 5.1: Route command success output through the LLM output transport

- integrate the transport layer into the current output path
- keep command handlers focused on raw output production

Status: Planned

#### Task 5.2: Choose the first commands to adopt the transport path

Recommended first candidates:

- `list`
- `recall`
- `index`
- any command likely to emit large result sets

Status: Planned

### Phase 6: Verification

#### Task 6.1: Transport tests

- small output stays inline
- large output spills to file
- partial hint fields are present

Status: Planned

#### Task 6.2: Browsing command tests

- `output meta` works
- `output show` works
- `output head` works
- `output tail` works
- `output slice` works

Status: Planned

#### Task 6.3: Retention tests

- oldest outputs are evicted after the 50-item limit
- manifest and files stay consistent across repeated writes

Status: Planned

## Why This Matters

For an LLM-oriented CLI, output delivery is a core runtime concern.

Without this layer:

- large responses can overflow context
- the model may only see a truncated middle or prefix without knowing it
- continuation becomes unreliable
- repeated large outputs can consume unbounded disk space

This system should be reusable across commands. It should not be designed only for one feature like `index` or `describe`.

## Scope

This plan covers:

- output size limiting
- spill-to-file behavior for oversized output
- persisted output browsing commands for LLMs
- partial-output hints
- retention using a fixed-size ring policy

This plan does not require:

- human-readable output ergonomics
- interactive paging like `less`
- terminal UI features
- shape/schema enforcement

## Core Principles

- output is designed for LLM consumption first
- every partial output must clearly say that it is partial
- every stored output must be addressable by an ID
- the system must support deterministic chunk browsing
- disk growth must be bounded automatically
- command handlers should not own truncation or file-retention logic

## Functional Requirements

### 1. Bounded inline output

The output layer must enforce a maximum inline output size.

If a command result exceeds that threshold:

- do not print the full output inline
- persist the full output to a managed output file
- return a partial inline result plus machine-readable continuation metadata

The threshold should be configurable, but the first implementation can ship with a fixed default.

### 2. Spill oversized output to managed files

When output is too large:

- write the full output to an output store directory
- assign a stable output ID
- preserve enough metadata for later browsing

The persisted file becomes the source of truth for further reads.

### 3. LLM browsing commands

The system must provide commands that let an LLM inspect the stored output in bounded slices.

The examples given by the user are directionally correct:

- `head 100`
- `tail 100`
- `offset 500 size 200`

Recommended command surface:

```bash
aascribe output list
aascribe output show <output-id>
aascribe output head <output-id> [--lines <n>]
aascribe output tail <output-id> [--lines <n>]
aascribe output slice <output-id> --offset <n> --limit <n>
aascribe output meta <output-id>
```

Recommended semantics:

- `output list`
  - show recent stored outputs with IDs and summary metadata
- `output show`
  - show output metadata plus the default first chunk
- `output head`
  - return the first `n` lines
- `output tail`
  - return the last `n` lines
- `output slice`
  - return a deterministic byte- or character-range slice
- `output meta`
  - return metadata only, with size, truncation status, and available navigation info

First implementation recommendation:

- support line-based reads for `head` and `tail`
- support byte-offset or rune-offset reads for `slice`
- document clearly which offset unit is used

### 4. Explicit partial-output hints

Whenever inline output is partial, the system must tell the LLM:

- this is partial output
- what portion is currently shown
- where the full output is stored
- how to request the next portion

Minimum required hint fields:

- `output_id`
- `stored: true`
- `truncated: true`
- `inline_range`
- `total_size`
- `next_suggestion`

Recommended additional fields:

- `store_path`
- `available_commands`
- `default_chunk_size`
- `remaining_estimate`

### 5. Fixed retention with ring-buffer behavior

Persisted outputs must not accumulate without bound.

Retention rule:

- keep only the most recent 50 stored output files
- when writing a new file beyond the limit, evict the oldest retained output

This is effectively a fixed-capacity rolling window. The implementation may use:

- explicit oldest-first deletion by timestamp/sequence
- or a true circular slot scheme

For the first implementation, the important behavior is:

- retained outputs are bounded at 50
- eviction is automatic
- the newest output is never discarded before older ones

## Proposed Architecture

The output system should be separate from command business logic.

Recommended layers:

### 1. Raw output layer

Command handlers return raw output content.

Example direction:

```go
type RawOutput struct {
    Text string
}
```

This layer should not know:

- size limits
- file persistence
- chunk navigation
- retention policy

### 2. Output transport layer

This is the new core layer.

Recommended responsibilities:

- decide inline vs stored delivery
- persist oversized output
- assign output IDs
- generate partial-output metadata
- serve navigation commands like `head`, `tail`, and `slice`
- apply retention policy

Recommended location:

- `pkg/llmoutput/`

This package should be reusable by other CLIs.

### 3. Final envelope/render layer

This layer formats the transport result into the CLI response contract.

It may remain in the current `output` package, but it should consume transport results rather than implementing storage logic itself.

## Recommended Core Types

```go
type RawOutput struct {
    Text string
}

type StoredOutputRef struct {
    ID         string
    Path       string
    TotalBytes int64
    CreatedAt  time.Time
}

type InlineChunk struct {
    Text       string
    Offset     int64
    Limit      int64
    TotalBytes int64
}

type ContinuationHint struct {
    Stored            bool
    Truncated         bool
    OutputID          string
    InlineRangeStart  int64
    InlineRangeEnd    int64
    TotalBytes        int64
    NextSuggestion    string
    AvailableCommands []string
}

type DeliveredOutput struct {
    InlineText string
    StoredRef  *StoredOutputRef
    Hint       ContinuationHint
}
```

Exact type names can change, but the responsibilities should remain.

## Output Store Layout

Recommended first layout under the `aascribe` store:

```text
<store>/outputs/
  manifest.json
  out_000001.txt
  out_000002.txt
  ...
```

Recommended metadata to retain per output:

- output ID
- file path
- created time
- total bytes
- line count if cheap to compute
- producing command
- whether the inline response was partial

The manifest should be simple and append/update friendly.

## Inline Response Contract

When output fits inline:

- return the full inline text
- `truncated` is `false`
- no stored file is required

When output is too large:

- return only the initial chunk inline
- persist the full text
- include continuation metadata

Example direction:

```json
{
  "ok": true,
  "data": {
    "text": "first chunk here...",
    "transport": {
      "stored": true,
      "truncated": true,
      "output_id": "out_000051",
      "inline_range": {
        "start": 0,
        "end": 4000
      },
      "total_bytes": 18244,
      "next_suggestion": "aascribe output slice out_000051 --offset 4000 --limit 4000",
      "available_commands": [
        "aascribe output show out_000051",
        "aascribe output head out_000051 --lines 100",
        "aascribe output tail out_000051 --lines 100",
        "aascribe output slice out_000051 --offset 4000 --limit 4000",
        "aascribe output meta out_000051"
      ]
    }
  }
}
```

The exact envelope shape can be refined later. The important requirement is that partial delivery must be explicit and queryable.

## Command Design Notes

Recommended first output-navigation command group:

```bash
aascribe output list
aascribe output meta <output-id>
aascribe output show <output-id>
aascribe output head <output-id> [--lines 100]
aascribe output tail <output-id> [--lines 100]
aascribe output slice <output-id> --offset <n> --limit <n>
```

Design notes:

- keep commands deterministic
- avoid interactive stateful sessions
- return machine-friendly text/JSON only
- include range metadata on every partial read

## Retention Policy

The system must automatically enforce a fixed retained-output limit.

Required first rule:

- keep only the newest 50 output files

Recommended implementation behavior:

- after persisting a new output, load the manifest
- if retained count exceeds 50:
  - delete the oldest files
  - remove their manifest entries

This is sufficient for the first version even if the internal implementation is not literally a circular array.

## Error Handling

The system should fail clearly when:

- an output ID does not exist
- a stored file is missing
- a requested slice is outside the valid range
- the output manifest is corrupted
- the output directory cannot be written

Recommended behavior:

- navigation commands return structured errors with valid usage hints
- slice reads clamp or fail explicitly; do not silently invent ranges
- if persistence fails, the command may fall back to inline truncation only if that fallback is explicitly safe and documented

## Test Plan

- small output stays inline
- oversized output spills to a managed file
- partial response includes output ID and continuation hints
- `output head` returns the expected prefix
- `output tail` returns the expected suffix
- `output slice` returns the expected deterministic range
- missing output ID returns a clear not-found error
- retention deletes outputs older than the newest 50
- manifest remains consistent after repeated writes and evictions

## Follow-On Work Enabled By This Plan

- safe large-output handling for `index`
- safe large-output handling for memory listing and recall
- a reusable output transport package for other CLI tools
- later integration with `OutputShape` if we want typed payloads on top of this transport layer

## Assumptions

- LLM-oriented output delivery is higher priority than shape modeling right now
- output is optimized for machine consumption, not human readability
- persisted outputs live under the active `aascribe` store unless we later define a shared global store
- fixed retention of 50 outputs is acceptable for the first implementation
