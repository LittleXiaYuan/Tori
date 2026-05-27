# Yunque SDK Publishing Script
# This script helps publish all three SDKs (Python, Rust, TypeScript) to their respective registries
# Usage: .\publish-sdk.ps1 [-DryRun] [-SkipTests] [-SDK <python|rust|typescript|all>]

param(
    [switch]$DryRun = $false,
    [switch]$SkipTests = $false,
    [ValidateSet('python', 'rust', 'typescript', 'all')]
    [string]$SDK = 'all',
    [string]$Version = ''
)

$ErrorActionPreference = "Stop"

# Colors for output
function Write-Success { Write-Host $args -ForegroundColor Green }
function Write-Info { Write-Host $args -ForegroundColor Cyan }
function Write-Warning { Write-Host $args -ForegroundColor Yellow }
function Write-Error { Write-Host $args -ForegroundColor Red }

# Get repository root
$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $RepoRoot

Write-Info "=== Yunque SDK Publishing Script ==="
Write-Info "Repository: $RepoRoot"
Write-Info "Dry Run: $DryRun"
Write-Info "Skip Tests: $SkipTests"
Write-Info "SDK: $SDK"
Write-Info ""

# Verify git status
function Test-GitStatus {
    Write-Info "Checking git status..."
    $status = git status --porcelain
    if ($status) {
        Write-Warning "Working directory has uncommitted changes:"
        Write-Host $status
        $continue = Read-Host "Continue anyway? (y/N)"
        if ($continue -ne 'y') {
            Write-Error "Aborted by user"
            exit 1
        }
    } else {
        Write-Success "Working directory is clean"
    }
}

# Verify version consistency
function Test-VersionConsistency {
    Write-Info "Checking version consistency..."

    # Python version
    $pythonVersion = (Get-Content "sdk/python/pyproject.toml" | Select-String 'version = "(.+)"').Matches.Groups[1].Value

    # Rust version
    $rustVersion = (Get-Content "sdk/rust/Cargo.toml" | Select-String 'version = "(.+)"').Matches.Groups[1].Value

    # TypeScript version
    $tsVersion = (Get-Content "packages/yunque-client/package.json" | ConvertFrom-Json).version

    Write-Info "Python:     $pythonVersion"
    Write-Info "Rust:       $rustVersion"
    Write-Info "TypeScript: $tsVersion"

    if ($pythonVersion -ne $rustVersion -or $rustVersion -ne $tsVersion) {
        Write-Error "Version mismatch detected!"
        exit 1
    }

    Write-Success "All versions match: $pythonVersion"
    return $pythonVersion
}

