# Installing aascribe And Skills

This guide explains how to install the `aascribe` CLI and the bundled `indexing-folder` skill.

## Build Or Provide The Binary

From the repository root, build the current-platform binary:

```bash
make
```

This writes:

```text
dist/aascribe
```

To build release binaries for supported targets:

```bash
make release
```

Release output is written under `dist/`, for example:

```text
dist/aascribe_windows_amd64.exe
dist/aascribe_linux_amd64
dist/aascribe_macos_arm64
```

## Verify The CLI

Run:

```bash
./dist/aascribe --version
./dist/aascribe --help
```

Initialize the default store for the current project:

```bash
./dist/aascribe init
```

The default store path is:

```text
./data/memory
```

`AASCRIBE_STORE` or `--store <path>` can override that default.

## Skill Layout

The repository includes an agent-facing indexing skill at:

```text
skills/indexing-folder/SKILL.md
```

The skill expects its bundled binary at:

```text
skills/indexing-folder/bin/aascribe
```

On Windows, use:

```text
skills/indexing-folder/bin/aascribe.exe
```

## Install The indexing-folder Skill

The repository provides three install scripts:

```text
scripts/install-index-skill.sh
scripts/install-index-skill.zsh
scripts/install-index-skill.ps1
```

The install scripts copy the skill into a target root at:

```text
<target-root>/skills/indexing-folder/SKILL.md
<target-root>/skills/indexing-folder/bin/aascribe
```

On Windows, a `.exe` binary is installed as:

```text
<target-root>/skills/indexing-folder/bin/aascribe.exe
```

### macOS/Linux With bash

The script uses `skills/indexing-folder/bin/aascribe` when present, otherwise `dist/aascribe` when present:

```bash
scripts/install-index-skill.sh <target-root>
```

To install from a specific built binary:

```bash
scripts/install-index-skill.sh <target-root> dist/aascribe
```

### macOS/Linux With zsh

The zsh installer uses the same default lookup as the bash installer:

```zsh
scripts/install-index-skill.zsh <target-root>
```

You can still pass a binary path explicitly:

```zsh
scripts/install-index-skill.zsh <target-root> dist/aascribe
```

### Windows With PowerShell

The PowerShell installer checks `skills/indexing-folder/bin/aascribe.exe`, `skills/indexing-folder/bin/aascribe`, then release outputs under `dist/`:

```powershell
.\scripts\install-index-skill.ps1 <target-root>
```

To install from a specific release binary:

```powershell
.\scripts\install-index-skill.ps1 <target-root> .\dist\aascribe_windows_amd64.exe
```

## Missing Binary Behavior

If no binary path is passed, the scripts check the skill-local binary location first, then common build outputs under `dist/`.

For bash/zsh:

```text
skills/indexing-folder/bin/aascribe
dist/aascribe
```

For PowerShell:

```text
skills/indexing-folder/bin/aascribe.exe
skills/indexing-folder/bin/aascribe
dist/aascribe_windows_amd64.exe
dist/aascribe
```

If the binary is missing, the script prints an error telling you where to place it, or to pass the binary path explicitly.

## Verify The Installed Skill

After installation, check:

```bash
<target-root>/skills/indexing-folder/bin/aascribe --version
```

Then use the skill's normal workflow from the target repository:

```bash
AASCRIBE=./skills/indexing-folder/bin/aascribe
"$AASCRIBE" index . --depth 2 --no-summary
"$AASCRIBE" map .
```

On Windows PowerShell:

```powershell
$AASCRIBE = ".\skills\indexing-folder\bin\aascribe.exe"
& $AASCRIBE index . --depth 2 --no-summary
& $AASCRIBE map .
```

## Related Docs

- [Usage Guide](USAGE.md)
- [Configuration](configuration.md)
- [Logging](logging.md)
- [Indexing Skill](../skills/indexing-folder/SKILL.md)
