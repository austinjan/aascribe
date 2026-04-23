# Bootstrap Plan For `aascribe` In Go

## Summary

Bootstrap `aascribe` as a Go CLI with the full documented command skeleton in place, but only `init` fully implemented in the first phase. The initial store will be filesystem-first rather than database-first, and the usage spec will be updated now so the documented behavior matches reality. The bootstrap should establish the long-term architecture once: shared argument parsing, output envelope handling, error/exit-code mapping, and a command module layout that future subcommands can fill in without structural churn.

## Implementation Changes

- Create a Go CLI project with a single entrypoint and internal packages for:
  - command parsing
  - shared output envelope rendering (`json` and `text`)
  - error types and exit-code mapping
  - store path resolution
  - one module per documented subcommand
- Use a full command parser matching the existing CLI surface:
  - global flags: `--store`, `--format`, `--quiet`, `--verbose`, `--help`, `--version`
  - subcommands: `init`, `index`, `describe`, `remember`, `consolidate`, `recall`, `list`, `show`, `forget`
- Implement `init` as the only working command in bootstrap:
  - resolve store path from `--store`, else `$AASCRIBE_STORE`, else `~/.aascribe`
  - create the root store directory and filesystem-first subdirectories/files needed for later memory and index features
  - support `--force` by reinitializing the store deterministically
  - return the standard JSON envelope on success/error
  - provide a human-readable text mode equivalent
- For non-`init` subcommands, wire them into the parser and shared response path but have them return a consistent “not implemented yet” runtime error with the standard envelope and a general nonzero exit code
- Update the usage documentation so `init` describes a filesystem-first store layout rather than claiming that databases are created immediately
- Keep the store layout future-friendly:
  - separate areas for short-term memory, long-term memory, and index/cache concerns
  - avoid embedding backend-specific assumptions into command interfaces yet
  - make later migration to SQLite possible without changing CLI flags or output shape

## Public Interfaces And Defaults

- CLI contract remains the main public interface; bootstrap preserves all documented command names and global flags
- `init` behavior for bootstrap:
  - success: creates or re-creates the store layout and reports the resolved absolute store path in metadata
  - existing store without `--force`: returns a structured error rather than partially overwriting
  - `--force`: removes/rebuilds bootstrap-managed store contents in a deterministic way
- Output contract:
  - JSON stays the default format
  - all command responses use the documented `ok` / `data` or `error` / `meta` envelope
  - `meta.command`, `meta.duration_ms`, and `meta.store` are always present when applicable
- Documentation contract:
  - [docs/USAGE.md](/Users/macmini-au/code/aascribe/docs/USAGE.md:1) becomes the bootstrap source of truth and explicitly reflects the filesystem-first initial implementation

## Test Plan

- CLI parsing tests:
  - global flags parse correctly with each subcommand
  - `init --store <path>` and `init --force` parse correctly
  - undocumented flags/arguments fail with invalid-arguments exit behavior
- `init` behavior tests:
  - creates a new store in an empty temp directory
  - uses environment/default path precedence correctly
  - fails cleanly when the store already exists and `--force` is not set
  - reinitializes cleanly when `--force` is set
  - produces both JSON and text output in the expected shape
- Shared contract tests:
  - JSON envelope shape matches the usage doc
  - error codes map to documented exit codes where already defined
  - stubbed commands return a consistent “not implemented yet” error path without breaking the envelope

## Assumptions

- Go is the implementation language for bootstrap
- The bootstrap should establish the full CLI skeleton now, not only the `init` command
- Filesystem-first storage is intentional for v1 bootstrap, and the spec should be revised now to match that choice
- It is acceptable for non-`init` subcommands to exist as parser-visible stubs during bootstrap
- The implementation should favor stable architecture and contract correctness over premature database design
