# PROGRESS.md

Single source of truth for current progress, WIP, backlog, completed work, and validation evidence.

## Current Focus

Establish a durable developer/agent harness for `aascribe` so future sessions can recover project context, constraints, architecture, decisions, and verification commands without relying on chat history.

## Progressing

- None. WIP limit: 1.

## Pending

- Keep this file updated whenever implementation work begins, pauses, completes, or changes priority.
- Add feature-specific backlog items here before starting future multi-step implementation work.
- Record validation results after each verification run.

## Done

- Implemented `install skills` and `uninstall skills` for the bundled `indexing-folder` skill, including user scope (`~/.codex`) and folder scope (`<folder>/skills/indexing-folder`) targets.
- Audited repository structure, existing docs, Go module metadata, Makefile targets, source directories, tests, and installation workflow.
- Identified `docs/USAGE.md` as the public CLI contract and `docs/shapes/` as the checked-in output data schema source of truth.
- Created harness files: `AGENTS.md`, `CONSTRAINS.md`, `ARCHITECTURE.md`, `DECISIONS.md`, and `instructions/`.
- Ran repository verification for the new harness baseline.
- Verified the current memory command surface against a temporary store.

## Validation Log

- 2026-06-30: `go test ./internal/cli ./internal/skill ./internal/command ./internal/app ./internal/output` passed.
- 2026-06-30: `go test ./...` passed.
- 2026-06-30: Manual temp-folder smoke test passed for `go run ./cmd/aascribe --format json install skills -f "$tmpdir"` followed by file/executable checks and `go run ./cmd/aascribe --format json uninstall skills -f "$tmpdir"`.
- 2026-06-30: `go vet ./...` passed with no reported issues.
- 2026-06-30: `make` passed and built `dist/aascribe`.
- 2026-06-30: `go test ./...` passed. Packages with tests passed under `internal/app`, `internal/command`, `internal/config`, `internal/index`, `internal/logging`, `internal/operation`, `internal/output`, and `pkg/llmoutput`.
- 2026-06-30: `go vet ./...` passed with no reported issues.
- 2026-06-30: `make` passed and built `dist/aascribe`.
- 2026-06-30: `go test ./internal/cli ./internal/command ./internal/app ./internal/store ./internal/output` passed.
- 2026-06-30: Manual temporary-store memory smoke test passed for `init`, but `remember`, `list`, `recall`, `show`, `consolidate --dry-run`, and `forget --force` all returned documented `NOT_IMPLEMENTED` errors with exit status 1.
