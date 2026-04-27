#!/usr/bin/env zsh
emulate -L zsh
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/install-index-skill.zsh <target-root> [binary-path]

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
script_dir=${0:A:h}
repo_root=${script_dir:h}
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
  print -u2 "error: missing skill source: $skill_source"
  exit 1
fi

if [[ ! -f "$binary_source" ]]; then
  print -u2 "error: missing aascribe binary: $binary_source"
  print -u2 "hint: put the aascribe binary at $default_binary, or pass a binary path explicitly."
  exit 1
fi

install_dir="$target_root/skills/indexing-folder"
bin_dir="$install_dir/bin"
binary_dest="$bin_dir/aascribe"

mkdir -p "$bin_dir"
cp "$skill_source" "$install_dir/SKILL.md"
if [[ "${binary_source:A}" != "${binary_dest:A}" ]]; then
  cp "$binary_source" "$binary_dest"
fi
chmod +x "$binary_dest"

print "Installed aascribe indexing skill:"
print "  $install_dir/SKILL.md"
print "  $binary_dest"
