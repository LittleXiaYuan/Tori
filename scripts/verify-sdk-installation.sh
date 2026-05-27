#!/usr/bin/env bash
# Yunque SDK Verification Script
# This script verifies that published SDKs can be installed and used correctly
# Usage: ./verify-sdk-installation.sh [--version VERSION] [--sdk python|rust|typescript|all]

set -euo pipefail

# Default options
VERSION=""
SDK="all"
CLEANUP=true

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Helper functions
log_info() { echo -e "${CYAN}$*${NC}"; }
log_success() { echo -e "${GREEN}✓ $*${NC}"; }
log_warning() { echo -e "${YELLOW}⚠ $*${NC}"; }
log_error() { echo -e "${RED}✗ $*${NC}"; }

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --sdk)
            SDK="$2"
            shift 2
            ;;
        --no-cleanup)
            CLEANUP=false
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version VER   Specify version to verify (default: latest)"
            echo "  --sdk SDK       Verify specific SDK: python, rust, typescript, or all (default: all)"
            echo "  --no-cleanup    Don't clean up test directories"
            echo "  -h, --help      Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

TEMP_DIR=$(mktemp -d)
trap 'if [[ "$CLEANUP" == "true" ]]; then rm -rf "$TEMP_DIR"; fi' EXIT

log_info "=== Yunque SDK Verification Script ==="
log_info "Temp directory: $TEMP_DIR"
log_info "SDK: $SDK"
if [[ -n "$VERSION" ]]; then
    log_info "Version: $VERSION"
else
    log_info "Version: latest"
fi
echo ""

# Verify Python SDK
verify_python_sdk() {
    local version=$1
    log_info "=== Verifying Python SDK ==="

    local test_dir="$TEMP_DIR/python-test"
    mkdir -p "$test_dir"
    cd "$test_dir"

    # Create virtual environment
    log_info "Creating virtual environment..."
    python3 -m venv venv
    source venv/bin/activate

    # Install package
    log_info "Installing yunque package..."
    if [[ -n "$version" ]]; then
        pip install "yunque==$version" --quiet
    else
        pip install yunque --quiet
    fi

    # Verify installation
    log_info "Verifying installation..."
    local installed_version=$(python -c "import yunque; print(yunque.__version__)" 2>/dev/null || echo "")
    if [[ -z "$installed_version" ]]; then
        log_error "Failed to import yunque package"
        deactivate
        return 1
    fi
    log_success "Installed version: $installed_version"

    # Test basic imports
    log_info "Testing basic imports..."
    python -c "
from yunque_client import Client
import yunque

# Test client creation
client = Client('http://localhost:9090')
print('Client created successfully')

# Test agent kit
kit = yunque.create_agent_kit()
print('Agent kit created successfully')
" || {
        log_error "Import test failed"
        deactivate
        return 1
    }
    log_success "Basic imports work"

    # Test example code
    log_info "Testing example code..."
    cat > test_example.py << 'EOF'
from yunque_client import Client
import yunque

# Create client
client = Client('http://localhost:9090')
print(f"Client base URL: {client}")

# Create agent kit
kit = yunque.create_agent_kit()
print("Agent kit components available:")
print(f"  - State: {hasattr(kit, 'state')}")
print(f"  - Reflect: {hasattr(kit, 'reflect')}")
print(f"  - Missions: {hasattr(kit, 'missions')}")
print(f"  - Scheduler: {hasattr(kit, 'scheduler')}")

print("\nPython SDK verification successful!")
EOF

    python test_example.py || {
        log_error "Example code failed"
        deactivate
        return 1
    }
    log_success "Example code works"

    deactivate
    log_success "Python SDK verification complete"
    echo ""
}

