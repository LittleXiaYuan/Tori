#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════╗
# ║  yunque-agent stability test — 2G server validation     ║
# ║  Tests: cgroup limits, OOM recovery, health checks      ║
# ╚══════════════════════════════════════════════════════════╝
#
# Usage:
#   sudo ./scripts/stress-test.sh              # run all checks
#   sudo ./scripts/stress-test.sh cgroup       # cgroup only
#   sudo ./scripts/stress-test.sh oom          # OOM recovery only
#   sudo ./scripts/stress-test.sh health       # health check only
#   sudo ./scripts/stress-test.sh memory       # runtime memory monitor
#
# Prerequisites: systemd-managed yunque-agent service

set -euo pipefail

SERVICE="yunque-agent"
AGENT_URL="http://localhost:9090"
PASS=0
FAIL=0
WARN=0

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_pass() { ((PASS++)); echo -e "  ${GREEN}[PASS]${NC} $1"; }
log_fail() { ((FAIL++)); echo -e "  ${RED}[FAIL]${NC} $1"; }
log_warn() { ((WARN++)); echo -e "  ${YELLOW}[WARN]${NC} $1"; }
log_info() { echo -e "  ${CYAN}[INFO]${NC} $1"; }

header() {
    echo ""
    echo "═══════════════════════════════════════════════"
    echo "  $1"
    echo "═══════════════════════════════════════════════"
}

# ─────────────── Check 1: cgroup Memory Limits ───────────────

check_cgroup() {
    header "Check: systemd cgroup Memory Limits"

    if ! systemctl is-active --quiet "$SERVICE" 2>/dev/null; then
        log_fail "Service '$SERVICE' is not running"
        log_info "Start it with: sudo systemctl start $SERVICE"
        return 1
    fi
    log_pass "Service '$SERVICE' is active"

    local pid
    pid=$(systemctl show -p MainPID --value "$SERVICE" 2>/dev/null)
    if [[ -z "$pid" || "$pid" == "0" ]]; then
        log_fail "Cannot determine PID for $SERVICE"
        return 1
    fi
    log_info "Service PID: $pid"

    local mem_max mem_high
    mem_max=$(systemctl show -p MemoryMax --value "$SERVICE" 2>/dev/null || echo "unknown")
    mem_high=$(systemctl show -p MemoryHigh --value "$SERVICE" 2>/dev/null || echo "unknown")

    log_info "MemoryMax configured: $mem_max"
    log_info "MemoryHigh configured: $mem_high"

    if [[ "$mem_max" == "infinity" || "$mem_max" == "unknown" ]]; then
        log_warn "MemoryMax is not set — cgroup hard limit not enforced"
    else
        log_pass "MemoryMax is configured: $mem_max"
    fi

    if [[ "$mem_high" == "infinity" || "$mem_high" == "unknown" ]]; then
        log_warn "MemoryHigh is not set — cgroup soft limit not enforced"
    else
        log_pass "MemoryHigh is configured: $mem_high"
    fi

    local cgroup_path="/sys/fs/cgroup/system.slice/${SERVICE}.service"
    if [[ -d "$cgroup_path" ]]; then
        log_pass "cgroup v2 path exists: $cgroup_path"

        if [[ -f "$cgroup_path/memory.max" ]]; then
            local cg_max
            cg_max=$(cat "$cgroup_path/memory.max")
            log_info "cgroup memory.max = $cg_max bytes ($(( ${cg_max} / 1024 / 1024 )) MiB)"
        fi

        if [[ -f "$cgroup_path/memory.high" ]]; then
            local cg_high
            cg_high=$(cat "$cgroup_path/memory.high")
            log_info "cgroup memory.high = $cg_high bytes ($(( ${cg_high} / 1024 / 1024 )) MiB)"
        fi

        if [[ -f "$cgroup_path/memory.current" ]]; then
            local cg_current
            cg_current=$(cat "$cgroup_path/memory.current")
            log_info "cgroup memory.current = $(( ${cg_current} / 1024 / 1024 )) MiB"
        fi
    else
        local cgroup_v1="/sys/fs/cgroup/memory/system.slice/${SERVICE}.service"
        if [[ -d "$cgroup_v1" ]]; then
            log_info "cgroup v1 detected at $cgroup_v1"
            if [[ -f "$cgroup_v1/memory.limit_in_bytes" ]]; then
                local v1_limit
                v1_limit=$(cat "$cgroup_v1/memory.limit_in_bytes")
                log_info "cgroup v1 memory.limit = $(( ${v1_limit} / 1024 / 1024 )) MiB"
            fi
        else
            log_warn "cgroup path not found — may be using different cgroup hierarchy"
        fi
    fi

    local env_vars
    env_vars=$(systemctl show -p Environment --value "$SERVICE" 2>/dev/null || echo "")

    if echo "$env_vars" | grep -q "GOMEMLIMIT"; then
        log_pass "GOMEMLIMIT is set in service environment"
    else
        if [[ -f "/opt/yunque-agent/.env" ]] && grep -q "GOMEMLIMIT" /opt/yunque-agent/.env 2>/dev/null; then
            log_pass "GOMEMLIMIT found in .env file"
        else
            log_warn "GOMEMLIMIT not found in service environment or .env"
        fi
    fi

    if echo "$env_vars" | grep -q "GOGC"; then
        log_pass "GOGC is set in service environment"
    else
        if [[ -f "/opt/yunque-agent/.env" ]] && grep -q "GOGC" /opt/yunque-agent/.env 2>/dev/null; then
            log_pass "GOGC found in .env file"
        else
            log_warn "GOGC not found in service environment or .env"
        fi
    fi

    local rss_kb
    rss_kb=$(ps -o rss= -p "$pid" 2>/dev/null || echo "0")
    local rss_mib=$(( rss_kb / 1024 ))
    log_info "Current process RSS: ${rss_mib} MiB"

    if (( rss_mib < 512 )); then
        log_pass "RSS (${rss_mib} MiB) within expected range for 2G server"
    elif (( rss_mib < 768 )); then
        log_warn "RSS (${rss_mib} MiB) approaching memory limit"
    else
        log_fail "RSS (${rss_mib} MiB) exceeds safe threshold for 2G server"
    fi
}

