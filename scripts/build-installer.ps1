# ╔══════════════════════════════════════════════╗
# ║  Build Yunque Agent Windows Installer        ║
# ╚══════════════════════════════════════════════╝
#
# Usage: .\scripts\build-installer.ps1 [-Version "1.0.0"]
#
# Prerequisites:
#   - Go 1.25+
#   - Inno Setup 6+ (iscc.exe in PATH or default install location)
#   - Node.js (for frontend build, optional if apps/web/out/ exists)

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

# Determine version
if (-not $Version) {
    $Version = git describe --tags --always --dirty 2>$null
    if (-not $Version) { $Version = "0.1.0-dev" }
}
$GitCommit = git rev-parse --short HEAD 2>$null
if (-not $GitCommit) { $GitCommit = "unknown" }
$BuildDate = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")

Write-Host "Building Yunque Agent $Version ($GitCommit)" -ForegroundColor Cyan

# Step 1: Build Go binary
Write-Host "`n[1/3] Compiling Go binary..." -ForegroundColor Yellow
$ldflags = "-s -w -H windowsgui " +
    "-X yunque-agent/internal/version.Version=$Version " +
    "-X yunque-agent/internal/version.GitCommit=$GitCommit " +
    "-X yunque-agent/internal/version.BuildDate=$BuildDate"

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

if (-not (Test-Path "dist")) { New-Item -ItemType Directory -Path "dist" | Out-Null }

go build -ldflags $ldflags -o "dist\yunque-agent.exe" .\cmd\agent
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
Write-Host "  Built: dist\yunque-agent.exe" -ForegroundColor Green

# Step 2: Ensure frontend (optional)
if (-not (Test-Path "apps/web\out\index.html")) {
    Write-Host "`n[2/3] Frontend not built, creating placeholder..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Path "apps/web\out" -Force | Out-Null
    Set-Content "apps/web\out\index.html" '<!DOCTYPE html><html><body><p>Run npm build in apps/web/</p></body></html>'
} else {
    Write-Host "`n[2/3] Frontend already built." -ForegroundColor Green
}

# Step 3: Build installer
Write-Host "`n[3/3] Building Inno Setup installer..." -ForegroundColor Yellow

$iscc = "iscc.exe"
$defaultPaths = @(
    "C:\InnoSetup6\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles}\Inno Setup 6\ISCC.exe"
)
foreach ($p in $defaultPaths) {
    if (Test-Path $p) { $iscc = $p; break }
}

& $iscc "/DMyAppVersion=$Version" "installer\yunque.iss"
if ($LASTEXITCODE -ne 0) { throw "Inno Setup build failed" }

Write-Host "`nInstaller built successfully!" -ForegroundColor Green
$outputFile = "installer\Output\YunqueAgent-Setup-$Version.exe"
if (Test-Path $outputFile) {
    $size = (Get-Item $outputFile).Length / 1MB
    Write-Host "  Output: $outputFile ($([math]::Round($size, 1)) MB)" -ForegroundColor Cyan
}