# Publish Python SDK
function Publish-PythonSDK {
    param([string]$Version, [bool]$DryRun, [bool]$SkipTests)

    Write-Info "`n=== Publishing Python SDK ==="
    Set-Location "$RepoRoot/sdk/python"

    # Clean previous builds
    Write-Info "Cleaning previous builds..."
    if (Test-Path "dist") { Remove-Item -Recurse -Force dist }
    if (Test-Path "build") { Remove-Item -Recurse -Force build }
    Get-ChildItem -Filter "*.egg-info" -Recurse | Remove-Item -Recurse -Force

    # Run tests
    if (-not $SkipTests) {
        Write-Info "Running tests..."
        python -m pytest
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Tests failed!"
            exit 1
        }
        Write-Success "Tests passed"
    }

    # Build package
    Write-Info "Building package..."
    python -m build
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Build failed!"
        exit 1
    }
    Write-Success "Build successful"

    # Check package
    Write-Info "Checking package with twine..."
    twine check dist/*
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Package check failed!"
        exit 1
    }
    Write-Success "Package check passed"

    # List package contents
    Write-Info "Package contents:"
    Get-ChildItem dist

    if ($DryRun) {
        Write-Warning "DRY RUN: Would upload to PyPI"
        Write-Info "To upload manually, run:"
        Write-Info "  cd sdk/python"
        Write-Info "  twine upload dist/*"
    } else {
        Write-Warning "Ready to upload to PyPI"
        $confirm = Read-Host "Upload to PyPI? (y/N)"
        if ($confirm -eq 'y') {
            Write-Info "Uploading to PyPI..."
            twine upload dist/*
            if ($LASTEXITCODE -ne 0) {
                Write-Error "Upload failed!"
                exit 1
            }
            Write-Success "Python SDK published to PyPI!"
        } else {
            Write-Warning "Upload skipped"
        }
    }

    Set-Location $RepoRoot
}

# Publish Rust SDK
function Publish-RustSDK {
    param([string]$Version, [bool]$DryRun, [bool]$SkipTests)

    Write-Info "`n=== Publishing Rust SDK ==="
    Set-Location "$RepoRoot/sdk/rust"

    # Clean build
    Write-Info "Cleaning build..."
    cargo clean

    # Run tests
    if (-not $SkipTests) {
        Write-Info "Running tests..."
        cargo test
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Tests failed!"
            exit 1
        }
        Write-Success "Tests passed"
    }

    # Build release
    Write-Info "Building release..."
    cargo build --release
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Build failed!"
        exit 1
    }
    Write-Success "Build successful"

    # Check package
    Write-Info "Checking package..."
    cargo package --list

    if ($DryRun) {
        Write-Warning "DRY RUN: Would publish to crates.io"
        Write-Info "To publish manually, run:"
        Write-Info "  cd sdk/rust"
        Write-Info "  cargo publish"
    } else {
        Write-Warning "Ready to publish to crates.io"
        Write-Warning "NOTE: Publishing to crates.io is PERMANENT and cannot be undone!"
        $confirm = Read-Host "Publish to crates.io? (y/N)"
        if ($confirm -eq 'y') {
            Write-Info "Publishing to crates.io..."
            cargo publish
            if ($LASTEXITCODE -ne 0) {
                Write-Error "Publish failed!"
                exit 1
            }
            Write-Success "Rust SDK published to crates.io!"
        } else {
            Write-Warning "Publish skipped"
        }
    }

    Set-Location $RepoRoot
}

# Publish TypeScript SDK
function Publish-TypeScriptSDK {
    param([string]$Version, [bool]$DryRun, [bool]$SkipTests)

    Write-Info "`n=== Publishing TypeScript SDK ==="
    Set-Location "$RepoRoot/packages/yunque-client"

    # Install dependencies
    Write-Info "Installing dependencies..."
    npm install
    if ($LASTEXITCODE -ne 0) {
        Write-Error "npm install failed!"
        exit 1
    }

    # Run prepublish checks
    if (-not $SkipTests) {
        Write-Info "Running prepublish checks..."
        npm run prepublishOnly
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Prepublish checks failed!"
            exit 1
        }
        Write-Success "Prepublish checks passed"
    }

    # Dry run pack
    Write-Info "Checking package contents..."
    npm pack --dry-run

    if ($DryRun) {
        Write-Warning "DRY RUN: Would publish to npm"
        Write-Info "To publish manually, run:"
        Write-Info "  cd packages/yunque-client"
        Write-Info "  npm publish"
    } else {
        Write-Warning "Ready to publish to npm"
        $confirm = Read-Host "Publish to npm? (y/N)"
        if ($confirm -eq 'y') {
            Write-Info "Publishing to npm..."
            npm publish
            if ($LASTEXITCODE -ne 0) {
                Write-Error "Publish failed!"
                exit 1
            }
            Write-Success "TypeScript SDK published to npm!"
        } else {
            Write-Warning "Publish skipped"
        }
    }

    Set-Location $RepoRoot
}

# Create git tag
function New-GitTag {
    param([string]$Version)

    Write-Info "`n=== Creating Git Tag ==="
    $tag = "v$Version"

    Write-Info "Creating tag: $tag"
    git tag -a $tag -m "Release $tag"

    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to create tag!"
        exit 1
    }

    Write-Success "Tag created: $tag"
    Write-Info "To push the tag, run:"
    Write-Info "  git push origin $tag"
}

# Main execution
try {
    # Pre-flight checks
    Test-GitStatus
    $detectedVersion = Test-VersionConsistency

    if ($Version -and $Version -ne $detectedVersion) {
        Write-Warning "Specified version ($Version) differs from detected version ($detectedVersion)"
        $continue = Read-Host "Continue with detected version? (y/N)"
        if ($continue -ne 'y') {
            exit 1
        }
    }
    $Version = $detectedVersion

    Write-Info "`nPublishing version: $Version"

    # Publish SDKs
    if ($SDK -eq 'all' -or $SDK -eq 'python') {
        Publish-PythonSDK -Version $Version -DryRun $DryRun -SkipTests $SkipTests
    }

    if ($SDK -eq 'all' -or $SDK -eq 'rust') {
        Publish-RustSDK -Version $Version -DryRun $DryRun -SkipTests $SkipTests
    }

    if ($SDK -eq 'all' -or $SDK -eq 'typescript') {
        Publish-TypeScriptSDK -Version $Version -DryRun $DryRun -SkipTests $SkipTests
    }

    # Create git tag
    if (-not $DryRun) {
        $createTag = Read-Host "`nCreate git tag v$Version? (y/N)"
        if ($createTag -eq 'y') {
            New-GitTag -Version $Version
        }
    }

    Write-Success "`n=== Publishing Complete ==="
    Write-Info "Next steps:"
    Write-Info "1. Push git tag: git push origin v$Version"
    Write-Info "2. Create GitHub release: gh release create v$Version"
    Write-Info "3. Verify installations:"
    Write-Info "   - pip install yunque"
    Write-Info "   - cargo add yunque-client"
    Write-Info "   - npm install yunque-client"
    Write-Info "4. Update documentation and announce release"

} catch {
    Write-Error "An error occurred: $_"
    exit 1
}
