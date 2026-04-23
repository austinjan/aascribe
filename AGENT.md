# AGENT.md

Guidance for coding agents working in this repository.

## Project Snapshot

- Project name: `aascribe`
- Current state: early-stage repository with product/CLI behavior documented before implementation
- Primary source of truth: [docs/USAGE.md](/Users/macmini-au/code/aascribe/docs/USAGE.md)

## What To Read First

1. Read [docs/USAGE.md](/Users/macmini-au/code/aascribe/docs/USAGE.md) before making changes.
2. Treat the command contract, flags, JSON envelope, and exit codes in that file as the baseline behavior.
3. If implementation files are added later, preserve compatibility with the documented CLI unless the user explicitly requests a spec change.

## Working Norms

- Prefer small, focused changes.
- Keep docs and implementation in sync.
- If behavior is ambiguous, follow `docs/USAGE.md` first and note any assumptions in your final handoff.
- Do not invent undocumented commands or flags unless the task is explicitly to extend the spec.
- Preserve machine-friendly output behavior for CLI responses, especially JSON shapes and exit codes.

## Documentation Expectations

- Update [docs/USAGE.md](/Users/macmini-au/code/aascribe/docs/USAGE.md) when changing user-visible CLI behavior.
- Add examples when introducing new commands or flags.
- Keep examples copy-pasteable.

## Implementation Preferences

- Favor clear command boundaries that map directly to the documented subcommands:
  - `init`
  - `index`
  - `describe`
  - `remember`
  - `consolidate`
  - `recall`
  - `list`
  - `show`
  - `forget`
- Keep output formatting separated from core command logic where possible.
- Design for both human-readable and agent-friendly operation.

## Current Repository Reality

- `README.md` is currently empty.
- `docs/USAGE.md` is the only substantive project document at the moment.
- If you add scaffolding, keep it minimal and consistent with the documented CLI surface.
