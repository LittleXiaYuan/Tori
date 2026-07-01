# 重构方案：Cogni 编排 · 技能/MCP 接入 · Pack 前端 UX

> 状态：**待审阅**（草案 v1）
> 范围：三块用户体验重构，分阶段、可独立交付
> 原则对齐：信念驱动（非指令驱动）；后端已真实，重心在前端 UX 与运行时接通

---

## 0. 一句话诊断

| 块 | 后端 | 前端 UX | 核心病灶 |
|----|------|---------|----------|
| **Cogni**（MCP/skill 编排层） | ✅ 完整 | ❌ 残废 | **创建后无法编辑**；详情只读裸 JSON；创建后不知道配了什么 |
| **技能 / MCP**（能力接入） | ✅ 已合流为一套（SkillAdapter） | ❌ 割裂 | UI 无法新增 MCP 服务器；无测试连接；工具详情只剩名字 chip |
| **Pack**（平台功能模块，~24 页） | ✅ 真实零空壳 | ⚠️ 开发者化 | 一半页面是「裸 JSON textarea + 运维术语 + JsonViewer 吐回」 |

**关键架构事实（已查实，决定方案走向）：**
1. MCP 工具经 `internal/mcp/adapter.go` 的 `SkillAdapter` 适配后，注册进 `pkg/skills` 的 `Registry`，**和原生 skill 并列出现在 `/v1/skills`**。→ 后端已是一套体系，前端不该再割成 `/skills` 与 `/settings/connectors` 两套。
2. 导航 `lib/nav-items.tsx` 已有成熟分层（core/pack/lab/control-plane + profile 过滤 + 按 pack 启用驱动）。→ **地基好，问题在各 pack 页面内部**，不需要重做导航骨架。
3. MCP 服务器配置目前只存于 `data/mcp.json`，**没有任何 HTTP 端点供前端增删改**。→ MCP 自助接入需要新增后端路由（唯一一处明显的后端缺口）。
4. Cogni 后端有 `POST /v1/cognis`（传完整 Declaration）但**没有 PATCH/update**；前端 client 也只有 add/remove/setEnabled。→ Cogni 编辑需要补一个更新路径。

---

## 1. Cogni 重构 —— 让编排「可改、可懂」

**目标取向**（已确认）：自然语言为主 + 结构化微调。保留「一句话生成」，叠加少量关键字段的表单，门槛最低，最贴合信念驱动理念。

### 1.1 后端缺口
- 新增 `PUT /v1/cognis/{id}`（或 `PATCH`）：接收完整或部分 Declaration，校验后落库 + 触发 reload。
  - 位置：Cogni 内核 pack 的路由处（与现有 `POST /v1/cognis` 同处）。
- 新增 `POST /v1/cognis/{id}/preview`（可选）：给定 Declaration 草稿，返回「它会在什么情况下激活、暴露哪些工具/技能、注入什么上下文」的人话摘要，供编辑时实时预览。
- SDK：在 `cogni-kernel-pack-client.ts` 增加 `update(id, decl)` 与 `preview(decl)`。

### 1.2 前端
**详情 Modal 的 Config tab：只读裸 JSON → 结构化编辑表单**，只暴露用户真正需要调的字段：
- **激活条件**（activation）：关键词/场景标签（chip 输入）、最低匹配分（slider）、启用开关。
- **能力范围**（surface）：从「已安装技能 + 已连接 MCP 工具」里多选（复用 `/v1/skills` 统一列表，见第 2 块）。这是 Cogni 编排的核心——勾选它该用哪些工具。
- **上下文注入**（context）：注入哪类记忆/知识（下拉 + scope），用人话标签而非 `memory_query` 原始字段。
- **行为指导**（behaviorText）：一个 textarea，但这是「给智能体的话」，是产品语义，不是 JSON。
- 高级字段（priority、min_score 细调等）折叠进「高级」区，默认不展示。
- 保留「查看原始声明」只读 JSON 作为 escape hatch，给高级用户。

**创建后给「人话摘要卡」**：调用 `preview`，告诉用户「这个 Cogni 会在你提到 X 时激活，自动用 A/B 工具，注入你的 C 记忆」。消除「创建完不知道发生了什么」的黑箱感。

**收敛入口**：`/packs/cognis`（纯说明页）降级为 `/cognis` 的一个折叠帮助区或直接重定向，去掉「整页都在说去别处操作」的困惑。

### 1.3 交付物
- 后端 `PUT /v1/cognis/{id}` + 测试；可选 `preview`。
- SDK `update`/`preview`。
- 前端 Config tab 表单化 + 创建后摘要卡。
- `make openapi` 重生成。

---

## 2. 技能 / MCP 重构 —— 一套「能力中心」+ MCP 自助接入

**洞察**：后端已合流，前端却两套。重构方向是**统一视图 + 补齐 MCP 自助接入的后端缺口**。

### 2.1 后端缺口（MCP 服务器管理 API —— 唯一需要新写后端的地方）
当前 MCP 服务器只能手改 `data/mcp.json`。新增一组路由（建议归入一个 `mcp-registry` pack 或挂在现有 mcp 模块）：
- `GET  /v1/mcp/servers` —— 列出已配置的 MCP 服务器及其连接状态、工具数。
- `POST /v1/mcp/servers` —— 新增一个服务器（transport/command/url/headers…），写入配置并尝试连接。
- `PUT/DELETE /v1/mcp/servers/{name}` —— 改 / 删。
- `POST /v1/mcp/servers/{name}/test` —— **测试连接**：尝试握手 + 拉工具列表，返回成功/失败 + 工具数 + 错误原因。**填表即可验证，不必盲连。**
- `GET  /v1/mcp/servers/{name}/tools` —— 该服务器提供的完整工具详情（name/description/inputSchema）。
- 数据结构复用 `internal/mcp/config.go` 的 `ServerConfig`。