# ─────────────── Check 2: OOM Auto-Restart ───────────────

check_oom_recovery() {
    header "Check: OOM Scenario & Auto-Restart"

    if ! systemctl is-active --quiet "$SERVICE" 2>/dev/null; then
        log_fail "Service must be running for OOM test"
        return 1
    fi

    local pre_pid
    pre_pid=$(systemctl show -p MainPID --value "$SERVICE" 2>/dev/null)
    log_info "Pre-test PID: $pre_pid"

    local restart_setting
    restart_setting=$(systemctl show -p Restart --value "$SERVICE" 2>/dev/null || echo "unknown")
    log_info "Restart policy: $restart_setting"

    if [[ "$restart_setting" == "on-failure" || "$restart_setting" == "always" ]]; then
        log_pass "Restart policy ($restart_setting) will handle OOM crashes"
    else
        log_warn "Restart policy '$restart_setting' may not auto-recover from OOM"
    fi

    local restart_sec
    restart_sec=$(systemctl show -p RestartUSec --value "$SERVICE" 2>/dev/null || echo "unknown")
    log_info "RestartSec: $restart_sec"

    local burst limit_interval
    burst=$(systemctl show -p StartLimitBurst --value "$SERVICE" 2>/dev/null || echo "unknown")
    limit_interval=$(systemctl show -p StartLimitIntervalUSec --value "$SERVICE" 2>/dev/null || echo "unknown")
    log_info "StartLimitBurst: $burst | StartLimitInterval: $limit_interval"

    log_info "Simulating OOM by sending SIGKILL to yunque-agent process..."
    kill -9 "$pre_pid" 2>/dev/null || {
        log_fail "Cannot kill process $pre_pid (permission denied?)"
        return 1
    }
    log_info "SIGKILL sent. Waiting for systemd to restart service..."

    local recovered=false
    for attempt in $(seq 1 12); do
        sleep 5
        if systemctl is-active --quiet "$SERVICE" 2>/dev/null; then
            local post_pid
            post_pid=$(systemctl show -p MainPID --value "$SERVICE" 2>/dev/null)
            if [[ "$post_pid" != "0" && "$post_pid" != "$pre_pid" ]]; then
                recovered=true
                log_pass "Service restarted with new PID: $post_pid (was $pre_pid) in ~$((attempt * 5))s"
                break
            fi
        fi
        log_info "Attempt $attempt/12 — service not yet ready..."
    done

    if ! $recovered; then
        log_fail "Service did not auto-restart within 60 seconds"
        log_info "Check: journalctl -u $SERVICE -n 50 --no-pager"
        return 1
    fi

    sleep 3
    log_info "Verifying health endpoint post-recovery..."
    local health_status
    health_status=$(curl -sf -o /dev/null -w "%{http_code}" "$AGENT_URL/healthz" 2>/dev/null || echo "000")
    if [[ "$health_status" == "200" ]]; then
        log_pass "/healthz returned 200 after OOM recovery"
    else
        log_fail "/healthz returned $health_status after OOM recovery"
    fi

    local chat_status
    chat_status=$(curl -sf -o /dev/null -w "%{http_code}" \
        -X POST "$AGENT_URL/v1/chat" \
        -H "Content-Type: application/json" \
        -d '{"messages":[{"role":"user","content":"hello"}],"session_id":"oom-recovery-test"}' \
        2>/dev/null || echo "000")
    if [[ "$chat_status" == "200" ]]; then
        log_pass "Chat API functional after OOM recovery"
    else
        log_warn "Chat API returned $chat_status after OOM recovery (may need API key)"
    fi
}

