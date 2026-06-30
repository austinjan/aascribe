# Development Instructions

Use this file when changing Go source, CLI behavior, docs, state files, or repository workflow.

## Session Start

1. Inspect `git status --short`.
2. Read `PROGRESS.md`, `CONSTRAINS.md`, and the relevant topic file.
3. For CLI changes, read `docs/USAGE.md` before editing.
4. For output data changes, read `docs/shapes/README.md` and the affected schema files.

## Code Workflow

- Keep changes small and scoped to the requested behavior.
- Preserve package boundaries:
  - parsing in `internal/cli`
  - orchestration in `internal/app`
  - command behavior in `internal/command`
  - rendering in `internal/output`
  - store layout in `internal/store`
  - logging in `internal/logging`
  - indexing in `internal/index`
  - async lifecycle in `internal/operation`
- Run `gofmt -w` on edited Go files before tests.
- Do not introduce broad abstractions until at least two real callers need them.

## Documentation Synchronization

- Update `docs/USAGE.md` for any public command, flag, output, or exit-code change.
- Update `docs/configuration.md` for config precedence, config shape, or env var behavior changes.
- Update `docs/logging.md` for logging, redaction, stdout/stderr, or log path changes.
- Update `docs/installation.md` for build, release, binary path, or install script changes.
- Update `docs/shapes/` when public output data contracts change.
- Update `ARCHITECTURE.md` when module responsibilities or public interfaces change.
- Update `DECISIONS.md` for durable architecture or product contract decisions.
- Update `PROGRESS.md` for WIP, completion, and validation evidence.

## Git Workflow

- Stage only the files that belong to the requested change.
- Keep commits atomic and meaningful.
- Do not commit generated `dist/`, local stores, logs, temporary files, or unrelated user changes.
- If unrelated dirty files exist, leave them alone and mention them in the handoff only when relevant.

## Local Data Safety

- Treat `./data/`, generated index metadata, and logs as runtime artifacts.
- Use temporary directories for manual command experiments when possible.
- Do not run cleanup commands that remove user data unless the user explicitly asks for cleanup.

