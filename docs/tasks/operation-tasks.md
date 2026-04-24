# Long-Running Operation Tasks

Task breakdown for adding a unified long-running operation mechanism to `aascribe`.

This document is execution-oriented. It defines one shared contract for commands that may take longer than a normal synchronous CLI call, using the current project constraints:

- Go is the implementation language
- the CLI/output contract in [../USAGE.md](../USAGE.md) must remain the source of truth
- command-specific payload shapes in [../shapes](../shapes/README.md) must remain separate from operation lifecycle shapes
- the existing large-output transport in [../archieved/llm-output-tasks.md](../archieved/llm-output-tasks.md) should integrate with this work instead of being replaced by it

## Summary

`aascribe` now has at least one command, `index`, that can take long enough to need a better lifecycle than "run once and block until stdout is ready".

The goal is not to special-case `index`.

The goal is to add one shared mechanism for long-running commands so agents and tools can interact with them consistently.

The design target is:

1. a long-running command can be started explicitly
2. the command returns an operation handle quickly
3. callers can inspect status, progress, and logs
4. callers can fetch the final result later
5. callers can cancel a running operation

In short:

- `command execution -> operation lifecycle -> result or output reference`

This is complementary to the output transport system:

- `operation` manages lifecycle, progress, and cancellation
- `output` manages oversized result payload delivery and browsing

## Current Status

Done in the current repo:

- synchronous command execution already has one stable top-level JSON envelope
- managed oversized output transport already exists through the `output` command family
- `index` already has engine-level context propagation for cancellation-sensitive work
- `index` already has enough internal stage boundaries to become the first real operation candidate
- checked-in lifecycle shapes now exist in [../shapes](../shapes/README.md) for accepted, status, events, result, and list payloads
- the first operation inspection command surface now exists: `list`, `status`, `events`, `result`, and `cancel`

Still intentionally incomplete:

- `index --async` exists, but binary smoke testing is still pending
- final operation results are stored directly in `result.json`; oversized result integration with managed output transport is not done yet
- operation retention and cleanup policy are not done yet
- async support is opt-in per command; `index` is the first supported command

## Scope And Sequencing

Recommended delivery order:

1. define the operation lifecycle contract and checked-in shapes
2. define operation persistence under the active store
3. add read-only inspection commands first: `status`, `events`, `result`, `list`
4. add cancellation
5. add a shared operation reporter for engines
6. migrate `index` in long-running mode
7. connect final operation results to managed output transport
8. add retention and cleanup policy

The first useful version should already preserve these system properties:

- lifecycle state is separate from command result payloads
- JSON envelopes remain valid and unpolluted by progress output
- operations survive process exit because they are persisted
- long-running work can be inspected and canceled
- managed output remains the single mechanism for oversized final payloads

## Problem Statement

Current synchronous command behavior is acceptable for small work, but it becomes awkward when:

- command execution may take tens of seconds or minutes
- an agent needs to know whether the command is still running or is stuck
- an agent wants progress updates without corrupting the final JSON envelope
- the final result is large enough to spill into managed output storage
- the command should be cancellable

Without a shared mechanism, each long-running command will invent its own ad hoc progress format, logging strategy, and recovery behavior.

That would be a step backward for `aascribe`, which already prefers explicit checked-in contracts such as:

- `OutputShape`
- managed `output` browsing
- structured parse errors
- explicit command help

## Goals

- define one shared lifecycle contract for long-running commands
- keep lifecycle state separate from command-specific result payloads
- make the mechanism easy for LLM agents to drive
- support both text and JSON caller flows
- support cancellation
- integrate cleanly with existing managed-output behavior

## Non-Goals For The First Operation Milestone

- distributed job execution
- remote workers
- websocket streaming
- background daemons outside the current CLI/store model
- full scheduler/orchestration for every command type
- replacing normal synchronous command mode for small/fast commands

## Core Design Direction

Long-running command handling should become a first-class shared subsystem.

Recommended model:

- normal synchronous command mode remains available
- long-running mode uses an explicit `operation` lifecycle
- operations are persisted under the active store
- the final operation result may inline its payload or point to an `output_id`

Recommended first command family to use this mechanism:

- `index`

Likely later candidates:

- `consolidate`
- large-scope `recall`
- future repo-wide analysis commands

## Responsibility Split

The shared operation system is responsible for:

- starting long-running work
- assigning stable operation IDs
- storing lifecycle state
- storing progress events
- storing completion metadata
- exposing status/result/cancel surfaces

The shared operation system is not responsible for:

