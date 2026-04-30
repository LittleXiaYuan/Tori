# 云雀 Agent (Yunque Agent)

开箱即用的 AI Agent 平台 — 双击运行，自动打开浏览器，零依赖。

Production-ready AI Agent with embedded WebUI, multi-model routing, self-iteration, and 90+ API endpoints.

> **Language Note** — Code comments, godoc, and architecture docs are in English.
> Log messages, system prompts, and user-facing strings are in Chinese, reflecting the primary target market.
> Contributions in either language are welcome.

## 仓库结构说明

| 目录 | 说明 |
|------|------|
| `heroui-web/` | **当前主前端**（HeroUI + Next.js 16），Go 嵌入 `heroui-web/out/` |
| `web/` | 已归档旧前端，不再维护，见 `web/README.md` |
| `docs/` | 面向用户的正式文档与文档站（VitePress） |
| `doc/` | 内部开发文档、连续性记录、设计蓝图（`.gitignore` 排除） |
| `browser-extension/` | 浏览器连接器扩展（Chrome/Edge） |
| `data/plugins/` | 第三方插件热加载目录 |
| `data/skills/` | 文件技能热加载目录 |

详细结构见 `docs/repo-layout.md`。

## 30 秒上手

```bash
cp .env.example .env       # ① 复制配置
# ② 编辑 .env，填入 LLM_API_KEY
go run ./cmd/agent         # ③ 启动
# ④ 浏览器自动打开 → http://localhost:9090/chat
```

仅需配置一个 `LLM_API_KEY` 即可开始对话。

## 更多启动方式

<details>
<summary>图形化安装向导</summary>

```bash
go run ./cmd/setup         # 网页引导配置 .env
go run ./cmd/agent
```
</details>

<details>
<summary>编译后运行（推荐发布构建）</summary>

```bash
make build-full            # 构建前端 + Go 二进制（强制校验前端产物）
./dist/yunque-agent        # 自动开浏览器
```

> **开发构建**（跳过前端）：`go build -o yunque-agent ./cmd/agent`，仅用于开发调试，不含最新前端。
</details>

<details>
<summary>Docker 一键部署</summary>

```bash
cp .env.example .env       # ① 复制配置
# ② 编辑 .env，填入 LLM_API_KEY 和 JWT_SECRET
#    JWT_SECRET 生成: openssl rand -hex 32

# 方式一：一键脚本（推荐）
./scripts/deploy.sh              # 轻量版（默认，内嵌 SQLite）
./scripts/deploy.sh prod         # 完整版（PostgreSQL + pgvector）
./scripts/deploy.sh dev          # 开发模式（前台运行，实时日志）
./scripts/deploy.sh stop         # 停止所有服务
./scripts/deploy.sh status       # 查看服务状态
./scripts/deploy.sh logs         # 查看日志

# 方式二：直接 docker compose
docker compose --profile lite up -d    # 轻量版
docker compose --profile full up -d    # 完整版（需额外设 POSTGRES_PASSWORD）
```

Dashboard: http://localhost:9090
</details>

<details>
<summary>跨平台构建</summary>

```bash
make release   # 生成 Windows/macOS/Linux (amd64+arm64) 6 个二进制
```
</details>

## 架构

