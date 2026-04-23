# Quick Start

## Build A Release Binary

Build an optimized release binary with:

```bash
cargo build --release
```

The compiled binary will be written to:

```bash
target/release/aascribe
```

Example:

```bash
./target/release/aascribe init
```

## Run In Dev Mode

For day-to-day development and debugging, run the CLI through Cargo:

```bash
cargo run -- <command> [args...]
```

Examples:

```bash
cargo run -- init
cargo run -- --format text init
cargo run -- --store ./tmp-store init --force
```

This uses the debug build, which is slower than `--release` but much better for iterative development and troubleshooting.

## Useful Debug Commands

Run the test suite:

```bash
cargo test
```

Format the code:

```bash
cargo fmt
```