- defining the command-specific result schema
- replacing managed large-output storage
- redefining command-specific engine behavior

Command-specific engines are responsible for:

- doing the real work
- reporting progress/events through a shared operation reporter
- returning either a normal command result or a reference to a stored output/result

The output transport system remains responsible for:

- deciding whether final result text is inline or spilled
- storing oversized results under managed output storage
- exposing `output list/meta/show/head/tail/slice`

## Proposed User-Facing Command Surface

Recommended first command group:

- `operation start ...`
- `operation status <operation-id>`
- `operation events <operation-id>`
- `operation result <operation-id>`
- `operation cancel <operation-id>`
- `operation list`

Open naming question:

- if `start` is too generic, we could also model long-run mode as a flag on supported commands, then use `operation ...` only for follow-up inspection

Recommended first-step contract:

- supported commands can expose a `--async` or `--operation` mode later
- first implementation may start with an explicit `operation` layer and one adapted command such as `index`

Concrete first-step recommendation:

- keep existing synchronous commands unchanged
- add an explicit `operation` command group
- add one explicit async entrypoint for the first migrated command, likely `index --async`
- avoid introducing hidden auto-background behavior in the first milestone

## Proposed Shapes

Recommended new shapes:

- `OperationAccepted`
- `OperationCancelResult`
- `OperationStatus`
- `OperationEventList`
- `OperationResult`
- `OperationList`

### `OperationAccepted`

Purpose:

- returned immediately after starting long-running work

Suggested fields:

- `operation_id`
- `command`
- `state`
- `started_at`
- `status_hint`
- `result_hint`
- `cancel_hint`

### `OperationStatus`

Purpose:

- current lifecycle + progress snapshot

Suggested fields:

- `operation_id`
- `command`
- `state`
- `stage`
- `message`
- `started_at`
- `updated_at`
- `completed_at`
- `progress`
- `result_ready`
- `error`

Suggested `progress` fields:

- `current`
- `total`
- `unit`
- `percent`

### `OperationEventList`

Purpose:

- append-only event/history view for a running or completed operation

Suggested fields:

- `operation_id`
- `events`

Suggested per-event fields:

- `time`
- `level`
- `stage`
- `message`
- `data`

### `OperationResult`

Purpose:

- final result accessor after the operation is complete

Suggested fields:

- `operation_id`
- `state`
- `completed_at`
- `data`
- `output_id`
- `truncated`
- `error`

Notes:

- `data` is the command-specific result payload when small enough
- `output_id` is used when the final result was routed through managed output storage
- result lifecycle metadata must stay separate from the command-specific `OutputShape`

Recommended later shape:

- `OperationList`

## State Model

Recommended minimal states:

- `pending`
- `running`
- `succeeded`
- `failed`
- `canceled`

Optional later states:

- `cancel_requested`
- `partial_success`

For the first milestone, keep the state model small unless a command truly needs a separate partial-success state.

## Persistence Model

Operations should be persisted under the active store.

Suggested store layout:

- `<store>/operations/`
- one directory per operation id

Suggested artifacts per operation:

- `operation.json`
- `events.jsonl`
- optional `result.json`

Suggested persisted fields:

- operation metadata
- current lifecycle state
- timestamps
- command identity and arguments
- compact progress snapshot
- final result reference

Important separation:

- operation persistence is lifecycle state
- command engine metadata such as `.aascribe_index_meta.json` remains command-specific engine state
- command result payload remains separate from both

Recommended file-level persistence rules:

- `operation.json` should be the latest compact snapshot
- `events.jsonl` should be append-only
- final result reference should be stored in `operation.json`; `result.json` is optional for small direct payloads
- writes should be atomic where snapshot replacement is used

## CLI / UX Principles

- synchronous commands should remain simple by default when work is small
- long-running mode must never corrupt the final JSON envelope by mixing progress lines into stdout
- progress/events should not be emitted as arbitrary ad hoc stderr text only
- text mode should remain LLM-readable and compact
- JSON mode should remain machine-friendly and explicit

Recommended text behavior:

- `operation start` returns a short accepted summary with next-step hints
- `operation status` returns concise current stage/progress text
- `operation result` returns either the final result or explicit output-followup guidance

## Integration With Existing Output Transport

This work should integrate with the existing output mechanism rather than compete with it.

Recommended interaction:

- a long-running operation completes
- the command-specific final result is rendered
- if the result is too large, existing output transport stores it
- `OperationResult` references the `output_id`

This gives a clean separation:

- operation lifecycle
- result transport

## Phase 0: Contract And Scope

