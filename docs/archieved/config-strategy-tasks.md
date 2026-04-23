# Skill-Friendly Configuration Tasks

Status: Completed

Completed outcomes:

- added a shared Go config loader
- implemented `flags > env > config` precedence
- switched secret loading to `api_key_env`
- added config validation and tests
- added user-facing config documentation and quick-start examples
- linked the config guide from top-level docs

Task breakdown for implementing a configuration strategy that works well when `aascribe` is called by skills, agents, and local users.

This document defines the recommended settings model for the CLI:

- config file for durable defaults
- environment variables for secrets and temporary session overrides
- CLI flags for per-invocation overrides

## Goal

Make `aascribe` easy to use from skills without forcing each skill to restate every setting on every command, while also avoiding raw secret storage in normal config files.

The configuration model should support:

- local manual CLI use
- skill-driven automation
- machine-specific secrets
- reproducible default behavior
- safe per-call overrides

## Core Decision

Implement a 3-level precedence model:

1. CLI flags
2. environment variables
3. config file

Responsibilities:

- config file:
  - provider
  - model
  - timeout
  - default store behavior
  - indexing defaults
  - summarization defaults
  - env var name for secrets
- environment variables:
  - actual API keys
  - temporary runtime overrides where useful
- CLI flags:
  - invocation-specific behavior for one command

## Phase 1: Config Model

### Task 1.1: Define the config file location

- Keep the base config at `<store>/config.toml`
- Continue using existing store path resolution first
- Ensure skills can rely on the config being colocated with the target store

### Task 1.2: Define the first config schema

Recommended MVP shape:

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

Rules:

- `llm.provider` is required
- `llm.model` is required
- `llm.api_key_env` is required
- `llm.timeout_seconds` is required for MVP clarity
- do not store raw `api_key` in config for the default path

### Task 1.3: Reserve room for profiles

Do not implement profiles in the first pass, but shape the config loader so a later extension can support:

```toml
[profiles.default.llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
```

This keeps future `--profile` support possible without rewriting the loader.

## Phase 2: Secret Loading

### Task 2.1: Load API key via env-var indirection

- Read `llm.api_key_env` from config
- Resolve the actual secret value from that environment variable
- Pass the resolved secret to Gemini client construction
- Keep secret handling out of command handlers

Example:

- config says `api_key_env = "GEMINI_API_KEY"`
- runtime loads `os.Getenv("GEMINI_API_KEY")`

### Task 2.2: Validate secret-related failures

Return clear typed errors for:

- missing config file
- missing `[llm]`
- missing `api_key_env`
- empty `api_key_env`
- referenced environment variable not set
- referenced environment variable set but empty

### Task 2.3: Do not silently fall back to raw config secrets

For the first implementation:

- do not auto-read `llm.api_key`
- do not silently fall back to unrelated env vars
- prefer explicitness over magic

If raw `api_key` support is added later, it should be opt-in and documented as a lower-priority compatibility path.

## Phase 3: Override Precedence

### Task 3.1: Define precedence in one shared loader

Centralize settings resolution so every command gets consistent behavior:

- CLI flags override env/config
- env overrides config where the field is designed to be overridable
- config provides defaults

Do not let each command implement its own precedence rules.

### Task 3.2: Keep secrets and non-secrets separate

Recommended behavior:

- non-secret defaults come from config
- secrets come from env via config indirection
- command flags should not normally carry secrets

This keeps skill prompts cleaner and avoids leaking secrets into shell history.

### Task 3.3: Identify first overridable fields

For MVP, support the precedence model at least for:

- store path
- output format
- LLM model
- timeout

If model/timeout do not yet exist as CLI flags, keep the loader ready for them without requiring command support immediately.

## Phase 4: Skill Integration

### Task 4.1: Document the skill usage pattern

Write guidance for skills to assume:

- config exists at the target store
- skills pass task-specific arguments only
- secrets are injected by environment, not copied into prompts

### Task 4.2: Keep commands skill-friendly

The command line should stay compact enough that a skill can reasonably generate:

```bash
aascribe describe ./file.go --store ./project-mem
aascribe index . --store ./project-mem --depth 2
```

without also needing to pass provider/model/secret material every time.

### Task 4.3: Avoid hidden behavior that surprises skills

- do not silently switch providers
- do not silently discover secret names from many fallback env vars
- do not mutate config during normal command execution

Skills should be able to depend on predictable configuration behavior.

## Phase 5: Documentation

### Task 5.1: Add a user-facing config usage document

- Write a dedicated config usage document under `docs/`
- Explain:
  - where the config file lives
  - how config, env vars, and CLI flags interact
  - how secrets are loaded through `api_key_env`
  - what fields are supported in the first config schema
- Keep the document user-facing, not implementation-internal

Recommended document topics:

- config file location
- precedence order: flags > env > config
- example `config.toml`
- how to export `GEMINI_API_KEY`
- how skills should call the CLI without passing secrets
- common error cases and how to fix them

### Task 5.2: Add a quick-start sample for config setup

- Include a copy-pasteable quick-start section in the config usage doc
- Provide a minimal first-run example such as:

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

- Also provide one skill-friendly example using an explicit store:

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

aascribe describe ./main.go --store ./project-mem
```

### Task 5.3: Link the new config doc from existing docs

- Add links from:
  - `README.md`
  - `quick_start.md`
  - any future LLM setup docs that need config guidance
- Make sure users can discover config setup without reading task or plan documents

## Phase 6: Tests

### Task T1: Config parsing tests

- valid config
- missing `[llm]`
- missing `provider`
- missing `model`
- missing `api_key_env`

### Task T2: Secret resolution tests

- referenced env var exists
- referenced env var missing
- referenced env var empty

### Task T3: Precedence tests

- CLI overrides config
- env overrides config for supported fields
- config applies when no override is present

### Task T4: Skill-oriented behavior tests

- a command can run with config defaults plus env secret only
- a command does not require secret flags
- missing secret yields a clear error message

## Acceptance Criteria

The first acceptable configuration implementation should:

- support durable defaults from `<store>/config.toml`
- load Gemini credentials via `api_key_env`
- keep raw secrets out of normal config by default
- apply one consistent precedence model across commands
- remain simple enough for skills to call without verbose flag lists
- include a user-facing config document with copy-pasteable quick-start examples

## Out Of Scope For First Config Milestone

- full profile support
- multi-provider configuration
- OS keychain integration
- interactive config editing commands
- remote secret managers
