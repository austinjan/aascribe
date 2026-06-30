# ARCHITECTURE.md

Durable architecture map for `aascribe`.

## System Summary

`aascribe` is a Go command-line application that manages local project memory stores, indexes folders, searches indexed or live content, runs LLM-backed description/summarization flows, and emits agent-friendly text or machine-readable JSON output. The CLI is filesystem-first and keeps command results, logs, config, and generated operation state separated.

## Module Responsibilities

- `cmd/aascribe/`: process entrypoint. Calls `internal/app.Run` and exits with its normalized status code.
- `internal/app/`: top-level runtime orchestration. Handles help/version fast paths, parsing, store resolution, logger setup, command execution, output writing, and exit-code normalization.
- `internal/cli/`: command-line parsing, help text, command structs, global flags, and parse errors.
- `internal/command/`: command dispatch and implementation glue across store, index, operation, output, config, search, and LLM packages.
- `internal/apperr/`: typed application errors and documented status-code mapping.
- `internal/store/`: store path resolution and filesystem layout bootstrap.
- `internal/config/`: TOML config loading, environment overrides, `.env` loading, and secret resolution by configured environment variable name.
- `internal/logging/`: structured logging, stderr/file sinks, and sensitive value redaction.
- `internal/output/`: text and JSON envelope rendering, output data shape mapping, and recovery hints.
- `internal/index/`: folder indexing, `.aaignore` handling, path tree generation, metadata, summaries, map output, and index maintenance.
- `internal/search/`: search behavior used by CLI search commands.
- `internal/skill/`: bundled skill installation and removal, including embedded skill assets, target-scope resolution, binary copying, and managed skill cleanup.
- `internal/operation/`: persisted async operation lifecycle, status, events, results, cancellation, and cleanup.
- `internal/llm/`: Gemini client integration for LLM-backed commands.
- `pkg/llmoutput/`: public package for parsing and representing LLM output blocks.
- `docs/`: product contract, installation, configuration, logging, plans, archived task context, schema docs, and reference material.
- `docs/shapes/`: checked-in JSON Schema files for public command `OutputShape` data payloads.
- `skills/indexing-folder/`: bundled agent skill that wraps `aascribe` indexing, mapping, searching, operations, and output transport workflows.
- `scripts/`: install scripts for copying the bundled indexing skill and binary into target roots.
- `tests/index-fixtures/`: deterministic filesystem fixtures for index behavior.
- `dist/`: generated build output from `make` and `make release`; not source.

## Public Interfaces / APIs

- CLI entrypoint: `aascribe` built from `./cmd/aascribe`.
- Development entrypoint: `go run ./cmd/aascribe -- <command> [args...]`.
- Global flags: `--store`, `--format`, `--quiet`, `--verbose`, `--help`, `--version`.
- Primary command contract: `docs/USAGE.md`.
- JSON output envelope:

```json
{
  "ok": true,
  "data": {},
  "meta": {
    "command": "example",
    "duration_ms": 1,
    "store": "/path/to/store"
  }
}
```

- Error envelope:

```json
{
  "ok": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable recovery guidance."
  },
  "meta": {}
}
```

- Output data contracts: shape names documented in `docs/USAGE.md` and schemas in `docs/shapes/`.
- Store default: `./data/memory`, overridden by `AASCRIBE_STORE` or `--store`.
- Config default: sibling `./data/config/config.toml`, with secrets loaded from environment variables named by config.
- Logs: stderr plus `<store-parent>/logs/aascribe.log` where applicable.
- Index metadata: `.aascribe_index_meta.json` may be written into indexed directories.
- Skill interface: `skills/indexing-folder/SKILL.md` documents how agents call the bundled binary.
- Skill installation targets: `aascribe install skills --user` writes to `~/.codex/skills/indexing-folder/`; `aascribe install skills -f <folder>` writes to `<folder>/skills/indexing-folder/`; uninstall removes the same managed skill directory.

## Dependencies

- Go toolchain declared by `go.mod`: `go 1.26.0`.
- TOML parsing: `github.com/BurntSushi/toml`.
- Environment file loading: `github.com/joho/godotenv`.
- LLM integration: `google.golang.org/genai` and Google Cloud auth/transport dependencies.
- Test comparison support: `github.com/google/go-cmp`.
- Build system: `Makefile` targets `all`, `release`, and `clean`.
- External service risk: Gemini-backed commands require a configured `GEMINI_API_KEY` or equivalent env var named by config.
- Filesystem risk: indexing and store operations depend on local file permissions, `.aaignore` behavior, and generated metadata cleanup.