# ─────────────── Check 3: Health Check ───────────────

check_health() {
    header "Check: Health Check & Monitoring Endpoints"

    local health_status
    health_status=$(curl -sf -o /dev/null -w "%{http_code}" "$AGENT_URL/healthz" 2>/dev/null || echo "000")
    if [[ "$health_status" == "200" ]]; then
        log_pass "/healthz returns 200"
    else
        log_fail "/healthz returns $health_status"
    fi

    local version_status
    version_status=$(curl -sf -o /dev/null -w "%{http_code}" "$AGENT_URL/v1/version" 2>/dev/null || echo "000")
    if [[ "$version_status" == "200" ]]; then
        local version_body
        version_body=$(curl -sf "$AGENT_URL/v1/version" 2>/dev/null || echo "{}")
        log_pass "/v1/version returns 200: $version_body"
    else
        log_warn "/v1/version returns $version_status"
    fi

    log_info "Testing health check under rapid polling (50 requests)..."
    local health_ok=0
    local health_err=0
    for _ in $(seq 1 50); do
        local code
        code=$(curl -sf -o /dev/null -w "%{http_code}" "$AGENT_URL/healthz" 2>/dev/null || echo "000")
        if [[ "$code" == "200" ]]; then
            ((health_ok++))
        else
            ((health_err++))
        fi
    done
    log_info "Health poll results: OK=$health_ok FAIL=$health_err"
    if (( health_err == 0 )); then
        log_pass "All 50 rapid health checks passed"
    elif (( health_err < 3 )); then
        log_warn "$health_err/50 health checks failed (intermittent)"
    else
        log_fail "$health_err/50 health checks failed"
    fi

    if [[ -f "/opt/yunque-agent/healthcheck.sh" ]]; then
        log_pass "Cron health check script exists"
    else
        log_warn "Cron health check script not found at /opt/yunque-agent/healthcheck.sh"
    fi

    if crontab -l 2>/dev/null | grep -q "healthcheck"; then
        log_pass "Health check cron job is configured"
    else
        log_warn "No health check cron job found"
    fi
}

# ─────────────── Check 4: Runtime Memory Monitor ───────────────

