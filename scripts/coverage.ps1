$ErrorActionPreference = "Stop"

Write-Host "=== Yunque Agent Test Coverage ==="
Write-Host ""

$goRoot = & go env GOROOT
$goExe = Join-Path $goRoot "bin\go.exe"
if (-not (Test-Path $goExe)) {
  throw "go.exe not found at $goExe"
}

$dirs = @(
  "data/plugins",
  "data/sessions",
  "data/persona/skills",
  "data/cron",
  "data/audit",
  "heroui-web/out"
)

foreach ($dir in $dirs) {
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
}

$indexPath = "heroui-web/out/index.html"
if (-not (Test-Path $indexPath)) {
  Set-Content -Path $indexPath -Value "<!DOCTYPE html><html><body></body></html>"
}

Write-Host "Running tests..."
& $goExe test ./... -coverprofile=coverage.out -count=1 -timeout 300s
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

Write-Host ""
Write-Host "=== Coverage by Package ==="
& $goExe tool cover -func coverage.out | Select-String -Pattern '^(total|yunque)' | Select-Object -First 30 | ForEach-Object { $_.Line }
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

Write-Host ""
Write-Host "=== Total ==="
& $goExe tool cover -func coverage.out | Select-Object -Last 1 | ForEach-Object { $_ }
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

& $goExe tool cover -html coverage.out -o coverage.html
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

Write-Host ""
Write-Host "HTML report: coverage.html"
