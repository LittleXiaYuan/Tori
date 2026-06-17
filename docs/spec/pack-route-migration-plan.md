# Pack 路由迁移计划（后端路由 Pack 化）

> 目标：把网关里直连的后端路由，按「每个 surface 一个能力包」的方向，逐组迁移到
> Pack Runtime 下（先桥接、后填肉），消除空壳包，最终让能力面都能在能力包中心
> 热启停。本计划是工程化执行蓝图，按组推进、每组可回滚、每步带测试。

状态：执行中（基线已收敛）。截至 2026-06-18：干净基线已提交，v2 微内核 + 5 个 surface 包已迁移上生产，knowledge 已全原生，control-plane 已开始按切片填肉；`go test` 与前端 pack 测试全绿；逐组真实进度见 §6。

---

## 0. 现状（为什么要做）

- 直连网关路由 ~~310 条（~~70%）：chat / memory / knowledge / tasks / plugins /
triggers / governance / providers / setup 等核心面全是网关直写。
- Pack 拥有的路由 ~~131 条（~~30%）：多为可选能力包（chaos-probe / sbom-drift /
inner-life / world-model / lora 等）。
- **8 个空壳包**（`routes: []`、无 `internal/packs/<x>/handler.go`，仅前端导航门控）：
`workspace` / `control-plane` / `cogni-console` / `knowledge` / `memory` /
`work` / `skills`。真正的 API 仍在网关。

---

## 1. 三个硬约束（决定了「不能一次性全改」）

1. **Pack 路由强校验 HTTP 方法**：`requirePackRoute` 对未声明的方法直接返回
  `405`，且要求 manifest 的 `routeSpecs` 声明了该 `method+path`。而当前直连路由
   **不校验方法**（handler 内部自行处理）。→ 每条路由迁移前必须**核实它实际接受
   哪些方法**，否则前端会静默 405。
2. **必须同步拆掉网关直连注册**：Go 的 `http.ServeMux` 对同一 path 重复注册会
  **panic**。每迁一组，必须同时把 `register*Routes` 里的对应直连删掉。
3. **干净基线**：迁移前先把在途的 pack 重构（前端 / pack 发现 / 新清单 / 回归
  测试 / 配置）提交，避免在未提交改动上重构核心路由。

---

## 2. 桥接模式（参照 browser-intent / cogni-kernel，低风险）

每组先做「桥接」：Pack 拥有路由注册 + 热门控，handler 暂时仍调网关实现。

```go
// 1) 网关出一个导出的分发入口（gateway 包内，可访问私有 handler）
func (g *Gateway) HandleKnowledgePack(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/v1/knowledge/search": g.handleKBSearch(w, r)
    // ... 其余路由
    default: http.NotFound(w, r)
    }
}

// 2) 新建 internal/packs/knowledge/handler.go，实现 BackendModule
type KnowledgeGateway interface{ HandleKnowledgePack(http.ResponseWriter, *http.Request) }
type Handler struct{ gw KnowledgeGateway }
func (h *Handler) PackID() string { return "yunque.pack.knowledge" }
func (h *Handler) Routes() []packruntime.BackendRoute {
    d := h.gw.HandleKnowledgePack
    return []packruntime.BackendRoute{
        {Methods: []string{"GET","POST"}, Path: "/v1/knowledge/search", Handler: d},
        // ... 每条带「核实过的方法集」
    }
}

// 3) 注册：gw.RegisterBackendPack(knowledgepack.NewHandler(gw))
// 4) 删除 registerKnowledgeRoutes 里的直连注册（避免 mux panic）
// 5) manifest 填 routes + routeSpecs（method 与 (2) 一致）
```

填肉（后续）：把 handler 实现从网关真正搬进 pack 模块（彻底解耦）。

---

## 3. 分组迁移清单（按优先级）

> 原则：**可选能力面先迁**（disable 了也不会让云雀挂）；**核心面不做成可禁用包**
> （chat/providers/governance/setup 等，禁了=应用挂），最多归到一个常开 core pack。


| 序   | 空壳包             | 对应网关路由组                                     | 路由（执行时逐条核实方法）                                                                                    |
| --- | --------------- | ------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| 1   | `knowledge`     | registerKnowledgeRoutes                     | `/v1/knowledge/{search,sources,stats,upload,ingest,import-url,import-repo,source,source/update}` |
| 2   | `memory`        | registerMemoryRoutes（memory 子集）             | `/v1/memory/{stats,search,recall/debug,add,compact,persona,update}`                              |
| 3   | `skills`        | registerPluginRoutes（skills 子集）             | `/v1/skills/`*、`/api/skillhub/*`、`/v1/market/*`（归属待定）                                            |
| 4   | `work`          | registerTaskRoutes / projects / workflowapi | `/v1/tasks/*`、`/v1/projects/*`、`/v1/workflows/*`                                                 |
| 5   | `cogni-console` | （cogni-kernel 已有 backend）                   | 多半只需把 console 菜单指向已有 `/v1/cognis/*`                                                              |
| 6   | `control-plane` | registerGovernanceRoutes 等                  | audit/trust/tenants/metrics…**偏管理核心，建议归常开 core pack 而非可禁用**                                      |
| 7   | `workspace`     | —                                           | 纯 dashboard 导航，可能无需后端路由                                                                          |


