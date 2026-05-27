#!/usr/bin/env bash
# Yunque SDK Publishing Script
# This script helps publish all three SDKs (Python, Rust, TypeScript) to their respective registries
# Usage: ./publish-sdk.sh [--dry-run] [--skip-tests] [--sdk python|rust|typescript|all]

set -euo pipefail

# Default options
DRY_RUN=false
SKIP_TESTS=false
SDK="all"
VERSION=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Helper functions
log_info() { echo -e "${CYAN}$*${NC}"; }
log_success() { echo -e "${GREEN}$*${NC}"; }
log_warning() { echo -e "${YELLOW}$*${NC}"; }
log_error() { echo -e "${RED}$*${NC}"; }

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --sdk)
            SDK="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --dry-run       Perform a dry run without actually publishing"
            echo "  --skip-tests    Skip running tests"
            echo "  --sdk SDK       Publish specific SDK: python, rust, typescript, or all (default: all)"
            echo "  --version VER   Specify version (default: auto-detect)"
            echo "  -h, --help      Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Get repository root
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

log_info "=== Yunque SDK Publishing Script ==="
log_info "Repository: $REPO_ROOT"
log_info "Dry Run: $DRY_RUN"
log_info "Skip Tests: $SKIP_TESTS"
log_info "SDK: $SDK"
echo ""

# Verify git status
check_git_status() {
    log_info "Checking git status..."
    if [[ -n $(git status --porcelain) ]]; then
        log_warning "Working directory has uncommitted changes:"
        git status --short
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_error "Aborted by user"
            exit 1
        fi
    else
        log_success "Working directory is clean"
    fi
}

# Verify version consistency
check_version_consistency() {
    log_info "Checking version consistency..."

    # Python version
    local python_version=$(grep -oP 'version = "\K[^"]+' sdk/python/pyproject.toml)

    # Rust version
    local rust_version=$(grep -oP '^version = "\K[^"]+' sdk/rust/Cargo.toml | head -1)

    # TypeScript version
    local ts_version=$(jq -r '.version' packages/yunque-client/package.json)

    log_info "Python:     $python_version"
    log_info "Rust:       $rust_version"
    log_info "TypeScript: $ts_version"

    if [[ "$python_version" != "$rust_version" ]] || [[ "$rust_version" != "$ts_version" ]]; then
        log_error "Version mismatch detected!"
        exit 1
    fi

    log_success "All versions match: $python_version"
    echo "$python_version"
}