### Task 0.1: Define the operation contract

Status: completed.

- settle the lifecycle states
- settle the minimum command surface
- settle whether async mode is explicit per command, through `operation start`, or both
- define how operation lifecycle shapes relate to command-specific output shapes

Completed outputs:

- lifecycle contract documented in this task doc
- state model settled to `pending`, `running`, `succeeded`, `failed`, `canceled` for the first milestone
- command-specific output shapes explicitly kept separate from operation lifecycle shapes
- initial checked-in lifecycle shape set added under [../shapes](../shapes/README.md)

Concrete cases:

- `operation status` returns lifecycle state without embedding an entire command result
- `operation result` returns final payload or an `output_id`, but never mixes in event history
- `operation events` returns progress/event history without pretending to be the command result
- command-specific schemas such as `PathIndexTree` remain valid and unchanged whether used synchronously or through operations

### Task 0.2: Define the store layout

Status: completed.

- decide where operations live under the active store
- decide whether results are embedded, referenced, or both
- decide retention/cleanup expectations

Completed outputs:

- operations now persist under `<store>/operations/<operation-id>/`
- snapshot file is `operation.json`
- append-only event log is `events.jsonl`
- final result file is `result.json`
- store initialization now creates the `operations` managed directory

Concrete cases:

- one operation directory can be copied or inspected independently
- a partially written operation snapshot does not corrupt prior readable state
- missing `result.json` still allows operation status inspection
- one failed operation does not corrupt unrelated operations in the same store

### Task 0.3: Define command adoption rules

Status: not started.

- decide which commands remain always synchronous
- decide which commands may opt into long-running mode
- define the first command to migrate, likely `index`

Concrete cases:

- `init` and `logs path` remain simple synchronous commands
- `index` supports both synchronous and async/operation execution during the transition phase
- later commands opt in intentionally instead of inheriting long-running behavior by accident

## Phase 1: Lifecycle Types And Persistence

### Task 1.1: Add operation models

Status: completed.

- add Go types for:
  - accepted
  - status
  - event
  - result
  - summary/list item

Concrete cases:

- shapes can be rendered in both compact text and JSON envelope mode
- lifecycle fields such as `state`, `stage`, and `progress` stay stable across commands
- shape files in `docs/shapes` are distinct from command result shapes

Completed outputs:

- [../shapes/OperationAccepted.schema.json](../shapes/OperationAccepted.schema.json)
- [../shapes/OperationStatus.schema.json](../shapes/OperationStatus.schema.json)
- [../shapes/OperationEventList.schema.json](../shapes/OperationEventList.schema.json)
- [../shapes/OperationResult.schema.json](../shapes/OperationResult.schema.json)
- [../shapes/OperationList.schema.json](../shapes/OperationList.schema.json)
- `internal/operation` Go types now mirror the same lifecycle model

### Task 1.2: Add operation persistence

Status: completed.

- create operation directories/files under the active store
- atomically persist status snapshots
- append events safely

Concrete cases:

- creating an operation writes a readable initial snapshot
- updating progress replaces the snapshot atomically
- appending an event never truncates earlier events
- process interruption between events and snapshot updates still leaves a recoverable operation

Completed outputs:

- `internal/operation` now persists `operation.json`, `events.jsonl`, and `result.json`
- status snapshots use atomic replace semantics through temp-file rename
- event appends are isolated to append-only writes
- read/list helpers exist for later `operation ...` command wiring

### Task 1.3: Add operation IDs

Status: completed.

- generate stable operation IDs
- ensure they are easy for both humans and agents to copy/reference

Concrete cases:

- IDs are short enough for follow-up commands
- IDs are unique across multiple rapid starts
- IDs remain stable across status/event/result lookups

Completed outputs:

- operation ids now use `op_<utc-stamp>_<hex-suffix>`
- ids are persisted as the directory name and lookup key for status/events/result

## Phase 2: Operation Command Surface

### Task 2.1: Add `operation status`

Status: completed.

- inspect one operation
- return current lifecycle state and compact progress

Concrete cases:

- running operation shows stage plus current progress snapshot
- completed operation shows completion metadata
- unknown operation id returns a typed not-found error

Completed outputs:

- `aascribe operation status <operation-id>` now loads persisted lifecycle snapshots
- text mode returns compact state/stage/progress lines
- JSON mode returns the `OperationStatus` payload shape

### Task 2.2: Add `operation events`

Status: completed.

- return event history
- support bounded/default views if the event list grows large

Concrete cases:

- newest events can be inspected without reading the entire history
- text mode returns compact event lines
- JSON mode returns structured events with timestamps and levels

