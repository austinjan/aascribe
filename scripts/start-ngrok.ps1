# A's free account-assigned ngrok domain stays fixed across restarts and IP changes.
param([Uri]$PublicUrl)
$ErrorActionPreference = 'Stop'
if ($PublicUrl -and ($PublicUrl.Scheme -ne 'https' -or $PublicUrl.AbsolutePath -ne '/' -or $PublicUrl.Query -or $PublicUrl.Fragment)) {
    throw 'PublicUrl must be an HTTPS origin with no path, query or fragment.'
}
$repo = Split-Path $PSScriptRoot -Parent
$state = Join-Path $repo 'data/remote-mcp'
$secret = Join-Path $state 'ngrok-token.dpapi'
if (-not $env:NGROK_AUTHTOKEN -and (Test-Path -LiteralPath $secret)) {
    $secure = (Get-Content -LiteralPath $secret -Raw).Trim() | ConvertTo-SecureString
    $env:NGROK_AUTHTOKEN = ([System.Net.NetworkCredential]::new('', $secure)).Password
}
if (-not $env:NGROK_AUTHTOKEN) { throw 'Set NGROK_AUTHTOKEN locally from your ngrok account; do not paste it into chat.' }
if (-not (Get-NetTCPConnection -LocalPort 8080 -LocalAddress 127.0.0.1 -State Listen -ErrorAction SilentlyContinue)) {
    throw 'Start the local MCP server first.'
}
if (Test-Path (Join-Path $state 'ngrok.pid')) {
    $previous = Get-Process -Id ([int](Get-Content (Join-Path $state 'ngrok.pid'))) -ErrorAction SilentlyContinue
    if ($previous -and $previous.Path -eq (Join-Path $state 'ngrok/ngrok.exe')) { throw 'The managed ngrok process is already running.' }
}
$binary = Join-Path $state 'ngrok/ngrok.exe'
if (-not (Test-Path -LiteralPath $binary)) { throw 'Install the official ngrok binary at data/remote-mcp/ngrok/ngrok.exe.' }
$env:NGROK_AUTHTOKEN | ConvertTo-SecureString -AsPlainText -Force | ConvertFrom-SecureString | Set-Content -LiteralPath $secret
$config = Join-Path $state 'ngrok.yml'
if (-not (Test-Path $config)) { "version: '3'`nagent:`n  web_addr: false`n" | Set-Content -LiteralPath $config }
$arguments = 'http http://127.0.0.1:8080 --host-header=localhost:8080 --inspect=false --log=stdout --log-format=json --config "{0}"' -f $config
if ($PublicUrl) { $arguments += ' --url "{0}"' -f $PublicUrl.GetLeftPart([UriPartial]::Authority) }
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$process = Start-Process -FilePath $binary -ArgumentList $arguments -WorkingDirectory $repo -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $state "ngrok-$stamp.stdout.log") -RedirectStandardError (Join-Path $state "ngrok-$stamp.stderr.log")
$process.Id | Set-Content (Join-Path $state 'ngrok.pid')
@{ pid = $process.Id; requested_url = [string]$PublicUrl; status = 'starting'; log = (Join-Path $state "ngrok-$stamp.stdout.log") } | ConvertTo-Json