```
┌─────────────────────────────────────────────────────────┐
│   Control Plane (Gateway) — 90+ API endpoints        │
│   Embedded WebUI · JWT/APIKey · CORS · Rate Limit     │
│   SSE Stream · WebSocket · Prometheus · Audit Chain    │
├─────────────────────────────────────────────────────────┤
│   Agent Core  (internal/agentcore/)                   │
│   Planner (multi-step, parallel FC)                   │
│   LLM Pool (fast/smart/expert) · Smart Router         │
│   Context Window · Session Provider Override           │
├─────────┬────────────┬───────────┬────────┬───────────┤
│ Memory  │ Knowledge  │ Guardrail │ Trust  │ Persona   │
│ 5-layer │ Graph+RAG  │ PII/Inj   │ Score  │ Emotion   │
│ Orch    │ Embeddings │ Moderate  │ 0→100  │ Identity  │
├─────────┴────────────┴───────────┴────────┴───────────┤
│   Execution Layer                                     │
│   Sandbox · Scheduler · Cron · Tools Process Manager  │
│   Channels: TG/Feishu/Discord/Slack/WA/Signal/Email   │
│            QQ/WeCom/DingTalk/WeChatOA/LINE/Kook/Satori│
│            + WebChat (HTTP gateway)                    │
├─────────────────────────────────────────────────────────┤
│   Skills & Plugins                                    │
│   SkillHub (ClawHub) · Marketplace · Hot-load         │
├─────────────────────────────────────────────────────────┤
│   Experimental  (internal/experimental/)              │
│   ReAct · Eval · Iterate · SkillGrow · Distill        │
│   Curiosity · Causal · World Model · MetaCog · Trait  │
├─────────────────────────────────────────────────────────┤
│   Storage: Embedded SQLite (modernc.org/sqlite)       │
│           Ledger KV (~25 namespaces) / Postgres opt.  │
│   Federation Hub · Multi-Agent Runtime Pool            │
└─────────────────────────────────────────────────────────┘
```

## 核心特性

- **开箱即用**: 单二进制，零依赖，自动开浏览器，嵌入式 WebUI
- **多模型路由**: Fast/Smart/Expert 三层池 + 智能路由 + 断路器 + Fallback
- **多LLM Provider**: 多厂商Provider注册中心，密钥轮换，会话级模型覆盖
- **5层记忆**: Short/Mid/Long + 知识图谱 + 可编辑记忆，统一召回
- **知识库 RAG**: 文件导入 + 混合检索 (BM25稀疏 + 向量密集 + RRF融合 + 可选Rerank二阶排序)
- **上下文压缩**: 多阶段压缩管线 (轮数限制 → LLM摘要 → 紧急减半)
- **富消息组件**: 15种消息组件 (文本/图片/音视频/文件/@/回复/卡片/按钮/链接/emoji/表情贴纸/微信表情/face 等)
- **受控自我迭代**: Agent 分析失败→提案→多 Agent 讨论→人工批准→应用
- **信任分系统**: 渐进式权限 (0→read, 30→write, 60→network, 80→shell)
- **安全护栏**: PII脱敏 + 注入防护 + 内容审核 + 风险分级审查
- **审计链**: Merkle 防篡改 + 每日 JSON Trail + 完整性验证
- **SkillHub**: 远程技能市场搜索/安装/卸载
- **15渠道接入**: Telegram/Feishu/Discord/Slack/WhatsApp/Signal/Email/QQ/企业微信/钉钉/微信公众号/LINE/Kook/Satori + WebChat（HTTP 网关）
- **语音能力**: TTS语音合成 + STT语音识别 (OpenAI Whisper兼容)
- **浏览器自动化**: Headless Chrome 网页截图/内容提取/表单填充
- **联邦**: 多 Agent 实例互联协作

## API 端点概览

所有 `/v1/*` 和 `/api/*` 端点需要 `X-API-Key` 或 `Authorization: Bearer <jwt>`。

