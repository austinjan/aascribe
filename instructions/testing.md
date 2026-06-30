# Testing Instructions

Use this file when writing tests, adjusting test fixtures, validating output contracts, or recording verification evidence.

## Standard Verification Commands

```bash
go test ./...
go vet ./...
make
```

Use `make release` when release packaging, cross-platform binaries, or install scripts are affected.

## Test Placement

- Tests for internal packages MUST live beside the package under `internal/...`.
- Tests for public package behavior MUST live beside the package under `pkg/...`.
- CLI integration behavior SHOULD be tested through `internal/app` or command-level tests when possible.
- Filesystem indexing tests MUST use deterministic fixtures under `tests/index-fixtures/` or temporary directories created by tests.

## Output Contract Tests

- Tests for `--format json` behavior MUST preserve the documented envelope.
- Tests for public command data payloads MUST stay aligned with shape names in `docs/USAGE.md`.
- When a command's public data shape changes, update or add the matching schema under `docs/shapes/`.
- Text output tests SHOULD check important user-facing hints without making harmless formatting too brittle.

## Mocking And External Services

- Tests MUST NOT require a real Gemini API call.
- Tests MUST inject secrets with `t.Setenv` or local temporary `.env` files only.
- Tests MUST NOT read a developer's real `GEMINI_API_KEY` unless the test is explicitly an opt-in integration test.
- LLM-backed behavior SHOULD be tested through seams such as command runners, fake config, or deterministic fixtures.

## Validation Log

After running verification, update `PROGRESS.md` with:

- command executed
- pass/fail status
- notable failure summary when failing
- date of the run