### 2.2 前端：合并为「能力中心」
把 `/skills`（已安装/市场/动态三 tab）与 MCP/连接器统一到一个心智模型下。建议结构：
- **我的能力**：统一列出原生技能 + MCP 工具 + 连接器动作（数据源就是 `/v1/skills`，已经合流）。每项可点开看描述/参数/来源/调用统计。
- **接入**：
  - MCP 服务器：表单新增（stdio 填 command/args，http 填 url/headers）→ **测试连接**按钮即时反馈 → 成功后列出它带来的工具。彻底干掉「手改 mcp.json」。
  - 连接器（GitHub/Notion…）：保留，但补「测试连接」、补工具详情（调 `connectorDetail` 把 `actions[]` 的 description/parameters 展开，而非只显示 5 个名字 chip）。
  - 技能市场 / GitHub slug 安装：保留现状（这块本来就友好）。
- **动态技能审批**：保留。

### 2.3 交付物
- 后端 MCP 服务器管理 6 个端点 + 测试。
- SDK `mcp-servers` 客户端 + 注册进 `check-sdk-boundaries.mjs`。
- 前端「能力中心」：统一列表 + MCP 表单接入 + 测试连接 + 工具详情抽屉。
- 连接器页补测试连接 + 工具详情（调已存在的 `connectorDetail`）。

---

## 3. Pack 前端 UX 提质 —— 「后端真，前端糙」批量整改

**不动后端、不动导航骨架**，只整治各 pack 页面内部的交互层。

### 3.1 病灶量化（已扫描，按 JSON 重度排序）
| Pack 页面 | TextArea | JsonViewer | 重度 |
|-----------|:---:|:---:|:---:|
| `wasm-plugin` | 13 | 25 | 🔴🔴🔴 |
| `memory-time-travel` | 2 | 22 | 🔴🔴🔴 |
| `studio`(pack studio) | 12 | 5 | 🔴🔴 |
| `skill-anomaly` | 3 | 7 | 🔴 |
| `chaos-probe` | 1(TextField) | 7 | 🔴 |
| `cognitive-canary` | 2 | 7 | 🔴 |
| `rpa-replay` | 3 | 5 | 🟠 |
| `guardrail-fuzzer` | 2 | 6 | 🟠 |
| `sbom-drift` | 0 | 4 | 🟡 |
| `computer-use` | 2 | 1 | 🟡 |
| `browser` | 0 | 2 | 🟢 已较好 |
| backup / lora / micro-agent / night-school / inner-life / world-model / experience | 0 | 0 | 🟢 已表单化，参考样板 |

> 典型病灶（chaos-probe）：用户在 `<TextField value={definitionJSON}>` 手写一坨 JSON，结果用 4 个 `JsonViewer` 原样吐回 JSON。运维术语（shadow traffic / promotion decision / LLM-as-Judge batch）无解释。

### 3.2 整改套路（一套可复用模式，逐页套用）
为「裸 JSON 编辑」与「JsonViewer 展示」建立两个共享改造范式：
1. **输入侧**：裸 JSON textarea → 结构化表单 / 向导。对确实需要高级 JSON 的场景，提供「表单」+「JSON（高级）」双模 tab，默认表单。
2. **输出侧**：`JsonViewer` 原样吐 → 摘要卡（人话 + 关键指标）+「查看原始 JSON」折叠。复用一个 `<ResultSummary>` 组件，各 pack 传入「怎么把这坨结果讲成人话」的渲染器。
3. **术语**：英文运维名 → 中文产品名 + tooltip 解释（已有 `pack-presentation.ts` 承载展示元数据，可扩展存放术语词条）。
4. **样板**：已表单化的 `backup`/`lora`/`night-school`/`world-model` 是好样板，新整改对齐它们。

### 3.3 分批
- **批次 1（高频/重灾）**：memory-time-travel、studio、cognitive-canary、chaos-probe —— 用户最可能碰到、JSON 最重。
- **批次 2（安全/进阶）**：wasm-plugin、skill-anomaly、guardrail-fuzzer、rpa-replay。
- **批次 3（收尾）**：sbom-drift、computer-use 及零散术语清理。

### 3.4 命名/导航小修
侧边栏中英混杂（Chaos Probe / Memory Time Travel / Guardrail Fuzzer）→ 统一中文展示名（在 `pack-presentation.ts` / pack manifest 的 `metadata` 里补 `displayName.zh`），保留英文为副标题。不改 pack id。

---

## 4. 实施顺序建议

```
阶段一（Cogni 可编辑）        —— 痛感最强、范围可控，先打通「可改」
阶段二（能力中心 + MCP 自助） —— 后端补 6 个端点是唯一新后端，价值高
阶段三（Pack UX 批次 1）      —— 复用范式，逐页推进，可并行多页
阶段四（Pack UX 批次 2/3 + 命名）
```

每阶段独立可交付、独立测试（`make test-all` / 各 pack page 已有 vitest）。每动路由后 `make openapi`。

---

## 5. 待你确认的开放点

1. **MCP 服务器管理 API** 挂哪：新建 `yunque.pack.mcp-registry` pack，还是挂进现有 mcp/mcpdispatch 模块？（倾向新建独立 pack，符合分层）
2. **能力中心**是新页 `/capabilities` 还是改造现有 `/skills`？（倾向改造 `/skills`，少一个新路由）
3. Pack UX 整改要不要先做 1 个**样板页**（建议 chaos-probe，体量适中、范式齐全）跑通端到端，确认风格后再批量？
4. 阶段一就开工，还是先把本方案的某些点再细化？
```

---
（本文件是规划草案，实施时逐条勾掉；可直接在此批注。）
