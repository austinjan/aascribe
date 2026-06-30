# CONSTRAINS.md

Repository-wide hard constraints. Every rule in this file is mandatory.

## Git And Commit Rules

- Commits MUST be meaningful and atomic: one coherent change per commit, including its matching docs and tests.
- Commits MUST NOT bundle unrelated refactors, generated artifacts, formatting churn, or local scratch files with feature work.
- Agents MUST inspect `git status --short` before editing and before staging.
- Agents MUST NOT revert, overwrite, or discard user changes unless the user explicitly instructs that action.
- Agents MUST stage only files that are part of the requested change.

## Coding Style Constraints

- Go files MUST be formatted with `gofmt` before validation or staging.
- New code MUST keep CLI parsing, command execution, output rendering, logging, and storage responsibilities separated by package boundaries.
- Public CLI behavior MUST remain consistent with `docs/USAGE.md`.
- Machine-readable JSON output MUST preserve the documented envelope with `ok`, `data` or `error`, and `meta`.
- Command result output MUST go to stdout.
- Logs MUST go to stderr and/or configured log files, not into stdout JSON payloads.
- New command data shapes MUST be documented in `docs/USAGE.md` and represented in `docs/shapes/` when they are public `OutputShape` contracts.
- Tests MUST use deterministic temporary directories and fixtures instead of depending on a developer's local store.

## Safety And Security Rules

- Secrets, API keys, tokens, passwords, and raw credentials MUST NOT be committed.
- Secrets MUST NOT be written to logs, stdout examples, validation logs, or checked-in config files.
- LLM provider secrets MUST be loaded through environment variables named by config, such as `GEMINI_API_KEY`.
- Command-line flags MUST NOT carry raw secrets unless the user explicitly approves a security model change.
- Error messages MUST provide recovery guidance without exposing sensitive values.
- Generated build outputs, memory stores, logs, and indexes MUST NOT be treated as source of truth.

## Documentation And State Rules

- `PROGRESS.md` MUST reflect current focus, WIP, backlog, completed work, and validation evidence.
- `PROGRESS.md` MUST keep WIP limited to one actively progressing item.
- `ARCHITECTURE.md` MUST change when module ownership, public interfaces, data contracts, or external dependencies change.
- `DECISIONS.md` MUST record durable architecture or product contract decisions.
- Topic files under `instructions/` MUST be updated when workflow instructions change.