# Verify Rust SDK
verify_rust_sdk() {
    local version=$1
    log_info "=== Verifying Rust SDK ==="

    local test_dir="$TEMP_DIR/rust-test"
    mkdir -p "$test_dir"
    cd "$test_dir"

    # Create test project
    log_info "Creating test project..."
    cargo init --bin --name yunque-test --quiet

    # Add dependency
    log_info "Adding yunque-client dependency..."
    if [[ -n "$version" ]]; then
        cargo add "yunque-client@$version" --quiet
    else
        cargo add yunque-client --quiet
    fi

    # Create test code
    log_info "Creating test code..."
    cat > src/main.rs << 'EOF'
use yunque_client::Client;

fn main() {
    // Create client
    let client = Client::new("http://localhost:9090");
    println!("Client created successfully");

    // Test that types are available
    println!("Rust SDK verification successful!");
}
EOF

    # Build
    log_info "Building project..."
    cargo build --quiet 2>&1 | grep -v "Compiling" | grep -v "Finished" || true

    # Run
    log_info "Running test..."
    cargo run --quiet || {
        log_error "Test run failed"
        return 1
    }
    log_success "Test run successful"

    log_success "Rust SDK verification complete"
    echo ""
}

# Verify TypeScript SDK
verify_typescript_sdk() {
    local version=$1
    log_info "=== Verifying TypeScript SDK ==="

    local test_dir="$TEMP_DIR/typescript-test"
    mkdir -p "$test_dir"
    cd "$test_dir"

    # Create package.json
    log_info "Creating test project..."
    cat > package.json << 'EOF'
{
  "name": "yunque-test",
  "version": "1.0.0",
  "type": "module",
  "private": true
}
EOF

    # Install package
    log_info "Installing yunque-client package..."
    if [[ -n "$version" ]]; then
        npm install "yunque-client@$version" --silent
    else
        npm install yunque-client --silent
    fi

    # Test basic imports
    log_info "Testing basic imports..."
    cat > test-imports.mjs << 'EOF'
import { createChatClient } from 'yunque-client/chat';
import { createAgentKitClient } from 'yunque-client/agent-kit';

console.log('✓ Chat client import successful');
console.log('✓ Agent kit import successful');
EOF

    node test-imports.mjs || {
        log_error "Import test failed"
        return 1
    }
    log_success "Basic imports work"

    # Test example code
    log_info "Testing example code..."
    cat > test-example.mjs << 'EOF'
import { createChatClient } from 'yunque-client/chat';
import { createAgentKitClient } from 'yunque-client/agent-kit';

// Create chat client
const chat = createChatClient({
  baseUrl: 'http://localhost:9090',
  token: 'test-token'
});
console.log('Chat client created successfully');

// Create agent kit client
const kit = createAgentKitClient({
  baseUrl: 'http://localhost:9090',
  token: 'test-token'
});
console.log('Agent kit client created successfully');

console.log('\nTypeScript SDK verification successful!');
EOF

    node test-example.mjs || {
        log_error "Example code failed"
        return 1
    }
    log_success "Example code works"

    # Test subpath exports
    log_info "Testing subpath exports..."
    cat > test-subpaths.mjs << 'EOF'
import { createChatClient } from 'yunque-client/chat';
import { createCognisClient } from 'yunque-client/cognis';
import { createMemoryClient } from 'yunque-client/memory';
import { createTasksClient } from 'yunque-client/tasks';
import { createKnowledgeClient } from 'yunque-client/knowledge';

console.log('✓ All subpath exports work');
EOF

    node test-subpaths.mjs || {
        log_error "Subpath exports test failed"
        return 1
    }
    log_success "Subpath exports work"

    log_success "TypeScript SDK verification complete"
    echo ""
}

# Main execution
main() {
    local failed=0

    if [[ "$SDK" == "all" ]] || [[ "$SDK" == "python" ]]; then
        verify_python_sdk "$VERSION" || ((failed++))
    fi

    if [[ "$SDK" == "all" ]] || [[ "$SDK" == "rust" ]]; then
        verify_rust_sdk "$VERSION" || ((failed++))
    fi

    if [[ "$SDK" == "all" ]] || [[ "$SDK" == "typescript" ]]; then
        verify_typescript_sdk "$VERSION" || ((failed++))
    fi

    echo ""
    if [[ $failed -eq 0 ]]; then
        log_success "=== All Verifications Passed ==="
        log_info "All SDKs are working correctly!"
    else
        log_error "=== $failed Verification(s) Failed ==="
        exit 1
    fi

    if [[ "$CLEANUP" == "false" ]]; then
        log_info "Test directory preserved at: $TEMP_DIR"
    fi
}

# Run main function
main
