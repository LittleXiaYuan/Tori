# 云雀 Agent (Yunque Agent)

开箱即用的 AI Agent 平台 — 双击运行，自动打开浏览器，零依赖。

Production-ready AI Agent with embedded WebUI, multi-model routing, self-iteration, and 90+ API endpoints.

> **Language Note** — Code comments, godoc, and architecture docs are in English.
> Log messages, system prompts, and user-facing strings are in Chinese, reflecting the primary target market.
> Contributions in either language are welcome.

## 仓库结构说明

- 当前前端以 `heroui-web/` 为准，Go 服务嵌入的也是 `heroui-web/out/`
- `web/` 已归档，仅保留给历史参考，详细说明见 `web/ARCHIVED.md`
- `docs/` 用于面向用户的正式文档与文档站内容
- `doc/` 用于开发过程文档、连续性记录、设计蓝图等内部沉淀
- 仓库目录说明见 `docs/repo-layout.md`

## 一键启动

### 方式 1：双击运行 (Windows / macOS / Linux)

下载对应平台的二进制，双击即可。首次运行先执行安装向导：

```bash
# 安装向导（网页图形界面）
go run ./cmd/setup

# 启动 Agent（自动打开浏览器）
go run ./cmd/agent
```

打开 http://localhost:9090 即可看到完整管理面板 + 聊天界面。

### 方式 2：编译后运行

```bash
cp .env.example .env       # 配置 LLM_API_KEY
go build -o yunque-agent ./cmd/agent
./yunque-agent             # 自动开浏览器
```

### 方式 3：Docker 部署

```bash
cp .env.example .env       # 配置 LLM_API_KEY
docker compose --profile lite up -d    # 轻量版，无数据库
docker compose --profile full up -d    # 完整版，PostgreSQL + pgvector
```

Dashboard: http://localhost:9090

### 方式 4：跨平台构建

```bash
make release   # 生成 Windows/macOS/Linux (amd64+arm64) 6 个二进制
```

## 架构

```
┌─────────────────────────────────────────────────────────┐
│   Control Plane (Gateway) — 90+ API endpoints       │
│   Embedded WebUI · JWT/APIKey · CORS · Rate Limit    │
│   SSE Stream · WebSocket · Prometheus · Audit Chain   │
├─────────────────────────────────────────────────────────┤
│   Agent Core                                         │
│   Planner (multi-step, parallel FC, reflect)         │
│   LLM Pool (fast/smart/expert) · Circuit Breaker     │
│   Smart Router · Context Window · Model Override      │
├─────────┬────────────┬───────────┬────────┬───────────┤
│ Memory  │ Knowledge  │ Guardrail │ Trust  │ Iterate   │
│ 5-layer │ Graph+RAG  │ PII/Inj   │ Score  │ Self-     │
│ Orch    │ Embeddings │ Moderate  │ 0→100  │ Improve   │
├─────────┴────────────┴───────────┴────────┴───────────┤
│   Execution Layer                                    │
│   Sandbox · Scheduler · Cron · Tools Process Manager │
│   Channels: TG/Feishu/Discord/Slack/WA/Signal/Email  │
│            WeCom/DingTalk/WeChatOA/LINE/Kook/Satori  │
├─────────────────────────────────────────────────────────┤
│   Skills & Plugins                                   │
│   SkillHub (ClawHub) · Marketplace · Hot-load        │
│   Skill Growth Detector · Knowledge Distill          │
├─────────────────────────────────────────────────────────┤
│   Storage (file-based default, optional PostgreSQL)   │
│   Federation Hub · Multi-Agent Runtime Pool           │
└─────────────────────────────────────────────────────────┘
```

## 核心特性

- **开箱即用**: 单二进制，零依赖，自动开浏览器，嵌入式 WebUI
- **多模型路由**: Fast/Smart/Expert 三层池 + 智能路由 + 断路器 + Fallback
- **多LLM Provider**: 多厂商Provider注册中心，密钥轮换，会话级模型覆盖
- **5层记忆**: Short/Mid/Long + 知识图谱 + 可编辑记忆，统一召回
- **知识库 RAG**: 文件导入 + 混合检索 (BM25稀疏 + 向量密集 + RRF融合 + 可选Rerank二阶排序)
- **上下文压缩**: 多阶段压缩管线 (轮数限制 → LLM摘要 → 紧急减半)
- **富消息组件**: 12种消息组件 (文本/图片/音视频/文件/@/按钮/链接等)
- **受控自我迭代**: Agent 分析失败→提案→多 Agent 讨论→人工批准→应用
- **信任分系统**: 渐进式权限 (0→read, 30→write, 60→network, 80→shell)
- **安全护栏**: PII脱敏 + 注入防护 + 内容审核 + 风险分级审查
- **审计链**: Merkle 防篡改 + 每日 JSON Trail + 完整性验证
- **SkillHub**: 远程技能市场搜索/安装/卸载
- **15渠道接入**: Telegram/Feishu/Discord/Slack/WhatsApp/Signal/Email/企业微信/钉钉/微信公众号/LINE/Kook/Satori/WebChat
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
- **Frontend**: Next.js 15 嵌入式 SPA (`//go:embed`)
- **Database**: 文件存储 (默认) / PostgreSQL + pgvector (可选)
- **LLM**: 任何 OpenAI 兼容 API
- **Deploy**: 单二进制 / Docker / docker-compose

## License

MIT
