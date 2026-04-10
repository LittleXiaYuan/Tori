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

.PHONY: build release clean test setup web-ensure web-build

## web-ensure: Ensure heroui-web/out/ exists (placeholder if no build)
web-ensure:
	@mkdir -p heroui-web/out
	@test -f heroui-web/out/index.html || echo '<!DOCTYPE html><html><body><p>Run make web-build</p></body></html>' > heroui-web/out/index.html

## web-build: Build Next.js frontend (requires Node.js)
web-build:
	@echo "Building frontend..."
	cd heroui-web && npm ci && npm run build
	@echo "Frontend built: heroui-web/out/"

## build: Build for current platform (with placeholder frontend if not built)
build: web-ensure
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME) ./cmd/agent
	@echo "Built: $(DIST_DIR)/$(APP_NAME)"

## build-full: Build frontend + Go binary
build-full: web-build
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME) ./cmd/agent
	@echo "Built (with frontend): $(DIST_DIR)/$(APP_NAME)"

## release: Cross-compile for all platforms (6 targets, with frontend)
release: clean web-build
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

## coverage: Run tests with coverage report
coverage: web-ensure
	@bash scripts/coverage.sh

## lint: Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	golangci-lint run ./...

## vet: Run go vet only
vet:
	go vet ./...

## setup: Build and run setup wizard
setup:
	go run ./cmd/setup

## clean: Remove build artifacts
clean:
	rm -rf $(DIST_DIR)