Completed outputs:

- `aascribe operation events <operation-id>` now reads `events.jsonl`
- text mode renders compact event history lines
- JSON mode returns the `OperationEventList` payload shape

### Task 2.3: Add `operation result`

Status: completed.

- return the final payload or output reference
- fail cleanly when the operation is not done yet

Concrete cases:

- succeeded operation returns direct result when small
- succeeded operation returns `output_id` when oversized
- running operation returns a typed "not complete" style error rather than empty success
- failed or canceled operation returns lifecycle-aware result metadata

Completed outputs:

- `aascribe operation result <operation-id>` now reads persisted `result.json`
- not-yet-ready results return `OPERATION_RESULT_NOT_READY`
- JSON mode returns the `OperationResult` payload shape

### Task 2.4: Add `operation cancel`

Status: completed.

- mark the operation as canceled
- propagate cancellation into the running engine through context

Concrete cases:

- canceling a running operation changes later status to `canceled`
- canceling a completed operation is rejected cleanly
- repeated cancel requests are idempotent or clearly rejected, but never corrupt state

Completed outputs:

- `aascribe operation cancel <operation-id>` now marks pending/running operations canceled
- canceled operations get terminal `OperationStatus` and `OperationResult` records
- already canceled operations return an idempotent cancel result
- succeeded or failed operations reject cancellation with `OPERATION_ALREADY_TERMINAL`

### Task 2.5: Add `operation list`

Status: completed.

- list recent operations
- include enough metadata for follow-up inspection

Concrete cases:

- newest operations are easy to inspect in text mode
- JSON mode includes ids, command names, states, and timestamps
- large operation history can later be combined with output transport if needed

Completed outputs:

- `aascribe operation list` now lists persisted operation snapshots for the active store
- text mode returns compact one-line summaries
- JSON mode returns the `OperationList` payload shape

## Phase 3: Engine Integration

### Task 3.1: Add a shared operation reporter

Status: completed.

- allow command engines to emit:
  - stage changes
  - progress updates
  - structured events

Concrete cases:

- the reporter can emit stage-only updates without percentage math
- the reporter can update counts such as `current` / `total`
- engines do not need to know whether the caller is using text or JSON mode

Completed outputs:

- `internal/operation.Reporter` now updates status snapshots and appends events through one command-neutral API
- reporter updates support stage, message, progress, level, and structured event data
- reporter tests verify status persistence and event history writes together

### Task 3.2: Integrate cancellation

Status: completed for `index --async`; not generalized to future commands.

- thread `context.Context` into long-running engines
- ensure cancellation updates operation state consistently
- make partial completion behavior explicit

Current gap:

- `index --async` watches persisted operation state and cancels its context when `operation cancel` marks the operation canceled
- future commands will need to opt into the same cancellation pattern

Concrete cases:

- canceling during a long file-summary phase stops future work and records `canceled`
- canceling after output spill has begun still leaves a readable final operation state
- failed and canceled outcomes remain distinct in status/result responses

### Task 3.3: Integrate `index` first

Status: implemented; smoke testing pending.

- run `index` through the shared operation lifecycle in long-running mode
- report folder/file scan stages
- report reuse vs rebuild progress
- return final `PathIndexTree` result or `output_id`

Concrete cases:

- starting async `index` returns quickly with an `operation_id`
- `operation status` can show stages such as scanning, processing files, writing metadata, finalizing result
- `operation events` shows notable transitions such as reused files, dirty metadata, canceled run, final success
- `operation result` returns the same command-level payload shape as synchronous `index`, just wrapped through the lifecycle path

Completed outputs:

- `aascribe index --async <path>` now creates an `OperationAccepted` payload and starts a background worker process
- the worker runs through hidden internal `operation run-index` plumbing
- operation status moves through pending/running/terminal states
- final index result is persisted to `result.json`
- cancellation is observed by the async worker through operation status polling and `context.Context`

### Task 3.4: Keep synchronous mode intact

Status: completed.

- the existing synchronous command path must remain available
- async operation mode must not silently change the existing `index` envelope contract

Concrete cases:

- `aascribe index ./docs` still behaves synchronously
- `aascribe index --async ./docs` or equivalent enters operation mode
- result payload parity between sync and async modes is verified

Completed outputs:

- synchronous `index` still returns `PathIndexTree`
- async `index` returns `OperationAccepted`
- async final result is retrieved through `operation result`

## Phase 4: Output Transport Integration

### Task 4.1: Connect final results to managed output

Status: not started.