check_memory_runtime() {
    header "Check: Runtime Memory Monitoring (60s)"

    if ! systemctl is-active --quiet "$SERVICE" 2>/dev/null; then
        log_fail "Service must be running for memory monitoring"
        return 1
    fi

    local pid
    pid=$(systemctl show -p MainPID --value "$SERVICE" 2>/dev/null)

    log_info "Monitoring PID $pid for 60 seconds (5s intervals)..."
    echo ""
    printf "  %-8s %-10s %-10s %-10s %-8s\n" "Time" "RSS(MiB)" "VSZ(MiB)" "Threads" "%MEM"
    printf "  %-8s %-10s %-10s %-10s %-8s\n" "────" "────────" "────────" "───────" "────"

    local max_rss=0
    local min_rss=999999
    local sum_rss=0
    local count=0

    for tick in $(seq 1 12); do
        if ! kill -0 "$pid" 2>/dev/null; then
            log_warn "Process $pid disappeared at tick $tick"
            break
        fi

        local rss vsz threads pmem
        read -r rss vsz threads pmem < <(ps -o rss=,vsz=,nlwp=,%mem= -p "$pid" 2>/dev/null || echo "0 0 0 0.0")

        local rss_mib=$(( rss / 1024 ))
        local vsz_mib=$(( vsz / 1024 ))

        printf "  %-8s %-10s %-10s %-10s %-8s\n" \
            "${tick}×5s" "${rss_mib}M" "${vsz_mib}M" "$threads" "${pmem}%"

        (( rss_mib > max_rss )) && max_rss=$rss_mib
        (( rss_mib < min_rss )) && min_rss=$rss_mib
        sum_rss=$(( sum_rss + rss_mib ))
        ((count++))

        sleep 5
    done

    echo ""
    if (( count > 0 )); then
        local avg_rss=$(( sum_rss / count ))
        log_info "Memory stats: Min=${min_rss}M Avg=${avg_rss}M Max=${max_rss}M"

        if (( max_rss < 400 )); then
            log_pass "Peak RSS (${max_rss}M) within GOMEMLIMIT=400MiB target"
        elif (( max_rss < 512 )); then
            log_warn "Peak RSS (${max_rss}M) between 400-512M — GC pressure expected"
        else
            log_fail "Peak RSS (${max_rss}M) exceeds 512M — risk of cgroup OOM kill"
        fi
    fi

    local total_mem free_mem
    total_mem=$(awk '/MemTotal/ {print int($2/1024)}' /proc/meminfo 2>/dev/null || echo "0")
    free_mem=$(awk '/MemAvailable/ {print int($2/1024)}' /proc/meminfo 2>/dev/null || echo "0")
    log_info "System memory: Total=${total_mem}M Available=${free_mem}M"

    if (( free_mem < 200 )); then
        log_fail "Available memory (${free_mem}M) critically low"
    elif (( free_mem < 500 )); then
        log_warn "Available memory (${free_mem}M) is low"
    else
        log_pass "Available memory (${free_mem}M) is healthy"
    fi

    if swapon --show 2>/dev/null | grep -q '/'; then
        log_pass "Swap is configured"
        local swap_used
        swap_used=$(awk '/SwapTotal/ {t=$2} /SwapFree/ {f=$2} END {print int((t-f)/1024)}' /proc/meminfo 2>/dev/null || echo "0")
        log_info "Swap used: ${swap_used}M"
    else
        log_warn "No swap configured — OOM risk higher without swap safety net"
    fi
}

# ─────────────── Main ───────────────

echo "╔══════════════════════════════════════════════════════╗"
echo "║  yunque-agent Stability Test Suite (2G Server)      ║"
echo "╠══════════════════════════════════════════════════════╣"
echo "║  Target: $AGENT_URL"
echo "║  Service: $SERVICE"
echo "║  Date: $(date '+%Y-%m-%d %H:%M:%S')"
echo "╚══════════════════════════════════════════════════════╝"

SCENARIO="${1:-all}"

case "$SCENARIO" in
    cgroup)
        check_cgroup
        ;;
    oom)
        check_oom_recovery
        ;;
    health)
        check_health
        ;;
    memory)
        check_memory_runtime
        ;;
    all)
        check_cgroup
        check_health
        check_memory_runtime
        echo ""
        echo "─────────────────────────────────────────────"
        echo "  NOTE: OOM test skipped in 'all' mode"
        echo "  Run: sudo ./scripts/stress-test.sh oom"
        echo "─────────────────────────────────────────────"
        ;;
    *)
        echo "Unknown scenario: $SCENARIO"
        echo "Usage: $0 [cgroup|oom|health|memory|all]"
        exit 1
        ;;
esac

echo ""
echo "═══════════════════════════════════════════════"
echo "  Final Score"
echo "═══════════════════════════════════════════════"
echo -e "  ${GREEN}PASS: $PASS${NC}  |  ${YELLOW}WARN: $WARN${NC}  |  ${RED}FAIL: $FAIL${NC}"

if (( FAIL > 0 )); then
    echo -e "\n  ${RED}⚠ $FAIL check(s) failed — review issues above${NC}"
    exit 1
elif (( WARN > 0 )); then
    echo -e "\n  ${YELLOW}△ All critical checks passed with $WARN warning(s)${NC}"
    exit 0
else
    echo -e "\n  ${GREEN}✓ All checks passed${NC}"
    exit 0
fi
