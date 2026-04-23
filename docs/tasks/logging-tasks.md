# Logging And Debugging Tasks

Status: Partially Completed

Completed in current branch:

- added a shared logger package
- wired `--verbose` to debug logging
- kept logs separate from stdout command output
- added stderr logging plus store-local file logging at `<store>/logs/aascribe.log`
- added `logs path`, `logs export`, and `logs clear`
- added logger and log-command tests
- added a user-facing logging guide

Task breakdown for adding a logging system to `aascribe` and making detailed logs a default engineering expectation for future features.

## Goal

Make `aascribe` observable enough that engineers and skills can debug failures, understand behavior, and analyze what happened during command execution without polluting normal command output.

The logging system should support:

- local debugging during development
- production-style command diagnostics
- tracing feature behavior such as config loading, summarization, indexing, and metadata reuse
- later analysis of failures and performance bottlenecks

## Core Rules

- User-facing command results stay on stdout
- Logs go to stderr and/or dedicated log sinks
- Logs must never corrupt JSON command output
- `--verbose` enables detailed debug logging
- New features should add meaningful logs as part of implementation, not as an afterthought
- Secrets must never be logged

## Phase 1: Logging Foundation

### Task 1.1: Add a shared logger package

- Create an internal logging package
- Support at least:
  - debug
  - info
  - warn
  - error
- Prefer structured key/value logging over ad hoc free-form strings

Status: Completed

### Task 1.2: Wire `--verbose` to logging behavior

- Make `--verbose` actually enable debug-level logs
- In non-verbose mode, keep logs minimal
- Do not change stdout command payload behavior

Status: Completed

### Task 1.3: Define default log destination

MVP recommendation:

- write logs to stderr by default
- keep stdout reserved for command results

Future-friendly extension:

- optional file logging under `<store>/logs/`

Status: Completed for stderr + store-local file logging

### Task 1.4: Add log management commands

Plan user-facing commands for:

- exporting the current log file
- clearing the current log file
- returning the active log file path

Recommended command shapes:

- `aascribe [global flags] logs path`
- `aascribe [global flags] logs export [--output <path>]`
- `aascribe [global flags] logs clear [--force]`

Behavior expectations:

- `logs path` returns the resolved log file path in both text and JSON modes
- `logs export` copies or writes the current log file to a target path
- `logs clear` truncates the active log file without breaking future logging
- these commands should respect the current store path resolution rules

Status: Completed

## Phase 2: Command Lifecycle Logs

### Task 2.1: Log command start and finish

For each command execution, log:

- command name
- resolved store path
- selected output format
- start timestamp
- finish status
- duration

Status: Completed

### Task 2.2: Log top-level failures

- Record the error code and failure stage
- Keep the user-facing error message clean
- Include enough context for debugging without exposing secrets

Status: Completed

## Phase 3: Config And Secret Logs

### Task 3.1: Log config resolution steps

Log:

- config path used
- whether config file was found
- whether config validation passed
- which precedence source won for each overridable field

Do not log:

- raw API keys
- full secret values

### Task 3.2: Log secret resolution safely

Allowed:

- env var name used, such as `GEMINI_API_KEY`
- whether the secret was found

Forbidden:

- secret contents
- token fragments

## Phase 4: Feature-Level Logging Requirements

### Task 4.1: LLM feature logs

When LLM-backed features are implemented, log:

- model selected
- provider selected
- request start/end
- timeout/retry events
- auth failures
- response parsing failures
- token usage if available

### Task 4.2: Index feature logs

When `index` is implemented, log:

- root path
- recursion depth
- include/exclude patterns
- metadata file load/save
- file counts
- changed vs unchanged file counts
- summary reuse decisions
- per-file failures
- child-before-parent recursion milestones

### Task 4.3: Describe feature logs

When `describe` is implemented, log:

- file path
- extraction result
- truncation decisions
- summary length requested
- LLM success/failure path

## Phase 5: Optional File Logging

### Task 5.1: Add store-local log directory support

Future path:

- `<store>/logs/aascribe.log`

Requirements:

- log file creation should not be mandatory for command success
- file logging should be additive to stderr, not replace it blindly

Status: Completed for `<store>/logs/aascribe.log`

### Task 5.2: Rotation strategy

Do not implement rotation in the first pass unless trivial.

But leave room for:

- size-based rotation
- date-based rotation

### Task 5.3: Define active log path resolution

- Standard active log path:
  - `<store>/logs/aascribe.log`
- `logs path` should expose this resolved path directly
- `logs export` and `logs clear` should operate on this same resolved path
- if file logging is disabled or the file does not exist yet, commands should return a clear structured result or error

Status: Completed

## Phase 6: Documentation

### Task 6.1: Document logging behavior

Add a user-facing logging section that explains:

- stdout vs stderr behavior
- what `--verbose` does
- what kinds of details are logged
- what will not be logged for security reasons
- how to find the active log file
- how to export logs
- how to clear logs

Status: Completed

### Task 6.2: Add implementation guidance

Document for contributors that:

- every new feature should emit meaningful logs
- logs should explain decisions, not just errors
- secrets must never appear in logs

## Phase 7: Tests

### Task T1: Logger behavior tests

- debug logs appear only in verbose mode
- stdout output remains unchanged
- stderr logging does not corrupt JSON output

Status: Completed

### Task T2: Security tests

- secrets are not emitted in logs
- config secret values are redacted or omitted completely

Status: Completed for structured logger redaction tests

### Task T3: Feature log tests

- config loading emits expected debug events
- command lifecycle logs are present
- future `describe`/`index` logging hooks can be asserted in tests

### Task T4: Log command tests

- `logs path` returns the expected file path
- `logs export` writes a copy of the log file
- `logs clear` truncates the log file safely
- these commands do not corrupt stdout JSON response handling

Status: Completed

## Acceptance Criteria

The first acceptable logging implementation should:

- provide a shared structured logger
- make `--verbose` meaningful
- keep logs separate from stdout command payloads
- log command lifecycle and config resolution
- provide commands to return, export, and clear the active log file
- establish a project rule that new features must include useful debug/analyze logs

## Out Of Scope For First Logging Milestone

- distributed tracing
- external log backends
- metrics dashboards
- full log rotation management
