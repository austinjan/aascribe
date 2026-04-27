#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/install-index-skill.sh <target-root> [binary-path]

Installs the aascribe indexing skill into:
  <target-root>/skills/indexing-folder/

Copies:
  skills/indexing-folder/SKILL.md
  <binary-path> -> skills/indexing-folder/bin/aascribe

If binary-path is omitted, the script checks:
  skills/indexing-folder/bin/aascribe
  dist/aascribe
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -lt 1 || $# -gt 2 ]]; then
  usage >&2
  exit 2
fi

target_root=$1
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
default_binary="$repo_root/skills/indexing-folder/bin/aascribe"
fallback_binary="$repo_root/dist/aascribe"
if [[ $# -eq 2 ]]; then
  binary_path=$2
elif [[ -f "$default_binary" ]]; then
  binary_path=$default_binary
elif [[ -f "$fallback_binary" ]]; then
  binary_path=$fallback_binary
else
  binary_path=$default_binary
fi

skill_source="$repo_root/skills/indexing-folder/SKILL.md"
binary_source=$binary_path
if [[ "$binary_source" != /* ]]; then
  binary_source="$repo_root/$binary_source"
fi

if [[ ! -f "$skill_source" ]]; then
  echo "error: missing skill source: $skill_source" >&2
  exit 1
fi

if [[ ! -f "$binary_source" ]]; then
  echo "error: missing aascribe binary: $binary_source" >&2
  echo "hint: put the aascribe binary at $default_binary, or pass a binary path explicitly." >&2
  exit 1
fi

install_dir="$target_root/skills/indexing-folder"
bin_dir="$install_dir/bin"
binary_dest="$bin_dir/aascribe"

mkdir -p "$bin_dir"
cp "$skill_source" "$install_dir/SKILL.md"
if [[ "$(cd "$(dirname "$binary_source")" && pwd)/$(basename "$binary_source")" != "$(cd "$bin_dir" && pwd)/aascribe" ]]; then
  cp "$binary_source" "$binary_dest"
fi
chmod +x "$binary_dest"

echo "Installed aascribe indexing skill:"
echo "  $install_dir/SKILL.md"
echo "  $binary_dest"
