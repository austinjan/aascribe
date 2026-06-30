# Deployment Instructions

Use this file when building binaries, preparing release artifacts, changing install scripts, or updating the bundled indexing skill.

## Build Commands

Current-platform build:

```bash
make
```

Release build:

```bash
make release
```

Clean generated artifacts:

```bash
make clean
```

## Build Outputs

- `make` writes `dist/aascribe`.
- `make release` writes platform binaries under `dist/`.
- `dist/` is generated output and MUST NOT be treated as source.

## Release Verification

After building, verify the binary:

```bash
./dist/aascribe --version
./dist/aascribe --help
```

When command behavior is affected, also run representative commands with a temporary store:

```bash
tmpdir="$(mktemp -d)"
./dist/aascribe --store "$tmpdir/memory" init
./dist/aascribe --store "$tmpdir/memory" --format json logs path
```

## Bundled Skill Installation

The bundled skill lives at `skills/indexing-folder/SKILL.md`.
The CLI installer embeds a copy at `internal/skill/assets/indexing-folder/SKILL.md`; keep the two files aligned whenever the skill changes.

CLI install commands:

```bash
aascribe install skills --user
aascribe install skills -f <folder>
aascribe uninstall skills --user
aascribe uninstall skills -f <folder>
```

Install scripts:

- `scripts/install-index-skill.sh`
- `scripts/install-index-skill.zsh`
- `scripts/install-index-skill.ps1`

When changing install behavior, update `docs/installation.md`, `docs/USAGE.md`, and any affected shape schemas. Test the CLI installer and at least the shell script relevant to the current platform when script behavior is also affected.

## Secret Handling

Release docs and examples MUST use placeholder values such as `your-real-key`.
Release artifacts and installed skills MUST NOT embed real API keys or tokens.
