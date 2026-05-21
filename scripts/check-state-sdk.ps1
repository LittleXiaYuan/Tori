# Runs the focused cross-language SDK validation suite.
# This is intentionally narrower than full CI: it checks all SDK manifests,
# TypeScript focused SDK slices, Go/Python/Rust SDK helpers, and docs gates.

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

Invoke-Step "SDK manifest suite" {
  node sdk\scripts\check-sdk-manifests.mjs
}

Invoke-Step "TypeScript focused state slices" {
  Push-Location packages\yunque-client
  try {
    npm run check:sdk-manifests
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    node scripts\run-incremental-tests.mjs state-snapshot state-actions state-capabilities
  } finally {
    Pop-Location
  }
}

Invoke-Step "TypeScript focused reflect slices" {
  Push-Location packages\yunque-client
  try {
    node scripts\run-incremental-tests.mjs reflect reflect-experiences reflect-strategies
  } finally {
    Pop-Location
  }
}

Invoke-Step "TypeScript focused plugin API slices" {
  Push-Location packages\yunque-client
  try {
    node scripts\run-incremental-tests.mjs plugin-api plugin-llm plugin-search plugin-send plugin-memory plugin-agent-memory plugin-knowledge plugin-cron plugin-extensions
  } finally {
    Pop-Location
  }
}

Invoke-Step "Go State SDK" {
  go test ./sdk/go/yunque -count=1
}

Invoke-Step "Python State SDK" {
  python -m unittest discover -s sdk\python\tests -v
}

Invoke-Step "Rust State SDK" {
  cargo test --manifest-path sdk\rust\Cargo.toml -q
}

Invoke-Step "Docs SDK manifest gate" {
  npm --prefix docs run check:sdk-manifest
}

Write-Host "`nSDK validation passed." -ForegroundColor Green