**Core**: `/healthz`, `/v1/version`, `/v1/chat`, `/v1/chat/stream`, `/v1/ws`, `/v1/token`
**Tenant**: `/v1/tenants` (GET/POST)
**Skills**: `/v1/skills`, `/v1/plugins`, `/v1/plugins/toggle`
**Memory**: `/v1/memory/stats`, `/v1/memory/search`, `/v1/memory/add`, `/v1/memory/compact`
**Knowledge**: `/v1/knowledge/search`, `/v1/knowledge/sources`, `/v1/knowledge/stats`, `/v1/knowledge/upload`, `/v1/knowledge/ingest`
**Session**: `/v1/conversations`, `/v1/conversations/messages`, `/v1/fork`, `/v1/fork/branch`, `/v1/fork/list`
**Scheduler**: `/v1/scheduler/jobs`, `/v1/scheduler/add`, `/v1/scheduler/remove`
**Cron**: `/v1/cron/list`, `/v1/cron/add`, `/v1/cron/remove`, `/v1/cron/run`
**Observe**: `/v1/metrics`, `/v1/metrics/prometheus`, `/v1/system/info`, `/v1/system/stats`, `/v1/cache/stats`
**Graph**: `/v1/graph/entities`, `/v1/graph/relations`, `/v1/graph/context`, `/v1/graph/stats`
**Identity**: `/v1/identity/resolve`, `/v1/identity/profiles`
**Cost**: `/v1/cost/summary`, `/v1/cost/budget`
**Heartbeat**: `/v1/heartbeat`, `/v1/heartbeat/trigger`, `/v1/heartbeat/logs`
**Inbox/Bots**: `/v1/inbox`, `/v1/inbox/read`, `/v1/bots`, `/v1/bots/detail`
**Search**: `/v1/search`, `/v1/search/providers`
**Providers**: `/api/providers`, `/api/providers/test`, `/api/providers/enable`, `/api/providers/disable`, `/api/providers/switch-model`, `/api/providers/session`
**Audit**: `/v1/audit/tail`, `/v1/audit/verify`, `/v1/audit/stats`
**Market**: `/v1/market/search`, `/v1/market/top`, `/v1/market/stats`
**Federation**: `/v1/federation/peers`, `/v1/federation/stats`
**Tools**: `/v1/tools/exec`, `/v1/tools/list`, `/v1/tools/poll`, `/v1/tools/kill`
**Embeddings**: `/v1/embeddings`, `/v1/subagent`, `/v1/subagent/message`
**Persona**: `/v1/persona`, `/v1/persona/skills`
**Router**: `/v1/router/stats`
**Usage**: `/v1/usage`, `/v1/quota`, `/v1/upload`
**SkillHub**: `/api/skillhub/search`, `/api/skillhub/install`, `/api/skillhub/installed`, `/api/skillhub/uninstall`, `/api/skillhub/trending`
**Iterate**: `/api/iterate/proposals`, `/api/iterate/approve`, `/api/iterate/reject`, `/api/iterate/trigger`, `/api/iterate/status`
**Trust**: `/api/trust/scores`, `/api/trust/reset`
**Audit Trail**: `/api/audit/trail`
**Skill Grow**: `/api/skillgrow/patterns`
**Review**: `/api/review/status`
**Webhook**: `/webhook/feishu`

完整 API 文档可在 Dashboard 的 System 标签页中查看。

## 环境变量

详见 `.env.example`，核心变量：

- `LLM_API_KEY` (必填) — 大模型 API 密钥
- `LLM_BASE_URL` — OpenAI 兼容 API 地址 (默认 gitcode)
- `LLM_MODEL` — 模型名
- `LLM_FAST_MODEL` / `LLM_EXPERT_MODEL` — 多模型路由
- `AGENT_ADDR` — 监听地址 (默认 :9090)
- `OPEN_BROWSER` — 启动时自动开浏览器 (默认 true, Docker 中设 false)
- `HEARTBEAT_ENABLED` / `SELF_ITERATE_ENABLED` — 高级功能开关
- `BRAVE_API_KEY` / `TAVILY_API_KEY` — 联网搜索
- `EMBED_BASE_URL` / `EMBED_MODEL` — 向量嵌入

## 测试

```bash
go test ./... -count=1
```

## 技术栈

- **Backend**: Go 1.25, stdlib HTTP, 零外部框架依赖
- **Frontend**: Next.js 16 嵌入式 SPA (`//go:embed`)
- **Database**: 嵌入式 SQLite (`modernc.org/sqlite`，默认，数据位于 `data/yunque.db`) / PostgreSQL + pgvector (可选，设置 `DATABASE_URL` 启用)
- **LLM**: 任何 OpenAI 兼容 API
- **Deploy**: 单二进制 / Docker / docker-compose

## 文档

完整文档请访问 [yunque.owo.today](https://yunque.owo.today)

## License

MIT

© 2025 云鸢科技（青岛）有限公司 × Dream Lab

© 2025 CloudTori × Dream Lab
