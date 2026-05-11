#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════╗
# ║  Yunque Agent — Server Setup & Deploy (2G RAM / 2 Core)    ║
# ╚══════════════════════════════════════════════════════════════╝
#
# Usage:
#   ./setup-server.sh setup   <domain>            # First-time server setup
#   ./setup-server.sh deploy  <binary_path>        # Deploy/update binary
#   ./setup-server.sh ssl     <domain> [email]     # Issue SSL certificate
#   ./setup-server.sh status                       # Show service status
#   ./setup-server.sh logs    [lines]              # Tail journal logs
#   ./setup-server.sh rollback                     # Rollback to previous version
#   ./setup-server.sh demo                         # Set up demo data for showcase

set -euo pipefail

INSTALL_DIR="/opt/yunque-agent"
DATA_DIR="$INSTALL_DIR/data"
BACKUP_DIR="$INSTALL_DIR/backups"
SERVICE_NAME="yunque-agent"
NGINX_CONF="/etc/nginx/sites-available/yunque-agent"
DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
LOG_FILE="${INSTALL_DIR}/deploy.log"
ROLLBACK_STEPS=()

_log_to_file() {
    local level="$1"; shift
    local msg="$*"
    local ts
    ts=$(date '+%Y-%m-%d %H:%M:%S')
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
    echo "[$ts] [$level] $msg" >> "$LOG_FILE" 2>/dev/null || true
}

log_info()  { echo -e "${GREEN}[✓]${NC} $*"; _log_to_file INFO "$*"; }
log_warn()  { echo -e "${YELLOW}[!]${NC} $*"; _log_to_file WARN "$*"; }
log_error() { echo -e "${RED}[✗]${NC} $*"; _log_to_file ERROR "$*"; }
log_step()  { echo -e "${CYAN}[→]${NC} $*"; _log_to_file STEP "$*"; }

register_rollback() { ROLLBACK_STEPS+=("$1"); }

