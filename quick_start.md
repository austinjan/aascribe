# Quick Start

See also:

- [Configuration](./docs/configuration.md)

## Build A Release Binary

Build the CLI with:

```bash
go build -o bin/aascribe ./cmd/aascribe
```

The compiled binary will be written to:

```bash
bin/aascribe
```

Example:

```bash
./bin/aascribe init
```

## Run In Dev Mode

For day-to-day development and debugging, run the CLI directly through `go run`:

```bash
go run ./cmd/aascribe -- <command> [args...]
```

Examples:

```bash
go run ./cmd/aascribe -- init
go run ./cmd/aascribe -- --format text init
go run ./cmd/aascribe -- --store ./tmp-store init --force
```

This uses a development build path, which is ideal for fast iteration and troubleshooting.

## Useful Debug Commands

Run the test suite:

```bash
go test ./...
```

Format the code:

```bash
gofmt -w ./cmd ./internal
```
