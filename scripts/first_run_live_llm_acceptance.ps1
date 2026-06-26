param(
    [string]$GoExe = "",
    [string]$WorkRoot = "",
    [string]$BinRoot = "",
    [string]$ConfigRoot = "",
    [string]$CredentialStoreRoot = "",
    [string]$LedgerPath = "",
    [string]$Addr = "127.0.0.1:8765",
    [string]$BaseUrl = "",
    [string]$Model = "",
    [string]$ApiKeyEnv = "GENESIS_PROVIDER_API_KEY",
    [string]$RuntimeToken = "",
    [string]$ModelRole = "foreground.coordinator",
    [string]$ProfileId = "live-acceptance",
    [string]$GatewayRoute = "live-acceptance",
    [string]$CredentialRef = "secret://models/provider/live-acceptance",
    [string]$Prompt = "Reply with exactly: GENESIS_LIVE_LLM_ACCEPTANCE_OK",
    [switch]$SkipFailureProbe,
    [switch]$KeepServer,
    [switch]$Help
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot

if ($Help) {
    @"
Genesis live LLM first-run acceptance

Required:
  -BaseUrl <openai-compatible base url>
  -Model <model id>
  `$env:$ApiKeyEnv=<provider api key>

Example:
  `$env:GENESIS_PROVIDER_API_KEY = "<provider api key>"
  powershell -NoProfile -ExecutionPolicy Bypass -File scripts\first_run_live_llm_acceptance.ps1 -BaseUrl https://provider.example.com/api -Model provider-model

The script builds genesisctl/genesisd, writes Genesis config and a secret:// credential
record, starts genesisd through Genesis config, calls /ready and /turn, inspects
timeline/events/context, restarts the server, replays the same projections, and
optionally checks a missing-credential failure path. It never accepts the raw API key
as a command-line argument.
"@
    exit 0
}

function Resolve-DefaultGoExe {
    if ($GoExe.Trim() -ne "") {
        return $GoExe
    }
    $workspaceGo = "D:\software\Go\bin\go.exe"
    if (Test-Path -LiteralPath $workspaceGo) {
        return $workspaceGo
    }
    return "go"
}

function New-DirectoryIfMissing {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Invoke-Native {
    param(
        [string]$FilePath,
        [string[]]$Arguments,
        [string]$WorkingDirectory = ""
    )

    $previous = Get-Location
    try {
        if ($WorkingDirectory.Trim() -ne "") {
            Set-Location -LiteralPath $WorkingDirectory
        }
        $output = & $FilePath @Arguments 2>&1
        $exitCode = $LASTEXITCODE
        $text = ($output | Out-String).Trim()
        if ($exitCode -ne 0) {
            throw "native command failed with exit code $exitCode`: $FilePath $($Arguments -join ' ')`n$text"
        }
        return $text
    }
    finally {
        Set-Location -LiteralPath $previous
    }
}

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST")]
        [string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [string]$Token = ""
    )

    $headers = @{}
    if ($Token.Trim() -ne "") {
        $headers["Authorization"] = "Bearer $Token"
    }

    if ($Method -eq "POST") {
        $payload = $Body | ConvertTo-Json -Depth 32 -Compress
        return Invoke-RestMethod -Method Post -Uri $Uri -Headers $headers -ContentType "application/json" -Body $payload
    }
    return Invoke-RestMethod -Method Get -Uri $Uri -Headers $headers
}

function Invoke-JsonExpectError {
    param(
        [ValidateSet("POST")]
        [string]$Method,
        [string]$Uri,
        [object]$Body,
        [string]$Token
    )

    $request = [System.Net.HttpWebRequest]::Create($Uri)
    $request.Method = $Method
    $request.ContentType = "application/json"
    if ($Token.Trim() -ne "") {
        $request.Headers["Authorization"] = "Bearer $Token"
    }
    $payload = $Body | ConvertTo-Json -Depth 32 -Compress
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($payload)
    $request.ContentLength = $bytes.Length
    $stream = $request.GetRequestStream()
    try {
        $stream.Write($bytes, 0, $bytes.Length)
    }
    finally {
        $stream.Close()
    }

    try {
        $response = $request.GetResponse()
        $response.Close()
        throw "expected HTTP error but request succeeded"
    }
    catch [System.Net.WebException] {
        $response = $null
        if ($_.Exception.PSObject.Properties.Name -contains "Response") {
            $response = $_.Exception.Response
        }
        if ($null -eq $response) {
            throw
        }
        $reader = New-Object System.IO.StreamReader($response.GetResponseStream())
        $text = $reader.ReadToEnd()
        return @{
            status_code = [int]$response.StatusCode
            body = ($text | ConvertFrom-Json)
        }
    }
}

function Get-StructuredErrorCode {
    param([object]$Body)
    if ($null -eq $Body) {
        return ""
    }
    $properties = $Body.PSObject.Properties.Name
    if ($properties -contains "error") {
        $errorObject = $Body.error
        if ($null -ne $errorObject -and $errorObject.PSObject.Properties.Name -contains "code") {
            return [string]$errorObject.code
        }
    }
    if ($properties -contains "code") {
        return [string]$Body.code
    }
    return ""
}

function ConvertTo-CompactJson {
    param([object]$Value)
    return ($Value | ConvertTo-Json -Depth 32 -Compress)
}

function Wait-GenesisReady {
    param(
        [string]$BaseUri,
        [string]$ExpectedStatus,
        [int]$TimeoutSeconds = 45
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $last = $null
    while ((Get-Date) -lt $deadline) {
        try {
            $ready = Invoke-Json -Method GET -Uri "$BaseUri/ready"
            $last = $ready
            if ($ready.readiness -eq $ExpectedStatus) {
                return $ready
            }
        }
        catch {
            $last = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 500
    }
    throw "genesisd did not report ready status '$ExpectedStatus' before timeout; last=$(ConvertTo-CompactJson $last)"
}

function Quote-ProcessArgument {
    param([string]$Value)
    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    if ($Value.Contains('"')) {
        throw "process argument contains an unsupported quote character"
    }
    return '"' + $Value + '"'
}

function Start-Genesisd {
    param(
        [string]$ExePath,
        [string]$ListenAddr,
        [string]$Ledger,
        [string]$Token,
        [string]$Config,
        [string]$Credentials,
        [string]$Role,
        [string]$Profile,
        [string]$HiddenApiKeyEnv,
        [string]$StdoutPath,
        [string]$StderrPath
    )

    foreach ($path in @($StdoutPath, $StderrPath)) {
        if (Test-Path -LiteralPath $path) {
            Remove-Item -LiteralPath $path
        }
    }
    $arguments = @(
        "-addr", $ListenAddr,
        "-ledger", $Ledger,
        "-runtime-token", $Token,
        "-provider", "genesis-config",
        "-config-root", $Config,
        "-credential-store-root", $Credentials,
        "-model-role", $Role,
        "-model-profile-id", $Profile
    )
    $argumentLine = ($arguments | ForEach-Object { Quote-ProcessArgument $_ }) -join " "
    $envPath = ""
    $hadApiKey = $false
    $apiKeyValueForRestore = ""
    if (-not [string]::IsNullOrWhiteSpace($HiddenApiKeyEnv)) {
        $envPath = "Env:$HiddenApiKeyEnv"
        $hadApiKey = Test-Path -LiteralPath $envPath
        if ($hadApiKey) {
            $apiKeyValueForRestore = (Get-Item -LiteralPath $envPath).Value
        }
    }
    try {
        if ($hadApiKey) {
            Remove-Item -LiteralPath $envPath
        }
        return Start-Process -FilePath $ExePath -ArgumentList $argumentLine -PassThru -WindowStyle Hidden -RedirectStandardOutput $StdoutPath -RedirectStandardError $StderrPath
    }
    finally {
        if ($hadApiKey) {
            Set-Item -LiteralPath $envPath -Value $apiKeyValueForRestore
        }
    }
}

function Stop-Genesisd {
    param([System.Diagnostics.Process]$Process)
    if ($null -eq $Process) {
        return
    }
    try {
        if (-not $Process.HasExited) {
            $Process.CloseMainWindow() | Out-Null
            if (-not $Process.WaitForExit(3000)) {
                $Process.Kill()
                $Process.WaitForExit(5000) | Out-Null
            }
        }
    }
    catch {
        if (-not $Process.HasExited) {
            $Process.Kill()
        }
    }
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$resolvedGo = Resolve-DefaultGoExe

if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    $envBaseUrl = [Environment]::GetEnvironmentVariable("GENESIS_PROVIDER_BASE_URL")
    if (-not [string]::IsNullOrWhiteSpace($envBaseUrl)) {
        $BaseUrl = $envBaseUrl
    }
}
if ([string]::IsNullOrWhiteSpace($Model)) {
    $envModel = [Environment]::GetEnvironmentVariable("GENESIS_PROVIDER_MODEL")
    if (-not [string]::IsNullOrWhiteSpace($envModel)) {
        $Model = $envModel
    }
}
if ([string]::IsNullOrWhiteSpace($ApiKeyEnv)) {
    $ApiKeyEnv = "GENESIS_PROVIDER_API_KEY"
}

$apiKeyValue = [Environment]::GetEnvironmentVariable($ApiKeyEnv)
if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    throw "-BaseUrl or GENESIS_PROVIDER_BASE_URL is required"
}
if ([string]::IsNullOrWhiteSpace($Model)) {
    throw "-Model or GENESIS_PROVIDER_MODEL is required"
}
if ([string]::IsNullOrWhiteSpace($apiKeyValue)) {
    throw "environment variable $ApiKeyEnv is required and must contain the provider API key"
}

if ($WorkRoot.Trim() -eq "") {
    $WorkRoot = Join-Path $ProjectRoot (Join-Path ".genesis-live" ("acceptance-" + [guid]::NewGuid().ToString("N").Substring(0, 12)))
}
New-DirectoryIfMissing -Path $WorkRoot

if ($BinRoot.Trim() -eq "") {
    $BinRoot = Join-Path $WorkRoot "bin"
}
if ($ConfigRoot.Trim() -eq "") {
    $ConfigRoot = Join-Path $WorkRoot "config"
}
if ($CredentialStoreRoot.Trim() -eq "") {
    $CredentialStoreRoot = Join-Path $WorkRoot "credentials"
}
if ($LedgerPath.Trim() -eq "") {
    $LedgerPath = Join-Path $WorkRoot "events.jsonl"
}
if ($RuntimeToken.Trim() -eq "") {
    $RuntimeToken = "local-live-acceptance-" + [guid]::NewGuid().ToString("N")
}

New-DirectoryIfMissing -Path $BinRoot
New-DirectoryIfMissing -Path $ConfigRoot
New-DirectoryIfMissing -Path $CredentialStoreRoot

$genesisdExe = Join-Path $BinRoot "genesisd.exe"
$genesisctlExe = Join-Path $BinRoot "genesisctl.exe"
$baseUri = "http://$Addr"
$server = $null
$healthyStdout = Join-Path $WorkRoot "genesisd.healthy.stdout.log"
$healthyStderr = Join-Path $WorkRoot "genesisd.healthy.stderr.log"
$failureStdout = Join-Path $WorkRoot "genesisd.failure.stdout.log"
$failureStderr = Join-Path $WorkRoot "genesisd.failure.stderr.log"

try {
    Invoke-Native -FilePath $resolvedGo -Arguments @("build", "-o", $genesisdExe, ".\cmd\genesisd") -WorkingDirectory $repoRoot | Out-Null
    Invoke-Native -FilePath $resolvedGo -Arguments @("build", "-o", $genesisctlExe, ".\cmd\genesisctl") -WorkingDirectory $repoRoot | Out-Null

    $setupOutput = Invoke-Native -FilePath $genesisctlExe -Arguments @(
        "provider-setup",
        "-config-root", $ConfigRoot,
        "-credential-store-root", $CredentialStoreRoot,
        "-model-role", $ModelRole,
        "-profile-id", $ProfileId,
        "-gateway-route", $GatewayRoute,
        "-base-url", $BaseUrl,
        "-model", $Model,
        "-credential-ref", $CredentialRef,
        "-api-key-env", $ApiKeyEnv
    )
    if ($setupOutput.Contains($apiKeyValue)) {
        throw "provider setup output leaked the raw API key"
    }
    $setup = $setupOutput | ConvertFrom-Json
    if (-not $setup.ok -or -not $setup.verified) {
        throw "provider setup did not report ok+verified: $(ConvertTo-CompactJson $setup)"
    }

    $server = Start-Genesisd -ExePath $genesisdExe -ListenAddr $Addr -Ledger $LedgerPath -Token $RuntimeToken -Config $ConfigRoot -Credentials $CredentialStoreRoot -Role $ModelRole -Profile $ProfileId -HiddenApiKeyEnv $ApiKeyEnv -StdoutPath $healthyStdout -StderrPath $healthyStderr
    $ready = Wait-GenesisReady -BaseUri $baseUri -ExpectedStatus "ready"
    if ($ready.provider.name -eq "fake" -or $ready.provider.readiness -ne "ready") {
        throw "ready provider is not a configured live provider: $(ConvertTo-CompactJson $ready.provider)"
    }

    $sessionId = "live-first-run-" + [guid]::NewGuid().ToString("N").Substring(0, 12)
    $turn = Invoke-Json -Method POST -Uri "$baseUri/turn" -Token $RuntimeToken -Body @{
        session_id = $sessionId
        idempotency_key = "first-live-turn"
        input_items = @(@{
            type = "text"
            text = $Prompt
        })
    }
    if ([string]::IsNullOrWhiteSpace($turn.turn_id)) {
        throw "turn response did not include turn_id: $(ConvertTo-CompactJson $turn)"
    }
    if ([string]::IsNullOrWhiteSpace($turn.final.text)) {
        throw "turn response did not include non-empty final text: $(ConvertTo-CompactJson $turn)"
    }
    if ($turn.final.model -eq "fake-model" -or $turn.final.text.TrimStart().StartsWith("fake:")) {
        throw "turn response came from the fake provider, not the configured live provider"
    }

    $timeline = Invoke-Json -Method GET -Uri "$baseUri/sessions/$sessionId/timeline" -Token $RuntimeToken
    if ($timeline.status -ne "ok" -or $timeline.items.Count -lt 2) {
        throw "timeline projection is not usable: $(ConvertTo-CompactJson $timeline)"
    }
    $events = Invoke-Json -Method GET -Uri "$baseUri/turns/$($turn.turn_id)/events" -Token $RuntimeToken
    if ($events.items.Count -lt 2) {
        throw "turn event replay is not usable: $(ConvertTo-CompactJson $events)"
    }
    $context = Invoke-Json -Method GET -Uri "$baseUri/turns/$($turn.turn_id)/context" -Token $RuntimeToken
    if ($context.status -ne "ok") {
        throw "turn context inspection is not usable: $(ConvertTo-CompactJson $context)"
    }
    $session = Invoke-Json -Method GET -Uri "$baseUri/sessions/$sessionId" -Token $RuntimeToken
    if ($session.turns.Count -lt 1 -or $session.events.Count -lt 2) {
        throw "session projection is not usable: $(ConvertTo-CompactJson $session)"
    }

    Stop-Genesisd -Process $server
    $server = $null

    $server = Start-Genesisd -ExePath $genesisdExe -ListenAddr $Addr -Ledger $LedgerPath -Token $RuntimeToken -Config $ConfigRoot -Credentials $CredentialStoreRoot -Role $ModelRole -Profile $ProfileId -HiddenApiKeyEnv $ApiKeyEnv -StdoutPath $healthyStdout -StderrPath $healthyStderr
    Wait-GenesisReady -BaseUri $baseUri -ExpectedStatus "ready" | Out-Null
    $replayedTimeline = Invoke-Json -Method GET -Uri "$baseUri/sessions/$sessionId/timeline" -Token $RuntimeToken
    $replayedEvents = Invoke-Json -Method GET -Uri "$baseUri/turns/$($turn.turn_id)/events" -Token $RuntimeToken
    $replayedContext = Invoke-Json -Method GET -Uri "$baseUri/turns/$($turn.turn_id)/context" -Token $RuntimeToken
    if ($replayedTimeline.items.Count -lt $timeline.items.Count -or $replayedEvents.items.Count -lt $events.items.Count -or $replayedContext.status -ne "ok") {
        throw "restart replay lost projection data"
    }

    $failureProbe = $null
    if (-not $SkipFailureProbe) {
        Stop-Genesisd -Process $server
        $server = $null
        $brokenCredentialRoot = Join-Path $WorkRoot "missing-credentials"
        New-DirectoryIfMissing -Path $brokenCredentialRoot
        $server = Start-Genesisd -ExePath $genesisdExe -ListenAddr $Addr -Ledger (Join-Path $WorkRoot "failure-events.jsonl") -Token $RuntimeToken -Config $ConfigRoot -Credentials $brokenCredentialRoot -Role $ModelRole -Profile $ProfileId -HiddenApiKeyEnv $ApiKeyEnv -StdoutPath $failureStdout -StderrPath $failureStderr
        $blocked = Wait-GenesisReady -BaseUri $baseUri -ExpectedStatus "not_ready"
        if ($blocked.provider.readiness -ne "not_ready" -or $blocked.provider.readiness_reason -ne "provider_credential_missing") {
            throw "failure probe did not report provider_credential_missing: $(ConvertTo-CompactJson $blocked)"
        }
        $turnError = Invoke-JsonExpectError -Method POST -Uri "$baseUri/turn" -Token $RuntimeToken -Body @{
            session_id = "live-first-run-failure-probe"
            input_items = @(@{ type = "text"; text = "This turn should be rejected by provider readiness." })
        }
        $turnErrorCode = Get-StructuredErrorCode -Body $turnError.body
        if ($turnError.status_code -ne 503 -or $turnErrorCode -ne "provider_unavailable") {
            throw "failure turn did not report provider_unavailable: $(ConvertTo-CompactJson $turnError)"
        }
        $failureProbe = @{
            ready_status = $blocked.readiness
            provider_status = $blocked.provider.readiness
            provider_reason = $blocked.provider.readiness_reason
            turn_status_code = $turnError.status_code
            turn_error_code = $turnErrorCode
        }
        if ($KeepServer) {
            Stop-Genesisd -Process $server
            $server = Start-Genesisd -ExePath $genesisdExe -ListenAddr $Addr -Ledger $LedgerPath -Token $RuntimeToken -Config $ConfigRoot -Credentials $CredentialStoreRoot -Role $ModelRole -Profile $ProfileId -HiddenApiKeyEnv $ApiKeyEnv -StdoutPath $healthyStdout -StderrPath $healthyStderr
            Wait-GenesisReady -BaseUri $baseUri -ExpectedStatus "ready" | Out-Null
        }
    }

    if (-not $KeepServer) {
        Stop-Genesisd -Process $server
        $server = $null
    }

    $summary = @{
        ok = $true
        work_root = $WorkRoot
        base_uri = $baseUri
        config_path = $setup.config_path
        credential_ref = $setup.credential_ref
        ledger_path = $LedgerPath
        session_id = $sessionId
        turn_id = $turn.turn_id
        provider_model = $turn.final.model
        final_text = $turn.final.text
        ready = @{
            status = $ready.readiness
            provider = $ready.provider.readiness
            runtime_auth = $ready.runtime_auth.readiness
            ledger = $ready.ledger.readiness
        }
        inspected = @{
            timeline_items = $timeline.items.Count
            event_items = $events.items.Count
            context_status = $context.status
        }
        restart_replay = @{
            timeline_items = $replayedTimeline.items.Count
            event_items = $replayedEvents.items.Count
            context_status = $replayedContext.status
        }
        failure_probe = $failureProbe
        keep_server = [bool]$KeepServer
    }
    $summary | ConvertTo-Json -Depth 32
}
finally {
    if (-not $KeepServer) {
        Stop-Genesisd -Process $server
    }
}
