param(
  [string]$Binary = ".\agent.exe",
  [string]$Addr = "127.0.0.1:19090",
  [int]$MemoryRows = 1000,
  [int]$SearchRuns = 20,
  [string]$Out = "docs\performance-baseline.local.json",
  [switch]$SkipAgentStartup
)

$ErrorActionPreference = "Stop"

function Percentile([double[]]$Values, [double]$P) {
  if ($Values.Count -eq 0) { return 0 }
  $sorted = $Values | Sort-Object
  $idx = [Math]::Ceiling(($P / 100.0) * $sorted.Count) - 1
  if ($idx -lt 0) { $idx = 0 }
  if ($idx -ge $sorted.Count) { $idx = $sorted.Count - 1 }
  return [Math]::Round([double]$sorted[$idx], 2)
}

function Convert-MemoryMetric($Bytes) {
  return [Math]::Round(([double]$Bytes / 1MB), 2)
}

if (-not (Test-Path $Binary)) {
  Write-Host "Binary not found at $Binary; building ./cmd/agent..."
  go build -o $Binary ./cmd/agent
}

$repoRoot = (Resolve-Path ".").Path
$dataDir = Join-Path $repoRoot ".perf-data"
$env:DATA_DIR = $dataDir
$env:AGENT_ADDR = $Addr
$env:YUNQUE_PROFILE = "perf-baseline"

if (Test-Path $dataDir) {
  Remove-Item -LiteralPath $dataDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

$startupMs = $null
$peakWorkingSetMB = $null
$agentProc = $null

try {
  if (-not $SkipAgentStartup) {
    $logPath = Join-Path $dataDir "agent.log"
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $agentProc = Start-Process -FilePath (Resolve-Path $Binary).Path -WorkingDirectory $repoRoot -PassThru -WindowStyle Hidden -RedirectStandardOutput $logPath -RedirectStandardError (Join-Path $dataDir "agent.err.log")
    $deadline = (Get-Date).AddSeconds(30)
    do {
      Start-Sleep -Milliseconds 200
      try {
        $resp = Invoke-WebRequest -UseBasicParsing -Uri "http://$Addr/healthz" -TimeoutSec 2
        if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) { break }
      } catch {
        # keep waiting until timeout
      }
      $agentProc.Refresh()
      $peakWorkingSetMB = [Math]::Max([double]($peakWorkingSetMB ?? 0), (Convert-MemoryMetric $agentProc.PeakWorkingSet64))
    } while ((Get-Date) -lt $deadline -and -not $agentProc.HasExited)
    $sw.Stop()
    if ($agentProc.HasExited) {
      throw "Agent exited before /healthz became available. See $logPath"
    }
    $startupMs = [Math]::Round($sw.Elapsed.TotalMilliseconds, 2)
    $agentProc.Refresh()
    $peakWorkingSetMB = [Math]::Max([double]($peakWorkingSetMB ?? 0), (Convert-MemoryMetric $agentProc.PeakWorkingSet64))
  }

  $ledgerOut = Join-Path $dataDir "ledger-bench.json"
  go run ./cmd/perf-baseline -memory-rows $MemoryRows -search-runs $SearchRuns -out $ledgerOut | Write-Host
  $ledger = Get-Content $ledgerOut -Raw | ConvertFrom-Json

  $result = [ordered]@{
    generated_at = (Get-Date).ToString("o")
    host = [ordered]@{
      os = [System.Environment]::OSVersion.VersionString
      cpu_count = [System.Environment]::ProcessorCount
      go_version = (& go version)
    }
    agent = [ordered]@{
      binary = $Binary
      addr = $Addr
      startup_ms = $startupMs
      peak_working_set_mb = $peakWorkingSetMB
    }
    ledger = $ledger
    notes = @(
      "Local baseline numbers depend on hardware, antivirus, filesystem cache, and whether LLM providers are configured.",
      "Use clean DATA_DIR for release comparisons.",
      "Do not commit docs/performance-baseline.local.json unless intentionally publishing a measured baseline."
    )
  }

  $json = $result | ConvertTo-Json -Depth 10
  New-Item -ItemType Directory -Force -Path (Split-Path $Out) | Out-Null
  Set-Content -Path $Out -Value $json -Encoding UTF8
  Write-Host "Performance baseline written to $Out"
} finally {
  if ($agentProc -and -not $agentProc.HasExited) {
    Stop-Process -Id $agentProc.Id -Force
  }
}
