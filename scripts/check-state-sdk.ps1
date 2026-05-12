# Runs the focused cross-language State Kernel SDK validation suite.
# This is intentionally narrower than full CI: it checks the SDK manifest,
# TypeScript focused state slices, Go/Python/Rust state helpers, and docs build.

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

Invoke-Step "State SDK manifest" {
  node sdk\scripts\check-state-sdk-manifest.mjs
}

Invoke-Step "TypeScript focused state slices" {
  Push-Location sdk\typescript
  try {
    npm run check:state-manifest
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    node scripts\run-incremental-tests.mjs state-snapshot state-actions state-capabilities
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

Write-Host "`nState SDK validation passed." -ForegroundColor Green
