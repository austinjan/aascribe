# AGENTS.md

Entry router for developer and agent work in this repository.

## Project Overview

`aascribe` is an early-stage Go CLI for project memory, recall, indexing, structured command output, and agent-friendly folder inspection. This repository owns the CLI implementation, its public command contract, JSON output shapes, bundled indexing skill, installation scripts, and project documentation.

## First-Run Commands

```bash
go mod download
go test ./...
go vet ./...
make
```

Useful development commands:

```bash
go run ./cmd/aascribe -- --help
go run ./cmd/aascribe -- --format json init --force
gofmt -w ./cmd ./internal ./pkg
make release
```

## What To Read First

1. Read `PROGRESS.md` for current focus, WIP, and latest validation evidence.
2. Read `CONSTRAINS.md` for non-negotiable project rules.
3. Read `docs/USAGE.md` before changing CLI behavior.
4. Read the applicable topic file under `instructions/` before implementation.

## Global Hard Constraints

1. Agents MUST keep `docs/USAGE.md` aligned with any user-visible command, flag, output, or exit-code change.
2. Agents MUST keep `docs/shapes/` aligned with documented `OutputShape` contracts.
3. Agents MUST keep command results on stdout and logs on stderr or configured log files.
4. Agents MUST NOT write secrets, API keys, tokens, or passwords to source, docs, config examples with real values, logs, tests, or validation output.
5. Agents MUST resolve LLM secrets through environment variables such as `GEMINI_API_KEY`, not through command-line secret flags.
6. Agents MUST preserve the standard JSON envelope shape for `--format json` responses unless the user explicitly approves a contract migration.
7. Agents MUST preserve documented exit-code semantics unless the user explicitly approves a contract migration.
8. Agents MUST use `gofmt` on edited Go files before validation.
9. Agents MUST run the narrowest relevant tests for small changes and `go test ./...` before staging broad or contract-level changes.
10. Agents MUST NOT treat generated `dist/`, `bin/`, logs, or store data as source files.
11. Agents MUST avoid destructive cleanup commands unless the user explicitly asks for cleanup.
12. Agents MUST keep WIP in `PROGRESS.md` limited to one actively progressing item.
13. Agents MUST update project state files when changing constraints, architecture, decisions, or progress.
14. Agents MUST make meaningful atomic commits: each commit must contain one coherent change with its matching docs/tests, and unrelated work MUST NOT be bundled.
15. Agents MUST inspect current repo state before editing and MUST NOT revert user changes unless explicitly instructed.

## Required Project State Files

- `PROGRESS.md`: single source of truth for current focus, WIP, backlog, completed work, and validation logs. Update it at the start/end of meaningful work and after every verification run.
- `CONSTRAINS.md`: repository-wide hard constraints. Every rule must use `MUST` or `MUST NOT`; do not place soft preferences here.
- `ARCHITECTURE.md`: responsibilities, public interfaces, data contracts, and dependencies. Update it when module ownership or external contracts change.
- `DECISIONS.md`: durable architecture/design decision log. Record important decisions, rejected alternatives, and consequences so future sessions do not re-litigate settled choices.

## Topic Instructions Links

- `instructions/testing.md`: use when writing tests, mocks, fixtures, schema checks, or running verification.
- `instructions/development.md`: use when changing Go code, CLI contracts, docs, project state, or git workflow.
- `instructions/deployment.md`: use when building binaries, release artifacts, install scripts, or bundled skills.

## Existing Reference Docs

- `README.md`: project overview and doc index.
- `quick_start.md`: local build, run, test, and formatting examples.
- `docs/USAGE.md`: primary CLI command contract.
- `docs/configuration.md`: config precedence and secret loading model.
- `docs/logging.md`: stdout/stderr/logging separation and redaction expectations.
- `docs/installation.md`: build, release, and skill installation workflow.
- `docs/shapes/README.md`: JSON schema policy for command output data shapes.

