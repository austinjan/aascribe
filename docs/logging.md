# Logging

`aascribe` keeps user-facing command output and debug logs separate.

- command results go to stdout
- logs go to stderr and, when enabled, to a log file alongside the active store directory

This keeps JSON output machine-readable while still giving you useful diagnostics.

## Verbose Mode

Use `--verbose` to enable debug logging:

```bash
go run ./cmd/aascribe -- --verbose --store ./project-mem list
```

Without `--verbose`, `aascribe` still emits important lifecycle and error logs, but keeps the volume lower.

## Active Log File

The standard log file path is:

```text
<store-parent>/logs/aascribe.log
```

You can ask `aascribe` for the resolved active path:

```bash
aascribe --store ./project-mem logs path
```

## Log Commands

### Return Log File Path

```bash
aascribe --store ./project-mem logs path
```

### Export Log File

```bash
aascribe --store ./project-mem logs export --output ./aascribe-debug.log
```

### Clear Log File

```bash
aascribe --store ./project-mem logs clear --force
```

## What Gets Logged

Current logging includes:

- command start
- command finish
- command failures
- resolved store path
- selected output format
- durations

The project rule is that new features should also add meaningful logs for:

- important decisions
- retries
- cache hits/misses
- file processing paths
- provider interactions

## What Does Not Get Logged

Secrets must never be written to logs.

Examples of protected values:

- API keys
- tokens
- passwords
- secret config values

When a sensitive field name appears in structured logs, its value is redacted.

## Stdout vs Stderr

If you use JSON output, stdout remains safe for piping:

```bash
aascribe --format json --store ./project-mem logs path | jq .
```

Logs are written separately to stderr, so they do not corrupt the JSON payload.
