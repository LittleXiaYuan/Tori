# ╔══════════════════════════════════════════╗
# ║  云雀 Agent (Yunque Agent) — Makefile    ║
# ╚══════════════════════════════════════════╝

APP_NAME    := yunque-agent
MODULE      := yunque-agent
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

LDFLAGS     := -s -w \
               -X $(MODULE)/internal/version.Version=$(VERSION) \
               -X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
               -X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

DIST_DIR    := dist

# Cross-compilation targets
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build build-full release clean test test-web test-all coverage lint lint-go lint-web vet setup openapi docs-api check check-all web-ensure web-build sbom vulncheck release-safe

## web-ensure: Ensure apps/web/out/ exists (placeholder if no build)
web-ensure:
	@mkdir -p apps/web/out
	@test -f apps/web/out/index.html || echo '<!DOCTYPE html><html><body><p>Run make web-build</p></body></html>' > apps/web/out/index.html

## web-build: Build Next.js frontend (requires Node.js)
web-build:
	@echo "Building frontend..."
	cd apps/web && npm ci && npm run build
	@echo "Frontend built: apps/web/out/"

## build: Build for current platform (with placeholder frontend if not built)
build: web-ensure
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME) ./cmd/agent
	@echo "Built: $(DIST_DIR)/$(APP_NAME)"

## build-full: Build frontend + Go binary
build-full: web-build
	@test -d apps/web/out/_next || (echo "ERROR: apps/web/out/_next not found — frontend build failed" && exit 1)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME) ./cmd/agent
	@echo "Built (with frontend): $(DIST_DIR)/$(APP_NAME)"

## release: Cross-compile for all platforms (6 targets, with frontend)
release: clean web-build
	@test -d apps/web/out/_next || (echo "ERROR: apps/web/out/_next not found — frontend build failed" && exit 1)
	@mkdir -p $(DIST_DIR)
	@$(foreach platform,$(PLATFORMS), \
		$(eval OS=$(word 1,$(subst /, ,$(platform)))) \
		$(eval ARCH=$(word 2,$(subst /, ,$(platform)))) \
		$(eval EXT=$(if $(findstring windows,$(OS)),.exe,)) \
		$(eval OUT=$(DIST_DIR)/$(APP_NAME)-$(OS)-$(ARCH)$(EXT)) \
		echo "Building $(OUT)..." && \
		CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o $(OUT) ./cmd/agent && \
	) true
	@echo ""
	@echo "Release binaries:"
	@ls -lh $(DIST_DIR)/

## test: Run all tests
test: web-ensure
	go test ./... -count=1

## test-web: Run frontend tests and typecheck
test-web:
	@echo "Running frontend tests..."
	cd apps/web && npm test && npm run typecheck

## test-all: Run Go tests plus frontend tests
test-all: test test-web

## coverage: Run tests with coverage report
ifeq ($(OS),Windows_NT)
coverage: web-ensure
	powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File scripts/coverage.ps1
else
coverage: web-ensure
	@bash scripts/coverage.sh
endif

## lint: Run all linters (Go + frontend)
lint: lint-go lint-web

## lint-go: Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint-go:
	golangci-lint run ./...

## lint-web: Run frontend type checking and lint
lint-web:
	@if [ -f apps/web/node_modules/.package-lock.json ]; then \
		cd apps/web && npx tsc --noEmit; \
	else \
		echo "SKIP: apps/web/node_modules not installed (run 'cd apps/web && npm ci' first)"; \
	fi

## vet: Run go vet only (lightweight alternative to full lint)
vet:
	go vet ./...

## check: Pre-commit gate — lint + test (fails fast)
check: lint test

## check-all: Full repository gate — lint + Go tests + frontend tests
check-all: lint test-all

## setup: Build and run setup wizard
setup:
	go run ./cmd/setup

## openapi: Regenerate docs/openapi.yaml from gateway routes
openapi:
	go run ./cmd/openapi-gen
	go test ./cmd/openapi-gen

## docs-api: Serve the API reference (Scalar, reads docs/openapi.yaml)
##           Open http://localhost:8000/api-reference.html in your browser.
docs-api:
	@echo "Serving API reference at http://localhost:8000/api-reference.html"
	cd docs && python -m http.server 8000

## sbom: Generate CycloneDX SBOM from Go modules
sbom:
	@echo "Generating SBOM..."
	@which cyclonedx-gomod >/dev/null 2>&1 || (echo "Install: go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest" && exit 1)
	@mkdir -p $(DIST_DIR)
	cyclonedx-gomod mod -json -output $(DIST_DIR)/sbom.cdx.json
	cp $(DIST_DIR)/sbom.cdx.json internal/sbom/sbom.cdx.json
	@echo "SBOM generated: $(DIST_DIR)/sbom.cdx.json (and embedded copy)"

## vulncheck: Scan for known Go vulnerabilities
vulncheck:
	@echo "Scanning for vulnerabilities..."
	@which govulncheck >/dev/null 2>&1 || (echo "Install: go install golang.org/x/vuln/cmd/govulncheck@latest" && exit 1)
	govulncheck ./...
	@echo "Vulnerability scan complete"

## release-safe: Release with SBOM generation and vulnerability gate
release-safe: vulncheck sbom release

## clean: Remove build artifacts
clean:
ifeq ($(OS),Windows_NT)
	@powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "Remove-Item -Path 'coverage*','*.exe','dist' -Force -Recurse -ErrorAction SilentlyContinue"
else
	@rm -rf dist coverage coverage.out coverage.html coverage_new coverage_report*.txt *.exe
endif
