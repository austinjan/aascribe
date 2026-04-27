<#
.SYNOPSIS
Installs the aascribe indexing skill into a target root.

.DESCRIPTION
Copies skills/indexing-folder/SKILL.md and the bundled aascribe binary into:
  <target-root>/skills/indexing-folder/

If BinaryPath is omitted, the installer looks for:
  skills/indexing-folder/bin/aascribe.exe
  skills/indexing-folder/bin/aascribe
  dist/aascribe_windows_amd64.exe
  dist/aascribe

When the source binary ends in .exe, it is installed as:
  skills/indexing-folder/bin/aascribe.exe

Otherwise, it is installed as:
  skills/indexing-folder/bin/aascribe

.PARAMETER TargetRoot
Directory where the skills/indexing-folder directory should be installed.

.PARAMETER BinaryPath
Optional path to the aascribe binary to bundle with the skill.
#>

param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string] $TargetRoot,

    [Parameter(Mandatory = $false, Position = 1)]
    [string] $BinaryPath = ""
)

$ErrorActionPreference = "Stop"

function Resolve-RepoPath {
    param([string] $Path)

    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $Path
    }

    return Join-Path $RepoRoot $Path
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$DefaultWindowsBinary = Join-Path $RepoRoot "skills/indexing-folder/bin/aascribe.exe"
$DefaultPosixBinary = Join-Path $RepoRoot "skills/indexing-folder/bin/aascribe"
$FallbackWindowsBinary = Join-Path $RepoRoot "dist/aascribe_windows_amd64.exe"
$FallbackPosixBinary = Join-Path $RepoRoot "dist/aascribe"

$SkillSource = Join-Path $RepoRoot "skills/indexing-folder/SKILL.md"
if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
    if (Test-Path -LiteralPath $DefaultWindowsBinary -PathType Leaf) {
        $BinarySource = $DefaultWindowsBinary
    } elseif (Test-Path -LiteralPath $DefaultPosixBinary -PathType Leaf) {
        $BinarySource = $DefaultPosixBinary
    } elseif (Test-Path -LiteralPath $FallbackWindowsBinary -PathType Leaf) {
        $BinarySource = $FallbackWindowsBinary
    } elseif (Test-Path -LiteralPath $FallbackPosixBinary -PathType Leaf) {
        $BinarySource = $FallbackPosixBinary
    } else {
        $BinarySource = $DefaultWindowsBinary
    }
} else {
    $BinarySource = Resolve-RepoPath $BinaryPath
}

if (-not (Test-Path -LiteralPath $SkillSource -PathType Leaf)) {
    throw "missing skill source: $SkillSource"
}

if (-not (Test-Path -LiteralPath $BinarySource -PathType Leaf)) {
    throw "missing aascribe binary: $BinarySource. Put the Windows binary at $DefaultWindowsBinary, the POSIX binary at $DefaultPosixBinary, or pass a binary path explicitly."
}

$InstallDir = Join-Path $TargetRoot "skills/indexing-folder"
$BinDir = Join-Path $InstallDir "bin"
$BinaryName = if ([System.IO.Path]::GetExtension($BinarySource) -eq ".exe") { "aascribe.exe" } else { "aascribe" }
$BinaryDest = Join-Path $BinDir $BinaryName

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
Copy-Item -LiteralPath $SkillSource -Destination (Join-Path $InstallDir "SKILL.md") -Force
if ((Resolve-Path -LiteralPath $BinarySource).Path -ne (Join-Path (Resolve-Path -LiteralPath $BinDir).Path $BinaryName)) {
    Copy-Item -LiteralPath $BinarySource -Destination $BinaryDest -Force
}

Write-Output "Installed aascribe indexing skill:"
Write-Output "  $(Join-Path $InstallDir 'SKILL.md')"
Write-Output "  $BinaryDest"
