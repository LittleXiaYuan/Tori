# Runs the focused cross-language Agent Kit SDK validation suite.
# This is intentionally narrower than full CI: it checks the Agent Kit manifest,
# the aggregate SDK manifest gate, Agent Kit tests in each language, and docs gates.

$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

function Invoke-Step {
  param(
    [string]$Name,
    [scriptblock]$Command
  )
  Write-Host "`n=== $Name ===" -ForegroundColor Cyan
  & $Command
  if ($LASTEXITCODE -ne 0) {
    throw "$Name failed with exit code $LASTEXITCODE"
  }
}

Invoke-Step "Agent Kit SDK manifest" {
  node sdk\scripts\check-agent-kit-sdk-manifest.mjs
}

Invoke-Step "SDK manifest suite" {
  node sdk\scripts\check-sdk-manifests.mjs
}

Invoke-Step "TypeScript Agent Kit slice" {
  Push-Location sdk\typescript
  try {
    node scripts\run-incremental-tests.mjs agent-kit
  } finally {
    Pop-Location
  }
}

Invoke-Step "Python Agent Kit helpers" {
  python -m unittest sdk.python.tests.test_agent_kit -v
}

Invoke-Step "Go Agent Kit helpers" {
  go test ./sdk/go/yunque -run "AgentKit|PluginRuntimeNamespace" -count=1
}

Invoke-Step "Rust Agent Kit helpers" {
  cargo test --manifest-path sdk\rust\Cargo.toml agent_kit -q
}

Invoke-Step "Docs SDK manifest gate" {
  npm --prefix docs run check:sdk-manifest
}

Remove-Item -Recurse -Force sdk\python\tests\__pycache__,sdk\python\yunque\__pycache__ -ErrorAction SilentlyContinue

Write-Host "`nAgent Kit SDK validation passed." -ForegroundColor Green