注：`memory` 组里 graph/identity/embeddings/search 是否归 memory，需按产品语义定；
本计划默认只迁 `/v1/memory/`*，其余另议。

---

## 4. 每组执行 checklist（6 步，逐组一个提交）

1. 读对应 handler，**核实每条路由接受的 HTTP 方法**（含多方法）。
2. 网关加 `Handle<X>Pack` 导出分发方法（新文件 `handlers_<x>_pack.go`）。
3. 新建 `internal/packs/<x>/handler.go`（BackendModule + 窄接口 + Routes 带方法集）。
4. manifest 填 `backend.routes` + `routeSpecs`（method/path 与 §3 一致）。
5. 注册 pack（`gw.RegisterBackendPack(...)`）+ **删除网关直连注册**。
6. `go build ./...` + `go vet` + 该组路由测试 + 内置包计数测试；通过后单独 commit。

---

## 5. 测试 & 回滚

- 每组一个独立 commit，出问题只回滚那一组。
- 必测：enable→路由可用；disable→该组路由 404（核心面不纳入可禁用）；方法不符→405；
`packruntime_bootstrap_test` 计数随包集更新。
- 建议加一个「路由覆盖」测试：断言迁移后所有原路径仍可路由（防漏迁）。

---

## 6. 进度跟踪（2026-06-18 增量 checkpoint）

> 图例：✅ 已全原生（实现在 pack 内）｜🟡 部分原生（核心已迁，少量仍桥接到网关）｜⬜ 空壳 / 菜单（暂无后端或仅导航）

| 组   | 包              | 状态  | 说明                                                                                     |
| --- | -------------- | --- | -------------------------------------------------------------------------------------- |
| 0   | 干净基线           | ✅   | 提交 `0a1496d0`（147 文件）：v2 微内核 + 核心/单体路由原生化                                                |
| 1   | knowledge      | ✅   | 全原生：search/sources/stats/ingest/source 增删改、import-url/import-repo、upload 均由 `internal/packs/knowledge` 服务；MinerU 上传解析已抽到 `internal/agentcore/knowledge` 供 admin 与 pack 共享 |
| 2   | memory         | ✅   | 全原生（提交 `6d24735b`：recall-debug/persona/update 填肉，删桥接）                                     |
| 3   | skills         | ✅   | `/v1/skills` 全原生：列表、scan、dynamic、approve、reject（无网关桥接；scan 经 `Gateway.ScanSkills()` 注入）   |
| 4   | work           | ✅   | tasks / projects / workflows 全原生（workflow 由 `WorkflowHandler().RouteSpecs()` 合并挂载）       |
| 5   | cogni-console  | ⬜   | 多为菜单指向已有 `/v1/cognis/*`（cogni-kernel 后端已原生）                                              |
| 6   | control-plane  | 🟡  | 路由所有权已覆盖 governance/approvals/inbox/tools/bots/plugins/metrics/system/tenants/providers 等；observability 与 approvals 已原生，其余仍通过 gateway bridge |
| 7   | workspace      | ⬜   | 纯 dashboard 导航，暂无后端路由                                                                    |

计划外额外完成（已上生产、全原生）：v2 微内核生命周期（enable/disable → Start/Stop）；单体抽离包 modes / reverie / ide / cron / triggers / documents / missions / files / instructions / emotion / graph。

### 验证门禁（2026-06-15 基线收敛）

- `go test ./pkg/packruntime ./internal/controlplane/gateway ./internal/packs/... ./cmd/agent -count=1` → 全 **ok**（memory / state 暂无单测；迁移路由行为由网关迁移测试与各 pack 单测覆盖）。
- `apps/web` → `npm test -- pack`：**18 文件 / 103 测试全过** + SDK 边界检查 ok。
- manifest `backend.routes` ↔ `handler.Routes()`：五个迁移包（knowledge/memory/skills/work/control-plane）均一致；网关已无被迁路径的直连注册（无 `http.ServeMux` 重复注册风险）。

### 仍待“填肉”的 bridge 路由

- knowledge：无，已全原生。
- control-plane：tenants / plugins / models / inbox / tools / bots / providers 及部分 governance 面仍通过 `HandleControlPlanePack` 桥接，后续继续按低风险 surface 小切片迁移。

> 2026-06-15 增量：skills `/v1/skills/scan` 已去壳进 `internal/packs/skills`（删除 `handlers_skills_pack.go` 桥接与网关 `handleSkillsScan`，新增 `Gateway.ScanSkills()` 注入），skills 组转为 ✅ 全原生。

> 2026-06-18 增量：knowledge `import-url` / `import-repo` / `upload` 已去壳进 `internal/packs/knowledge`；upload/MinerU 共享逻辑抽为 `internal/agentcore/knowledge`。control-plane observability 五条读路由已原生，gateway 仅提供只读窄 accessor。

> 2026-06-18 增量：control-plane approvals 五条 HITL 路由已原生（列表、approve、deny、decide、rules），gateway 仅提供 `ApprovalManager()` 与 `TenantOf()` 窄 accessor。
