# A small MCP Streamable HTTP client for local and Tunnel smoke tests.
param(
    [Parameter(Mandatory = $true)][Uri]$Url,
    [string]$Question,
    [ValidateRange(1, 360)][int]$TimeoutSeconds = 110,
    [switch]$IndexFirst,
    [switch]$IndexOnly,
    [switch]$Refresh
)
$ErrorActionPreference = 'Stop'
if ($IndexOnly) { $IndexFirst = $true }
if (-not $IndexOnly -and [string]::IsNullOrWhiteSpace($Question)) { throw 'Supply Question or IndexOnly.' }
if ($Url.Scheme -ne 'https' -and -not ($Url.Scheme -eq 'http' -and $Url.IsLoopback)) {
    throw 'Use HTTPS for remote servers; HTTP is allowed only on loopback.'
}
if (-not $env:AASCRIBE_MCP_TOKEN) { throw 'Set AASCRIBE_MCP_TOKEN in this client environment.' }
$headers = @{
    Authorization = "Bearer $env:AASCRIBE_MCP_TOKEN"
    Accept = 'application/json, text/event-stream'
}
function Invoke-McpMessage($Message) {
    $json = $Message | ConvertTo-Json -Depth 20 -Compress
    $response = Invoke-RestMethod -Uri $Url -Method Post -Headers $headers -ContentType 'application/json; charset=utf-8' -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $TimeoutSeconds
    if ($response.error) { throw "MCP error: $($response.error.message)" }
    return $response
}
$initialize = Invoke-McpMessage @{
    jsonrpc = '2.0'; id = 1; method = 'initialize'
    params = @{ protocolVersion = '2025-11-25'; capabilities = @{}; clientInfo = @{ name = 'aascribe-smoke'; version = '0.1.0' } }
}
$headers['MCP-Protocol-Version'] = $initialize.result.protocolVersion
$null = Invoke-McpMessage @{ jsonrpc = '2.0'; method = 'notifications/initialized' }
$tools = Invoke-McpMessage @{ jsonrpc = '2.0'; id = 2; method = 'tools/list'; params = @{} }
if ('ask_project' -notin $tools.result.tools.name) { throw 'Server did not advertise ask_project.' }
if ($IndexFirst) {
    if ('index_project' -notin $tools.result.tools.name) { throw 'Server did not advertise index_project.' }
    $indexed = Invoke-McpMessage @{
        jsonrpc = '2.0'; id = 4; method = 'tools/call'
        params = @{ name = 'index_project'; arguments = @{ refresh = [bool]$Refresh } }
    }
    if ($indexed.result.isError) { throw "index_project failed: $($indexed.result.content.text -join ' ')" }
    $indexed.result.structuredContent | ConvertTo-Json -Depth 20
}
if ($IndexOnly) { return }
$result = Invoke-McpMessage @{
    jsonrpc = '2.0'; id = 3; method = 'tools/call'
    params = @{ name = 'ask_project'; arguments = @{ question = $Question } }
}
if ($result.result.isError) { throw "ask_project failed: $($result.result.content.text -join ' ')" }
$result.result.structuredContent | ConvertTo-Json -Depth 20