# Publish Python SDK
publish_python_sdk() {
    local version=$1
    local dry_run=$2
    local skip_tests=$3

    log_info ""
    log_info "=== Publishing Python SDK ==="
    cd "$REPO_ROOT/sdk/python"

    # Clean previous builds
    log_info "Cleaning previous builds..."
    rm -rf dist build *.egg-info

    # Run tests
    if [[ "$skip_tests" != "true" ]]; then
        log_info "Running tests..."
        python -m pytest
        log_success "Tests passed"
    fi

    # Build package
    log_info "Building package..."
    python -m build
    log_success "Build successful"

    # Check package
    log_info "Checking package with twine..."
    twine check dist/*
    log_success "Package check passed"

    # List package contents
    log_info "Package contents:"
    ls -lh dist/

    if [[ "$dry_run" == "true" ]]; then
        log_warning "DRY RUN: Would upload to PyPI"
        log_info "To upload manually, run:"
        log_info "  cd sdk/python"
        log_info "  twine upload dist/*"
    else
        log_warning "Ready to upload to PyPI"
        read -p "Upload to PyPI? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Uploading to PyPI..."
            twine upload dist/*
            log_success "Python SDK published to PyPI!"
        else
            log_warning "Upload skipped"
        fi
    fi

    cd "$REPO_ROOT"
}

# Publish Rust SDK
publish_rust_sdk() {
    local version=$1
    local dry_run=$2
    local skip_tests=$3

    log_info ""
    log_info "=== Publishing Rust SDK ==="
    cd "$REPO_ROOT/sdk/rust"

    # Clean build
    log_info "Cleaning build..."
    cargo clean

    # Run tests
    if [[ "$skip_tests" != "true" ]]; then
        log_info "Running tests..."
        cargo test
        log_success "Tests passed"
    fi

    # Build release
    log_info "Building release..."
    cargo build --release
    log_success "Build successful"

    # Check package
    log_info "Checking package..."
    cargo package --list

    if [[ "$dry_run" == "true" ]]; then
        log_warning "DRY RUN: Would publish to crates.io"
        log_info "To publish manually, run:"
        log_info "  cd sdk/rust"
        log_info "  cargo publish"
    else
        log_warning "Ready to publish to crates.io"
        log_warning "NOTE: Publishing to crates.io is PERMANENT and cannot be undone!"
        read -p "Publish to crates.io? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Publishing to crates.io..."
            cargo publish
            log_success "Rust SDK published to crates.io!"
        else
            log_warning "Publish skipped"
        fi
    fi

    cd "$REPO_ROOT"
}

# Publish TypeScript SDK
publish_typescript_sdk() {
    local version=$1
    local dry_run=$2
    local skip_tests=$3

    log_info ""
    log_info "=== Publishing TypeScript SDK ==="
    cd "$REPO_ROOT/packages/yunque-client"

    # Install dependencies
    log_info "Installing dependencies..."
    npm install

    # Run prepublish checks
    if [[ "$skip_tests" != "true" ]]; then
        log_info "Running prepublish checks..."
        npm run prepublishOnly
        log_success "Prepublish checks passed"
    fi

    # Dry run pack
    log_info "Checking package contents..."
    npm pack --dry-run

    if [[ "$dry_run" == "true" ]]; then
        log_warning "DRY RUN: Would publish to npm"
        log_info "To publish manually, run:"
        log_info "  cd packages/yunque-client"
        log_info "  npm publish"
    else
        log_warning "Ready to publish to npm"
        read -p "Publish to npm? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Publishing to npm..."
            npm publish
            log_success "TypeScript SDK published to npm!"
        else
            log_warning "Publish skipped"
        fi
    fi

    cd "$REPO_ROOT"
}

# Create git tag
create_git_tag() {
    local version=$1

    log_info ""
    log_info "=== Creating Git Tag ==="
    local tag="v$version"

    log_info "Creating tag: $tag"
    git tag -a "$tag" -m "Release $tag"

    log_success "Tag created: $tag"
    log_info "To push the tag, run:"
    log_info "  git push origin $tag"
}

# Main execution
main() {
    # Pre-flight checks
    check_git_status
    local detected_version=$(check_version_consistency)

    if [[ -n "$VERSION" ]] && [[ "$VERSION" != "$detected_version" ]]; then
        log_warning "Specified version ($VERSION) differs from detected version ($detected_version)"
        read -p "Continue with detected version? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    VERSION="$detected_version"

    log_info ""
    log_info "Publishing version: $VERSION"

    # Publish SDKs
    if [[ "$SDK" == "all" ]] || [[ "$SDK" == "python" ]]; then
        publish_python_sdk "$VERSION" "$DRY_RUN" "$SKIP_TESTS"
    fi

    if [[ "$SDK" == "all" ]] || [[ "$SDK" == "rust" ]]; then
        publish_rust_sdk "$VERSION" "$DRY_RUN" "$SKIP_TESTS"
    fi

    if [[ "$SDK" == "all" ]] || [[ "$SDK" == "typescript" ]]; then
        publish_typescript_sdk "$VERSION" "$DRY_RUN" "$SKIP_TESTS"
    fi

    # Create git tag
    if [[ "$DRY_RUN" != "true" ]]; then
        echo ""
        read -p "Create git tag v$VERSION? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            create_git_tag "$VERSION"
        fi
    fi

    log_success ""
    log_success "=== Publishing Complete ==="
    log_info "Next steps:"
    log_info "1. Push git tag: git push origin v$VERSION"
    log_info "2. Create GitHub release: gh release create v$VERSION"
    log_info "3. Verify installations:"
    log_info "   - pip install yunque"
    log_info "   - cargo add yunque-client"
    log_info "   - npm install yunque-client"
    log_info "4. Update documentation and announce release"
}

# Run main function
main