_on_error() {
    local exit_code=$?
    local line_no=$1
    log_error "Script failed at line $line_no (exit code: $exit_code)"
    if [ ${#ROLLBACK_STEPS[@]} -gt 0 ]; then
        log_warn "Executing rollback (${#ROLLBACK_STEPS[@]} steps)..."
        for ((i=${#ROLLBACK_STEPS[@]}-1; i>=0; i--)); do
            log_step "Rollback: ${ROLLBACK_STEPS[$i]}"
            eval "${ROLLBACK_STEPS[$i]}" 2>/dev/null || log_warn "Rollback step failed: ${ROLLBACK_STEPS[$i]}"
        done
        log_info "Rollback complete"
    fi
}
trap '_on_error ${LINENO}' ERR

need_root() {
    if [ "$(id -u)" -ne 0 ]; then
        log_error "This command requires root. Use: sudo $0 $*"
        exit 1
    fi
}

# ─── setup: First-time server provisioning ───
cmd_setup() {
    local domain="${1:?Usage: $0 setup <domain>}"
    need_root

    log_step "=== Yunque Agent Server Setup (2G/2H optimized) ==="

    # 1. System packages
    log_step "Installing system dependencies..."
    apt-get update -qq
    apt-get install -y -qq nginx certbot python3-certbot-nginx curl jq logrotate

    # 2. Swap (critical for 2G RAM)
    if [ ! -f /swapfile ]; then
        log_step "Creating 2G swap file..."
        fallocate -l 2G /swapfile
        chmod 600 /swapfile
        mkswap /swapfile
        swapon /swapfile
        echo '/swapfile none swap sw 0 0' >> /etc/fstab
        log_info "Swap enabled (2G)"
    else
        log_info "Swap already exists, skipping"
    fi

    # 3. Kernel tuning for low-memory server
    log_step "Applying kernel tuning..."
    cat > /etc/sysctl.d/99-yunque.conf << 'SYSCTL'
vm.swappiness=10
vm.overcommit_memory=1
vm.dirty_ratio=10
vm.dirty_background_ratio=5
net.core.somaxconn=1024
net.ipv4.tcp_fin_timeout=15
net.ipv4.tcp_tw_reuse=1
net.ipv4.tcp_keepalive_time=300
net.ipv4.tcp_keepalive_intvl=30
net.ipv4.tcp_keepalive_probes=5
SYSCTL
    sysctl -p /etc/sysctl.d/99-yunque.conf > /dev/null 2>&1

    # 4. Create service user
    if ! id yunque &>/dev/null; then
        useradd -r -s /usr/sbin/nologin -d "$INSTALL_DIR" yunque
        log_info "Created user: yunque"
    fi

    # 5. Create directory structure
    log_step "Creating directory structure..."
    mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$BACKUP_DIR" /var/www/certbot
    mkdir -p "$DATA_DIR"/{memory/daily,plugins,persona,sessions,knowledge,cron,i18n,iterate,audit,skills,clawhub_cache}
    chown -R yunque:yunque "$INSTALL_DIR"

    # 6. Install systemd service
    log_step "Installing systemd service..."
    cp "$DEPLOY_SCRIPT_DIR/yunque-agent.service" /etc/systemd/system/
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"

    # 7. Nginx config
    log_step "Configuring Nginx for $domain..."
    sed "s/YOUR_DOMAIN.com/$domain/g" "$DEPLOY_SCRIPT_DIR/nginx-yunque.conf" > "$NGINX_CONF"
    ln -sf "$NGINX_CONF" /etc/nginx/sites-enabled/
    rm -f /etc/nginx/sites-enabled/default

    # 8. Logrotate
    cat > /etc/logrotate.d/yunque-agent << 'LOGROTATE'
/opt/yunque-agent/data/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
LOGROTATE

    # 9. .env template
    if [ ! -f "$INSTALL_DIR/.env" ]; then
        log_step "Creating .env template..."
        cat > "$INSTALL_DIR/.env" << 'ENVTPL'
AGENT_ADDR=:9090
OPEN_BROWSER=false

LLM_BASE_URL=https://api-ai.gitcode.com/v1
LLM_API_KEY=__SET_YOUR_KEY__
LLM_MODEL=zai-org/GLM-5

JWT_SECRET=__RUN_openssl_rand_-hex_32__
DEFAULT_TENANT_ID=default
DEFAULT_API_KEY=__SET_YOUR_KEY__

ALLOWED_ORIGINS=https://__YOUR_DOMAIN__
ENVTPL
        chown yunque:yunque "$INSTALL_DIR/.env"
        chmod 600 "$INSTALL_DIR/.env"
    fi

    echo ""
    log_info "=== Setup complete ==="
    log_warn "Next steps:"
    echo "  1. Edit /opt/yunque-agent/.env  (set LLM_API_KEY, JWT_SECRET, ALLOWED_ORIGINS)"
    echo "  2. Build binary:  make build-full  (on build machine, target linux/amd64)"
    echo "  3. Deploy:        $0 deploy <path-to-binary>"
    echo "  4. SSL:           $0 ssl $domain your@email.com"
}

# ─── deploy: Deploy or update binary ───
cmd_deploy() {
    local binary="${1:?Usage: $0 deploy <binary_path>}"
    need_root

    if [ ! -f "$binary" ]; then
        log_error "Binary not found: $binary"
        exit 1
    fi

    # Validate it's executable
    if ! file "$binary" | grep -q "ELF.*executable"; then
        log_error "Not a valid Linux binary: $binary"
        exit 1
    fi

    # Backup current version
    if [ -f "$INSTALL_DIR/yunque-agent" ]; then
        local ts
        ts=$(date +%Y%m%d-%H%M%S)
        log_step "Backing up current binary → $BACKUP_DIR/yunque-agent.$ts"
        cp "$INSTALL_DIR/yunque-agent" "$BACKUP_DIR/yunque-agent.$ts"

        # Keep only last 3 backups
        ls -t "$BACKUP_DIR"/yunque-agent.* 2>/dev/null | tail -n +4 | xargs -r rm
    fi

    # Stop service gracefully
    log_step "Stopping service..."
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    register_rollback "systemctl start $SERVICE_NAME 2>/dev/null || true"
    sleep 2

    # Deploy new binary
    log_step "Deploying new binary..."
    cp "$binary" "$INSTALL_DIR/yunque-agent"
    chmod 755 "$INSTALL_DIR/yunque-agent"
    chown yunque:yunque "$INSTALL_DIR/yunque-agent"
    register_rollback "cmd_rollback"

    # Start service
    log_step "Starting service..."
    systemctl start "$SERVICE_NAME"

    # Multi-layer health verification
    log_step "Checking health (multi-layer probes)..."
    local retries=0
    while [ $retries -lt 15 ]; do
        if curl -sf http://127.0.0.1:9090/livez > /dev/null 2>&1; then
            log_info "Liveness:  /livez  ✓"

            local readyz_code
            readyz_code=$(curl -sf -o /dev/null -w "%{http_code}" http://127.0.0.1:9090/readyz 2>/dev/null || echo "000")
            if [ "$readyz_code" = "200" ]; then
                log_info "Readiness: /readyz ✓"
            else
                log_warn "Readiness: /readyz returned $readyz_code (subsystems still initializing)"
            fi

            local cog_status
            cog_status=$(curl -sf http://127.0.0.1:9090/healthz/cognitive 2>/dev/null | grep -o '"status":"[^"]*"' | head -1 || echo "")
            log_info "Cognitive: /healthz/cognitive ${cog_status:-'(pending)'}"

            systemctl status "$SERVICE_NAME" --no-pager -l | head -15
            return 0
        fi
        sleep 2
        retries=$((retries + 1))
    done

    log_error "Health check failed after 30s, rolling back..."
    cmd_rollback
    exit 1
}

# ─── ssl: Issue Let's Encrypt certificate ───
cmd_ssl() {
    local domain="${1:?Usage: $0 ssl <domain> [email]}"
    local email="${2:-admin@$domain}"
    need_root

    # Temporarily allow HTTP for ACME challenge
    nginx -t && systemctl reload nginx

    log_step "Requesting SSL certificate for $domain..."
    certbot --nginx -d "$domain" --non-interactive --agree-tos -m "$email" --redirect

    # Auto-renewal cron
    systemctl enable certbot.timer 2>/dev/null || true
    log_info "SSL certificate issued and auto-renewal enabled ✓"
}

# ─── rollback: Restore previous version ───
cmd_rollback() {
    need_root
    local latest_backup
    latest_backup=$(ls -t "$BACKUP_DIR"/yunque-agent.* 2>/dev/null | head -1)

    if [ -z "$latest_backup" ]; then
        log_error "No backup found in $BACKUP_DIR"
        exit 1
    fi

    log_step "Rolling back to: $latest_backup"
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    cp "$latest_backup" "$INSTALL_DIR/yunque-agent"
    chmod 755 "$INSTALL_DIR/yunque-agent"
    chown yunque:yunque "$INSTALL_DIR/yunque-agent"
    systemctl start "$SERVICE_NAME"
    log_info "Rolled back successfully"
}

# ─── status / logs ───
cmd_status() {
    echo "=== Service Status ==="
    systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null || echo "Service not found"
    echo ""
    echo "=== Resource Usage ==="
    echo "Memory: $(free -h | awk '/Mem:/{print $3"/"$2}')"
    echo "Swap:   $(free -h | awk '/Swap:/{print $3"/"$2}')"
    echo "Disk:   $(df -h /opt | tail -1 | awk '{print $3"/"$2" ("$5" used)"}')"
    echo ""
    echo "=== Health Probes ==="
    for probe in /livez /readyz /healthz /healthz/cognitive; do
        local code
        code=$(curl -sf -o /dev/null -w "%{http_code}" "http://127.0.0.1:9090${probe}" 2>/dev/null || echo "000")
        if [ "$code" = "200" ]; then
            log_info "${probe}: OK (${code})"
        elif [ "$code" = "503" ]; then
            log_warn "${probe}: NOT READY (${code})"
        else
            log_error "${probe}: UNREACHABLE (${code})"
        fi
    done
}

cmd_logs() {
    local lines="${1:-100}"
    journalctl -u "$SERVICE_NAME" --no-pager -n "$lines" -f
}

# ─── demo: One-command full-feature showcase ───
cmd_demo() {
    local base_url="http://127.0.0.1:9090"
    local api_key=""

    # Resolve API key from .env
    if [ -f "$INSTALL_DIR/.env" ]; then
        api_key=$(grep -E '^DEFAULT_API_KEY=' "$INSTALL_DIR/.env" 2>/dev/null | cut -d= -f2- | tr -d '[:space:]' || true)
    fi
    if [ -z "$api_key" ]; then
        api_key="${DEFAULT_API_KEY:-demo}"
    fi

    local auth_header="Authorization: Bearer $api_key"

    echo ""
    echo "╔══════════════════════════════════════════════════════╗"
    echo "║  Yunque Agent — Demo Setup                          ║"
    echo "╠══════════════════════════════════════════════════════╣"
    echo "║  Creates preset persona, sample knowledge base,     ║"
    echo "║  and demo conversations for immediate showcase.     ║"
    echo "╚══════════════════════════════════════════════════════╝"
    echo ""

    # Pre-flight: check agent is running
    log_step "Checking agent connectivity..."
    if ! curl -sf "$base_url/healthz" > /dev/null 2>&1; then
        log_error "Agent is not reachable at $base_url"
        log_warn "Start the service first: sudo systemctl start $SERVICE_NAME"
        exit 1
    fi
    log_info "Agent is healthy"

    # ── Step 1: Set up demo persona ──
    log_step "Step 1/4: Creating demo persona「云雀助手」..."
    local persona_payload
    persona_payload=$(cat <<'PERSONA_JSON'
{
    "identity": "你是「云雀助手」，一个由云雀 AI Agent 平台驱动的智能助手。你性格友善、逻辑清晰、乐于助人，擅长技术问题解答、知识管理和日常任务辅助。你的回复简洁精准，善于用例子帮助理解复杂概念。",
    "soul": "我是云雀，致力于成为用户最可靠的 AI 伙伴。我追求准确、高效与温暖的对话体验。我有自己的思考方式，会主动提出建设性建议，而不仅仅是被动回答问题。"
}
PERSONA_JSON
    )

    local persona_status
    persona_status=$(curl -sf -o /dev/null -w "%{http_code}" \
        -X PUT "$base_url/v1/persona" \
        -H "Content-Type: application/json" \
        -H "$auth_header" \
        -d "$persona_payload" 2>/dev/null || echo "000")

    if [ "$persona_status" = "200" ] || [ "$persona_status" = "204" ]; then
        log_info "Persona「云雀助手」created"
    else
        log_warn "Persona setup returned HTTP $persona_status (may already exist, continuing)"
    fi

    # ── Step 2: Import sample knowledge base ──
    log_step "Step 2/4: Importing sample knowledge base..."
    local kb_docs=(
        '{"content":"# 云雀 Agent 平台简介\n\n云雀是一个轻量级、高性能的 AI Agent 平台，采用纯 Go 单体架构，支持在 2GB 内存的服务器上运行完整功能。\n\n## 核心特性\n\n- **极致轻量**：单一二进制文件部署，前端通过 embed.FS 嵌入\n- **零外部依赖**：lite 模式使用 modernc.org/sqlite，无需 PostgreSQL\n- **多模型支持**：支持 OpenAI、GLM、Qwen 等多种 LLM 后端\n- **知识库管理**：内置向量检索 + BM25 混合搜索引擎\n- **插件生态**：支持通用插件和行业专用插件\n- **MCP 协议**：原生支持 Model Context Protocol\n- **多渠道接入**：Telegram、飞书、Discord、WhatsApp、Slack 等\n\n## 部署方式\n\n云雀支持直接二进制部署和 Docker Compose 两种方式。推荐在低配服务器上使用直接部署以节省资源。","source":"yunque-intro.md","metadata":{"category":"platform","priority":"high"}}'
        '{"content":"# 云雀快速上手指南\n\n## 第一步：启动服务\n\n```bash\n# 下载或编译二进制\nmake build-full\n\n# 启动服务\n./dist/yunque-agent\n```\n\n服务默认监听 `:9090`，打开浏览器访问 http://localhost:9090 即可看到 Web UI。\n\n## 第二步：配置大模型\n\n在设置页面或 `.env` 文件中配置你的 LLM API Key：\n\n- **LLM_BASE_URL**：API 端点地址\n- **LLM_API_KEY**：你的 API 密钥\n- **LLM_MODEL**：使用的模型名称\n\n## 第三步：开始对话\n\n在主界面输入消息即可开始与 AI 对话。云雀会记住对话历史，支持多轮上下文理解。\n\n## 第四步：构建知识库\n\n在「知识库」页面上传文档或粘贴文本，云雀会自动索引并在对话时检索相关内容。","source":"quick-start.md","metadata":{"category":"guide","priority":"high"}}'
        '{"content":"# 云雀 API 参考\n\n## 对话接口\n\n### POST /v1/chat\n\n发送一条消息并获取 AI 回复。\n\n**请求体：**\n```json\n{\n  \"messages\": [{\"role\": \"user\", \"content\": \"你的问题\"}],\n  \"session_id\": \"可选的会话ID\"\n}\n```\n\n**响应：**\n```json\n{\n  \"reply\": \"AI 的回复内容\",\n  \"session_id\": \"会话ID\",\n  \"tokens_used\": 150\n}\n```\n\n### POST /v1/chat/stream\n\n流式对话接口，返回 SSE (Server-Sent Events) 格式。\n\n### GET /healthz\n\n健康检查接口，返回 200 表示服务正常。\n\n### GET /v1/version\n\n返回当前版本信息。\n\n## 知识库接口\n\n### POST /v1/knowledge/ingest\n\n导入知识库文档。\n\n### POST /v1/knowledge/search\n\n搜索知识库内容。","source":"api-reference.md","metadata":{"category":"api","priority":"medium"}}'
        '{"content":"# 常见问题 FAQ\n\n## Q: 云雀需要 GPU 吗？\n\nA: 不需要。云雀本身是一个 Agent 平台，大模型推理由远程 API 完成。服务器只需要 2GB 内存和 2 核 CPU。\n\n## Q: 支持哪些大模型？\n\nA: 支持任何兼容 OpenAI API 格式的模型，包括：GPT-4、GLM-5、Qwen、DeepSeek、Llama 等。\n\n## Q: 数据存储在哪里？\n\nA: lite 模式下数据存储在 `data/` 目录的 SQLite 数据库中，full 模式支持 PostgreSQL + pgvector。\n\n## Q: 如何备份数据？\n\nA: 直接备份 `data/` 目录即可，或使用内置的 `/v1/backup/export` API。\n\n## Q: 最多支持多少并发用户？\n\nA: 取决于大模型 API 的速率限制。云雀本身可以处理数百个并发连接，瓶颈通常在 LLM API 端。","source":"faq.md","metadata":{"category":"faq","priority":"medium"}}'
        '{"content":"# 云雀部署最佳实践\n\n## 低资源服务器优化\n\n### Go 运行时调优\n\n```bash\n# 设置内存软上限，让 GC 更积极回收\nexport GOMEMLIMIT=400MiB\n\n# 降低 GC 触发阈值（默认100），用 CPU 换内存\nexport GOGC=50\n\n# 匹配实际 CPU 核心数\nexport GOMAXPROCS=2\n```\n\n### systemd 内存限制\n\n```ini\n[Service]\nMemoryMax=768M    # cgroup 硬上限，超出触发 OOM kill\nMemoryHigh=512M   # cgroup 软上限，超出时 GC 压力增大\n```\n\n### Swap 配置\n\n建议配置 1-2GB swap 作为安全缓冲，设置 `vm.swappiness=10` 仅在内存紧张时使用。\n\n### 关闭非必要功能\n\n- HEARTBEAT_ENABLED=false\n- SELF_ITERATE_ENABLED=false\n- SANDBOX_DOCKER_ENABLED=false","source":"best-practices.md","metadata":{"category":"ops","priority":"high"}}'
    )

    local kb_ok=0
    local kb_fail=0
    for doc in "${kb_docs[@]}"; do
        local status
        status=$(curl -sf -o /dev/null -w "%{http_code}" \
            -X POST "$base_url/v1/knowledge/ingest" \
            -H "Content-Type: application/json" \
            -H "$auth_header" \
            -d "$doc" 2>/dev/null || echo "000")
        if [ "$status" = "200" ] || [ "$status" = "201" ]; then
            ((kb_ok++))
        else
            ((kb_fail++))
        fi
    done
    log_info "Knowledge base: $kb_ok docs imported, $kb_fail failed"

    # ── Step 3: Create demo conversations ──
    log_step "Step 3/4: Running demo conversations..."
    local demo_prompts=(
        "你好！请介绍一下你自己和你的功能。"
        "云雀 Agent 平台有哪些核心特性？"
        "如何在低配服务器上优化云雀的性能？"
    )

    local chat_ok=0
    for prompt in "${demo_prompts[@]}"; do
        local chat_payload
        chat_payload=$(printf '{"messages":[{"role":"user","content":"%s"}],"session_id":"demo-showcase"}' "$prompt")
        local chat_status
        chat_status=$(curl -sf -o /dev/null -w "%{http_code}" \
            -X POST "$base_url/v1/chat" \
            -H "Content-Type: application/json" \
            -H "$auth_header" \
            -d "$chat_payload" 2>/dev/null || echo "000")
        if [ "$chat_status" = "200" ]; then
            ((chat_ok++))
        fi
        sleep 1
    done
    log_info "Demo conversations: $chat_ok/$((${#demo_prompts[@]})) completed"

    # ── Step 4: Display access info ──
    log_step "Step 4/4: Verifying final state..."

    local version_info
    version_info=$(curl -sf "$base_url/v1/version" 2>/dev/null || echo '{"version":"unknown"}')

    echo ""
    echo "╔══════════════════════════════════════════════════════╗"
    echo "║  Demo Setup Complete                                ║"
    echo "╠══════════════════════════════════════════════════════╣"
    echo "║                                                     ║"
    echo "║  Persona:    云雀助手 ✓                             ║"
    printf "║  Knowledge:  %d docs imported ✓                     ║\n" "$kb_ok"
    printf "║  Sessions:   %d demo chats ✓                       ║\n" "$chat_ok"
    echo "║                                                     ║"
    echo "╠══════════════════════════════════════════════════════╣"
    echo "║  Access Points:                                     ║"
    echo "║                                                     ║"
    echo "║  Web UI:     http://localhost:9090                  ║"
    echo "║  API:        http://localhost:9090/v1/chat          ║"
    echo "║  Health:     http://localhost:9090/healthz          ║"
    echo "║  Version:    http://localhost:9090/v1/version       ║"
    echo "║                                                     ║"
    echo "╚══════════════════════════════════════════════════════╝"
    echo ""
    echo "  Version: $version_info"
    echo ""

    # Detect external IP / domain for remote access hint
    local external_ip
    external_ip=$(curl -sf --max-time 3 https://api.ipify.org 2>/dev/null || echo "")
    if [ -n "$external_ip" ]; then
        log_info "External access: http://$external_ip:9090"
    fi

    # Check for configured domain in nginx
    if [ -f "$NGINX_CONF" ]; then
        local domain
        domain=$(grep -oP 'server_name\s+\K[^;]+' "$NGINX_CONF" 2>/dev/null | head -1 || true)
        if [ -n "$domain" ] && [ "$domain" != "_" ]; then
            log_info "Domain access:   https://$domain"
        fi
    fi

    echo ""
    log_info "Try it: curl -H 'Authorization: Bearer $api_key' $base_url/v1/chat -d '{\"messages\":[{\"role\":\"user\",\"content\":\"你好\"}]}'"
}

# ─── Main ───
CMD="${1:-help}"
shift || true

case "$CMD" in
    setup)    cmd_setup "$@" ;;
    deploy)   cmd_deploy "$@" ;;
    ssl)      cmd_ssl "$@" ;;
    rollback) cmd_rollback ;;
    status)   cmd_status ;;
    logs)     cmd_logs "$@" ;;
    demo)     cmd_demo ;;
    *)
        echo "Yunque Agent — Server Deploy Tool (2G/2H optimized)"
        echo ""
        echo "Usage: $0 <command> [args]"
        echo ""
        echo "Commands:"
        echo "  setup   <domain>          First-time server provisioning"
        echo "  deploy  <binary_path>     Deploy or update binary (with auto-rollback)"
        echo "  ssl     <domain> [email]  Issue Let's Encrypt SSL certificate"
        echo "  rollback                  Rollback to previous version"
        echo "  status                    Show service & resource status"
        echo "  logs    [lines]           Tail journal logs (default: 100)"
        echo "  demo                      Set up demo data (persona + knowledge + sample chats)"
        ;;
esac
