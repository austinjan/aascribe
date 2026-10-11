# Run on A. Local credentials are encrypted with Windows DPAPI for this user.
param(
    [string]$ProjectRoot = (Split-Path $PSScriptRoot -Parent),
    [ValidateSet('openai-codex', 'google')][string]$Provider = 'openai-codex',
    [string]$Model,
    [string]$AuthFile
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$state = Join-Path $repo 'data/remote-mcp'
$null = New-Item -ItemType Directory -Force -Path $state
$secretFile = Join-Path $state 'credentials.dpapi'
if (Test-Path $secretFile) {
    $secure = (Get-Content -LiteralPath $secretFile -Raw).Trim() | ConvertTo-SecureString
    $credential = [System.Net.NetworkCredential]::new('', $secure)
    $saved = $credential.Password | ConvertFrom-Json
    $env:AASCRIBE_MCP_TOKEN = $saved.token
} else {
    if (-not $env:AASCRIBE_MCP_TOKEN) {
        $bytes = New-Object byte[] 32
        $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
        try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
        $env:AASCRIBE_MCP_TOKEN = [Convert]::ToBase64String($bytes)
    }
    $saved = @{ token = $env:AASCRIBE_MCP_TOKEN } | ConvertTo-Json -Compress
    $saved | ConvertTo-SecureString -AsPlainText -Force | ConvertFrom-SecureString | Set-Content -LiteralPath $secretFile
}
if (-not $Model) { $Model = if ($Provider -eq 'openai-codex') { 'gpt-6.1-sol' } else { 'gemini-2.5-flash' } }
if ($Provider -eq 'openai-codex') {
    $managedAuth = -not $AuthFile
    if ($managedAuth) { $AuthFile = Join-Path $env:USERPROFILE '.codex/auth.json' }
    $AuthFile = (Resolve-Path -LiteralPath $AuthFile).Path
    # Require a completed OAuth login; never silently fall back to an API key.
    $auth = Get-Content -LiteralPath $AuthFile -Raw | ConvertFrom-Json
    if ($managedAuth) {
        if ($auth.auth_mode -ne 'chatgpt') { throw 'Complete codex login with your ChatGPT subscription first.' }
        $codex = (Get-Command codex -ErrorAction Stop).Source
    } elseif ($auth.'openai-codex'.type -ne 'oauth') { throw 'Complete node runtime/pi/login-codex.mjs first.' }
} elseif (-not $env:GEMINI_API_KEY) { throw 'Set GEMINI_API_KEY for the explicitly selected Google provider.' }
if (Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) {
    throw 'Port 8080 is already listening; inspect the existing process before restarting.'
}
$binary = Join-Path $repo 'dist/aascribe-mcp.exe'
$worker = Join-Path $repo 'runtime/pi/worker.mjs'
$node = (Get-Command node -ErrorAction Stop).Source
$resolvedRoot = (Resolve-Path -LiteralPath $ProjectRoot).Path
foreach ($path in @($binary, $worker, $node, $resolvedRoot, $AuthFile, $Model, $codex)) {
    if ($path -and $path.Contains('"')) { throw 'Paths cannot contain double quotes.' }
}
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$arguments = '--root "{0}" --node "{1}" --pi-worker "{2}" --listen 127.0.0.1:8080 --query-timeout 90s --pi-provider {3} --pi-model "{4}"' -f $resolvedRoot, $node, $worker, $Provider, $Model
if ($Provider -eq 'openai-codex') {
    if ($managedAuth) { $arguments += ' --pi-codex-auth-file "{0}" --codex-command "{1}"' -f $AuthFile, $codex }
    else { $arguments += ' --pi-auth-file "{0}"' -f $AuthFile }
}
$process = Start-Process -FilePath $binary -ArgumentList $arguments -WorkingDirectory $repo -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $state "server-$stamp.stdout.log") -RedirectStandardError (Join-Path $state "server-$stamp.stderr.log")
$process.Id | Set-Content (Join-Path $state 'server.pid')
@{ pid = $process.Id; root = $resolvedRoot; endpoint = 'http://127.0.0.1:8080/mcp'; provider = $Provider; model = $Model; credentials = $secretFile } | ConvertTo-Json
