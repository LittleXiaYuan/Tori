param(
    [string]$BaseUrl = "http://localhost:9090",
    [string]$ApiKey = $env:YUNQUE_API_KEY,
    [string]$SessionId = "demo-cogni-review",
    [switch]$SkipChat
)

if (-not $ApiKey) {
    Write-Error "Missing API key. Set `$env:YUNQUE_API_KEY or pass -ApiKey."
    exit 1
}

$Headers = @{
    "X-API-Key" = $ApiKey
}

$script:Failed = 0

function Assert-Demo {
    param(
        [string]$Name,
        [bool]$Condition,
        [string]$Detail = ""
    )

    if ($Condition) {
        Write-Host "[PASS] $Name" -ForegroundColor Green
        return
    }

    $script:Failed++
    if ($Detail) {
        Write-Host "[FAIL] $Name - $Detail" -ForegroundColor Red
    } else {
        Write-Host "[FAIL] $Name" -ForegroundColor Red
    }
}

function Invoke-YunqueJson {
    param(
        [string]$Method,
        [string]$Path,
        [object]$Body = $null
    )

    $uri = "$BaseUrl$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers
    }

    return Invoke-RestMethod `
        -Method $Method `
        -Uri $uri `
        -Headers $Headers `
        -ContentType "application/json" `
        -Body ($Body | ConvertTo-Json -Depth 20)
}

Write-Host "== 1. Reload Cogni declarations =="
$reload = Invoke-YunqueJson -Method Post -Path "/v1/cognis/reload"
$reload | ConvertTo-Json -Depth 8
Assert-Demo "reload completed" ($null -ne $reload.version)
Assert-Demo "reload has no errors" (($null -eq $reload.errors) -or ($reload.errors.Count -eq 0))

Write-Host "`n== 2. List Cognis =="
$list = Invoke-YunqueJson -Method Get -Path "/v1/cognis"
$codeReviewer = $list.cognis | Where-Object { $_.id -eq "code-reviewer" } | Select-Object -First 1
$codeReviewer | ConvertTo-Json -Depth 8
Assert-Demo "code-reviewer is registered" ($null -ne $codeReviewer)
Assert-Demo "code-reviewer is enabled" (($null -ne $codeReviewer) -and $codeReviewer.enabled)

Write-Host "`n== 3. Verify code-reviewer checks =="
$verify = Invoke-YunqueJson -Method Post -Path "/v1/cognis/code-reviewer/verify"
$verify | ConvertTo-Json -Depth 12
Assert-Demo "verify endpoint returned results" (($null -ne $verify.results) -and ($verify.results.Count -gt 0))
Assert-Demo "all declaration checks passed" ($verify.failed -eq 0) "failed=$($verify.failed)"

if (-not $SkipChat) {
    Write-Host "`n== 4. Send chat message that should activate code-reviewer =="
    $chatBody = @{
        session_id = $SessionId
        messages = @(
            @{
                role = "user"
                content = "帮我审查一下这个 PR 的代码，重点看安全和可维护性"
            }
        )
    }
    $chat = Invoke-YunqueJson -Method Post -Path "/v1/chat" -Body $chatBody
    if ($chat.reply) {
        $chat.reply
    } else {
        $chat | ConvertTo-Json -Depth 8
    }
    Assert-Demo "chat returned a reply" (-not [string]::IsNullOrWhiteSpace($chat.reply))
} else {
    Write-Host "`n== 4. Chat skipped by -SkipChat =="
}

Write-Host "`n== 5. Trace activation/context/tool filtering =="
$trace = Invoke-YunqueJson -Method Get -Path "/v1/cognis/code-reviewer/trace?limit=5"
$trace | ConvertTo-Json -Depth 20
if (-not $SkipChat) {
    $latestTrace = $trace.traces | Select-Object -First 1
    $activation = $null
    if ($latestTrace -and $latestTrace.activations) {
        $activation = $latestTrace.activations | Where-Object { $_.id -eq "code-reviewer" } | Select-Object -First 1
    }
    Assert-Demo "trace recorded code-reviewer" ($null -ne $activation)
    Assert-Demo "trace shows code-reviewer activated" (($null -ne $activation) -and $activation.activated)
    Assert-Demo "context was injected" (($null -ne $latestTrace) -and ($latestTrace.context.bytes -gt 0))
    Assert-Demo "tool surface filter ran" (($null -ne $latestTrace) -and ($null -ne $latestTrace.tool_filter))
}

Write-Host "`n== 6. Health =="
$health = Invoke-YunqueJson -Method Get -Path "/v1/cognis/code-reviewer/health"
$health | ConvertTo-Json -Depth 12
if (-not $SkipChat) {
    Assert-Demo "health has evaluations" ($health.evaluations -gt 0)
    Assert-Demo "health is not unhealthy" ($health.status -ne "unhealthy") "status=$($health.status)"
}

if ($script:Failed -gt 0) {
    Write-Error "$script:Failed demo assertion(s) failed."
    exit 1
}

Write-Host "`nCogni demo checks passed." -ForegroundColor Green
