#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════╗
# ║  Yunque Agent — One-click Deploy Script             ║
# ╚══════════════════════════════════════════════════════╝
#
# Usage:
#   ./scripts/deploy.sh              # default: lite mode (embedded SQLite)
#   ./scripts/deploy.sh dev          # dev mode: build + start with logs
#   ./scripts/deploy.sh prod         # prod mode: full stack (PostgreSQL)
#   ./scripts/deploy.sh prod-lite    # prod mode: lite (embedded SQLite)
#   ./scripts/deploy.sh stop         # stop all services
#   ./scripts/deploy.sh logs         # tail logs
#   ./scripts/deploy.sh status       # show service status

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PARENT_DIR="$(cd "$PROJECT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "${CYAN}[STEP]${NC}  $*"; }

check_prerequisites() {
    log_step "Checking prerequisites..."

    if ! command -v docker &>/dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        log_info  "  → https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! docker info &>/dev/null 2>&1; then
        log_error "Docker daemon is not running. Please start Docker."
        exit 1
    fi

    if ! docker compose version &>/dev/null 2>&1; then
        log_error "Docker Compose V2 is required. Please update Docker."
        exit 1
    fi

    log_info "Docker $(docker --version | grep -oP '\d+\.\d+\.\d+') ✓"
    log_info "Compose $(docker compose version --short) ✓"
}

ensure_env() {
    local env_file="$PROJECT_DIR/.env"
    local env_example="$PROJECT_DIR/.env.example"

    if [ ! -f "$env_file" ]; then
        if [ -f "$env_example" ]; then
            log_warn ".env not found, copying from .env.example..."
            cp "$env_example" "$env_file"
            log_warn "Please edit .env and set at minimum:"
            log_warn "  • LLM_API_KEY"
            log_warn "  • JWT_SECRET  (run: openssl rand -hex 32)"
            log_warn "  • POSTGRES_PASSWORD (for full mode, run: openssl rand -hex 32)"
            exit 1
        else
            log_error "Neither .env nor .env.example found."
            exit 1
        fi
    fi

    source "$env_file" 2>/dev/null || true

    if [ -z "${LLM_API_KEY:-}" ] || [ "$LLM_API_KEY" = "sk-your-api-key-here" ]; then
        log_error "LLM_API_KEY is not set in .env"
        exit 1
    fi

    local jwt="${JWT_SECRET:-}"
    if [ -z "$jwt" ] || [[ "$jwt" == *"REPLACE"* ]] || [[ "$jwt" == *"changeme"* ]] || [ "${#jwt}" -lt 16 ]; then
        log_error "JWT_SECRET is missing or insecure in .env"
        log_info  "Generate one: openssl rand -hex 32"
        exit 1
    fi

    log_info "Environment validated ✓"
}

do_build() {
    local profile="$1"
    log_step "Building images (profile: $profile)..."
    docker compose -f "$PROJECT_DIR/docker-compose.yml" \
        --project-directory "$PARENT_DIR" \
        --profile "$profile" \
        build \
        --build-arg VERSION="${VERSION:-0.1.0}" \
        --build-arg GIT_COMMIT="$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
        --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

do_up() {
    local profile="$1"
    local detach="${2:--d}"
    log_step "Starting services (profile: $profile)..."
    docker compose -f "$PROJECT_DIR/docker-compose.yml" \
        --project-directory "$PARENT_DIR" \
        --profile "$profile" \
        up $detach --remove-orphans
}

do_stop() {
    log_step "Stopping all services..."
    docker compose -f "$PROJECT_DIR/docker-compose.yml" \
        --project-directory "$PARENT_DIR" \
        --profile lite --profile full \
        down
    log_info "All services stopped."
}

do_logs() {
    docker compose -f "$PROJECT_DIR/docker-compose.yml" \
        --project-directory "$PARENT_DIR" \
        --profile lite --profile full \
        logs -f --tail=100
}

do_status() {
    docker compose -f "$PROJECT_DIR/docker-compose.yml" \
        --project-directory "$PARENT_DIR" \
        --profile lite --profile full \
        ps -a
}

wait_healthy() {
    local service="$1"
    local max_wait=60
    local elapsed=0
    log_step "Waiting for $service to become healthy..."
    while [ $elapsed -lt $max_wait ]; do
        local status
        status=$(docker compose -f "$PROJECT_DIR/docker-compose.yml" \
            --project-directory "$PARENT_DIR" \
            ps --format json 2>/dev/null | \
            grep -o "\"Health\":\"[^\"]*\"" | head -1 | cut -d'"' -f4 || echo "unknown")
        if [ "$status" = "healthy" ]; then
            log_info "$service is healthy ✓"
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    log_warn "$service health check timed out after ${max_wait}s (may still be starting)"
}

MODE="${1:-lite}"

case "$MODE" in
    dev)
        check_prerequisites
        ensure_env
        do_build lite
        log_info "Starting in DEV mode (foreground with logs)..."
        do_up lite ""
        ;;
    prod)
        check_prerequisites
        ensure_env
        if [ -z "${POSTGRES_PASSWORD:-}" ] || [[ "${POSTGRES_PASSWORD:-}" == *"REPLACE"* ]]; then
            log_error "POSTGRES_PASSWORD is missing or insecure for full mode."
            log_info  "Generate one: openssl rand -hex 32"
            exit 1
        fi
        do_build full
        do_up full -d
        wait_healthy "postgres"
        log_info "Production (full) stack is running!"
        log_info "  Dashboard: http://localhost:${AGENT_PORT:-9090}"
        log_info "  API:       http://localhost:${AGENT_PORT:-9090}/v1/"
        ;;
    prod-lite|lite)
        check_prerequisites
        ensure_env
        do_build lite
        do_up lite -d
        wait_healthy "agent-lite"
        log_info "Lite stack is running!"
        log_info "  Dashboard: http://localhost:${AGENT_PORT:-9090}"
        log_info "  API:       http://localhost:${AGENT_PORT:-9090}/v1/"
        ;;
    stop)
        do_stop
        ;;
    logs)
        do_logs
        ;;
    status)
        do_status
        ;;
    restart)
        do_stop
        check_prerequisites
        ensure_env
        do_build lite
        do_up lite -d
        log_info "Restarted (lite mode)."
        ;;
    *)
        echo "Usage: $0 {dev|prod|prod-lite|lite|stop|logs|status|restart}"
        echo ""
        echo "Modes:"
        echo "  dev        Build + start lite mode in foreground (with logs)"
        echo "  prod       Full stack: PostgreSQL + pgvector + Agent"
        echo "  prod-lite  Production lite: embedded SQLite (no external deps)"
        echo "  lite       Same as prod-lite (default)"
        echo "  stop       Stop all services"
        echo "  logs       Tail service logs"
        echo "  status     Show service status"
        echo "  restart    Stop + rebuild + start (lite)"
        exit 1
        ;;
esac