- when final operation result is oversized, store it through the existing output transport
- keep the operation result small and explicit

Concrete cases:

- one oversized completed operation exposes `output_id`
- `operation result` gives explicit next-step hints that match existing `output show/head/tail/slice`
- output transport retention and operation lifecycle do not drift out of sync silently

### Task 4.2: Define operation-to-output hints

Status: not started.

- make it easy for agents to go from `operation result` to `output show/head/tail/slice`

Concrete cases:

- text mode includes one compact follow-up hint
- JSON mode includes structured `output_id` without embedding human-only prose

## Phase 5: Cleanup And Retention

### Task 5.1: Define operation retention policy

Status: not started.

- decide how many operations to retain
- decide how completed/failed/canceled operations are pruned

Concrete cases:

- retention should not silently remove running operations
- pruning should preserve store consistency when result/output references exist
- agents should still be able to inspect recent failed operations after completion

### Task 5.2: Add cleanup command or policy hooks

Status: not started.

- explicit cleanup command is preferred over silent deletion if retained state becomes user-visible

Concrete cases:

- dry-run cleanup is preferred if cleanup becomes recursive or destructive
- cleanup must not remove still-running operations

## Verification

### Task T1: Contract tests

- operation shapes remain distinct from command result shapes
- text mode and JSON mode remain stable
- errors do not corrupt the envelope

Concrete cases:

- `operation status` does not suddenly inline `PathIndexTree`
- `operation result` preserves command-specific shape fidelity
- parse/help text clearly explains lifecycle commands and follow-up steps

### Task T2: Persistence tests

- operation status persists and reloads correctly
- event append is durable
- completed operations expose final result references

Concrete cases:

- process restart or new CLI invocation can still read a running/completed operation
- snapshot replacement does not produce malformed JSON
- missing optional result artifact is surfaced cleanly

### Task T3: Cancellation tests

- canceled operations move to `canceled`
- engines observe cancellation through context
- operation result clearly distinguishes canceled vs failed

Concrete cases:

- cancellation during `index` file processing leaves readable lifecycle state
- cancellation does not masquerade as ordinary failure
- later `status` / `result` lookups remain stable after cancellation

### Task T4: Index integration tests

- long-running `index` can be started and inspected
- progress snapshots update while work is running
- final result is retrievable after completion
- oversized final result can still be browsed through output transport

Concrete cases:

- async `index` on a fixture tree produces readable status transitions
- dirty / unchanged file reuse shows up in progress or events in a stable way
- canceled async `index` leaves command engine metadata and operation lifecycle in consistent states

## Phase 6: Documentation And UX Polish

### Task 6.1: Update `USAGE.md`

- document the operation command group
- document how long-running mode is entered for supported commands
- document how operations and output transport interact

### Task 6.2: Add shape files

- add checked-in schemas for:
  - `OperationAccepted`
  - `OperationStatus`
  - `OperationEventList`
  - `OperationResult`
  - `OperationList`

### Task 6.3: Add examples for agent flows

- start long-running `index`
- inspect status
- inspect events
- fetch result
- follow `output_id` if present

## Acceptance Criteria

The first acceptable operation implementation should:

- provide one shared lifecycle contract for long-running commands
- keep lifecycle state separate from command-specific output payloads
- support status, events, result, and cancel flows
- integrate with context cancellation
- integrate cleanly with managed output transport
- support at least one real migrated command, ideally `index`

The first acceptable `index` migration should additionally:

- preserve synchronous `index` behavior for existing callers
- expose an explicit long-running mode with a stable operation handle
- report meaningful progress/state transitions during execution
- return the same command-level result shape at completion as the synchronous path

## Recommended Sequencing

Recommended delivery order:

1. define operation lifecycle shapes and store layout
2. add operation persistence and read APIs
3. add `operation status/events/result/cancel/list`
4. add operation reporter hooks
5. migrate `index` in long-running mode
6. integrate operation results with output transport
7. add retention/cleanup policy

## Open Risks

- overloading operation state with command-specific semantics will make the lifecycle model inconsistent across commands
- mixing operation events into the normal success envelope will break both text and JSON callers
- storing too much final result directly inside lifecycle snapshots may duplicate existing output transport responsibilities
- cancellation semantics must stay explicit, especially for partial work such as `index`

## Open Questions

- should async execution be triggered by `operation start`, a command flag such as `--async`, or both
- should operation events be command-neutral only, or allow command-specific structured event payloads
- should partial-success be its own operation state, or remain command-specific result detail
- should operation retention be fixed-count, time-based, or user-managed
