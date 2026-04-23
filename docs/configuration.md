# Configuration

`aascribe` uses a skill-friendly configuration model:

- config file for durable defaults
- environment variables for secrets
- CLI flags for per-command overrides

Precedence order:

1. CLI flags
2. environment variables
3. config file

## Config File Location

The main config file lives at:

```text
<store>/config.toml
```

Examples:

- default store: `~/.aascribe/config.toml`
- explicit store: `./project-mem/config.toml`

`<store>` is resolved using the same logic as the CLI:

1. `--store <path>`
2. `AASCRIBE_STORE`
3. `~/.aascribe`

## Why Secrets Use Environment Variables

For the Gemini API key, the config file stores the name of an environment variable, not the raw secret value.

Example:

```toml
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
```

Then `aascribe` loads the actual key from:

```bash
export GEMINI_API_KEY="your-real-key"
```

This keeps secrets out of normal config files and works better for skills and automation.

## Supported Config Shape

First supported schema:

```toml
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30

[defaults]
format = "json"

[index]
max_file_size = 1048576
default_depth = 3
```

## Quick Start

### Default Store Setup

```bash
mkdir -p ~/.aascribe

cat > ~/.aascribe/config.toml <<'EOF'
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
EOF

export GEMINI_API_KEY="your-real-key"

go run ./cmd/aascribe -- describe ./README.md
```

### Skill-Friendly Explicit Store

```bash
mkdir -p ./project-mem

cat > ./project-mem/config.toml <<'EOF'
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
EOF

export GEMINI_API_KEY="your-real-key"

./bin/aascribe --store ./project-mem describe ./main.go
```

## How Skills Should Use `aascribe`

Skills should:

- assume config exists in the target store
- pass task-specific arguments only
- rely on environment injection for secrets
- avoid passing secrets on the command line

Examples:

```bash
aascribe --store ./project-mem describe ./file.go
aascribe --store ./project-mem index . --depth 2
```

## Environment Overrides

These env vars can override selected non-secret settings:

- `AASCRIBE_FORMAT`
- `AASCRIBE_LLM_MODEL`
- `AASCRIBE_LLM_TIMEOUT_SECONDS`

Example:

```bash
export AASCRIBE_LLM_MODEL="gemini-2.5-pro"
export AASCRIBE_LLM_TIMEOUT_SECONDS="45"
```

## Common Errors

### `CONFIG_NOT_FOUND`

`aascribe` could not find `<store>/config.toml`.

Fix:

- create the config file in the resolved store
- or pass the correct `--store` path

### `INVALID_CONFIG`

The config file exists but is missing required fields or contains invalid values.

Check:

- `[llm]` exists
- `provider = "gemini"`
- `model` is set
- `api_key_env` is set
- `timeout_seconds` is a positive integer

### `MISSING_SECRET`

The environment variable named by `api_key_env` is missing or empty.

Fix:

```bash
export GEMINI_API_KEY="your-real-key"
```
