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
./data/config/config.toml
```

Examples:

- default store `./data/memory`: `./data/config/config.toml`
- explicit store `./project-mem`: `./config/config.toml`

The memory store is still resolved using the same logic as the CLI:

1. `--store <path>`
2. `AASCRIBE_STORE`
3. `./data/memory` in the current working directory

The config directory is a sibling `config/` directory beside the memory store directory.
This keeps durable memory data separate from config and `.env` secret files.

`aascribe init` creates a default `config.toml` skeleton when the config file does not already exist.
It preserves an existing config, including when `--force` is used.

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

The default skeleton can safely fill in non-secret fields. Secret fields cannot be assigned a default value.

Required fields for summary-backed `index` and other LLM-backed commands:

| Field | Required | Meaning |
|---|---:|---|
| `llm.provider` | yes | Must be `"gemini"` in the current implementation |
| `llm.model` | yes | Gemini model used for summaries and debug LLM commands |
| `llm.api_key_env` | yes | Environment variable name that stores the secret key |
| `llm.timeout_seconds` | yes | Positive request timeout |
| `gemini_api_key` / `GEMINI_API_KEY` | yes | Secret Gemini API key value. No default is possible |
| `defaults.format` | no | Default output format. Defaults to `json` when omitted |
| `index.max_file_size` | no | Maximum file size for summarization. Defaults to `1048576` |
| `index.default_depth` | no | Default index depth. Defaults to `3` |

The secret value itself is not stored in `config.toml`. Set it with:

```bash
aascribe config gemini_api_key="your-real-key"
```

or provide the environment variable directly:

```bash
export GEMINI_API_KEY="your-real-key"
```

## Quick Start

### Default Store Setup

```bash
aascribe config gemini_api_key="your-real-key"

aascribe describe ./README.md
```

### Skill-Friendly Explicit Store

```bash
aascribe --store ./project-mem config gemini_api_key="your-real-key"

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

`aascribe` could not find the resolved `config.toml`.

Fix:

- run `aascribe config gemini_api_key="your-real-key"` to create the default config skeleton and store the required secret
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

The environment variable named by `api_key_env` is missing or empty. This is a required secret for summary-backed `index`; it has no default.

Fix:

```bash
aascribe config gemini_api_key="your-real-key"
```
