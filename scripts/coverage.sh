#!/bin/bash
set -e

echo "=== Yunque Agent Test Coverage ==="
echo ""

# Prepare data dirs
mkdir -p data/plugins data/sessions data/persona/skills data/cron data/audit web/out
test -f web/out/index.html || echo '<!DOCTYPE html><html><body></body></html>' > web/out/index.html

# Run tests with coverage
echo "Running tests..."
go test ./... -coverprofile=coverage.out -count=1 -timeout 300s

echo ""
echo "=== Coverage by Package ==="
go tool cover -func=coverage.out | grep -E "^(total|yunque)" | head -30

echo ""
echo "=== Total ==="
go tool cover -func=coverage.out | tail -1

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
echo ""
echo "HTML report: coverage.html"
