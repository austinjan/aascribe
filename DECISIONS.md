# DECISIONS.md

Durable architecture and product decision log.

## 2026-06-30: Use Root State Files Plus Topic Instructions As The Agent Harness

Decision: Establish `AGENTS.md`, `PROGRESS.md`, `CONSTRAINS.md`, `ARCHITECTURE.md`, `DECISIONS.md`, and `instructions/` as the cross-session harness for this repository.

Why: The repo has existing product docs and implementation, but future agents need a small entry router, one progress source of truth, explicit hard constraints, an architecture map, and durable decision history to prevent context loss.

Rejected alternatives:

- Keep only the existing `AGENT.md`: rejected because it does not provide state tracking, architecture ownership, validation logs, or topic-based workflow instructions.
- Put all instructions in one large file: rejected because long monolithic agent docs are less likely to be read and maintained.

Consequences:

- Future sessions start with `AGENTS.md`, then route to state files and topic instructions.
- Work that changes contracts, architecture, decisions, or validation state must update the matching state file.

## 2026-06-30: Treat docs/USAGE.md As The Public CLI Contract

Decision: `docs/USAGE.md` remains the primary source of truth for command names, flags, output envelopes, exit codes, and examples.

Why: Existing guidance and docs already identify it as the baseline behavior contract, and implementation tests exercise behavior against that surface.

Rejected alternatives:

- Treat source code as the only source of truth: rejected because the repository is intentionally documentation-led for CLI behavior.
- Move the command contract into `AGENTS.md`: rejected because agent instructions should route to the contract, not duplicate it.

Consequences:

- Any user-visible CLI change must update `docs/USAGE.md`.
- Tests should cover behavior that is promised in `docs/USAGE.md`.

## 2026-06-30: Keep Output Shapes As Checked-In Schema Contracts

Decision: Public `OutputShape` data payloads are documented in `docs/USAGE.md` and represented by JSON Schema files under `docs/shapes/`.

Why: Agent and tool callers need stable machine-readable data contracts, and the repo already contains a schema directory with explicit policy.

Rejected alternatives:

- Let Go structs implicitly define the contract: rejected because downstream agents need inspectable docs and schemas.
- Rewrite existing schemas in place for breaking changes: rejected because silent contract changes would break callers.

Consequences:

- New public shapes need matching schema files.
- Backward-incompatible shape changes should use a new shape name or an explicitly approved migration.

## 2026-06-30: Keep Secrets Environment-Based

Decision: LLM secrets are loaded from environment variables named by config, not stored directly in config files or passed through normal command flags.

Why: This keeps secrets out of source, shell history, logs, and agent prompts while preserving skill-friendly configuration.

Rejected alternatives:

- Store raw API keys in `config.toml`: rejected because checked-in examples and generated files could leak credentials.
- Require every command to pass a secret flag: rejected because it is noisy and unsafe for shell history.

Consequences:

- Missing secrets should produce clear recovery guidance.
- Logs and errors must never expose secret values.

## 2026-06-30: Install Skills Through The CLI Using Managed Target Directories

Decision: `aascribe install skills` installs the bundled `indexing-folder` skill to either `~/.codex/skills/indexing-folder/` with `--user` or `<folder>/skills/indexing-folder/` with `-f/--folder`. `aascribe uninstall skills` removes that same managed skill directory.

Why: Users need a direct CLI path for installing and removing the bundled skill without calling repo-local scripts, and uninstall must have a narrow boundary that clears all related skill files without touching unrelated skill roots.

Rejected alternatives:

- Install into multiple implicit scopes at once: rejected because destructive uninstall would become ambiguous.
- Remove the entire target `skills/` directory: rejected because other user or project skills may live beside `indexing-folder`.
- Depend only on repo-local install scripts: rejected because installed binaries should be able to install their own bundled skill asset.

Consequences:

- The skill markdown is embedded into the CLI for install portability.
- The installer copies the current `aascribe` executable into the skill `bin/` directory.
- Future bundled skill changes must keep the embedded asset and source skill content aligned.
