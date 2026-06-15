# Pack 路由迁移计划（后端路由 Pack 化）

> 目标：把网关里直连的后端路由，按「每个 surface 一个能力包」的方向，逐组迁移到
> Pack Runtime 下（先桥接、后填肉），消除空壳包，最终让能力面都能在能力包中心
> 热启停。本计划是工程化执行蓝图，按组推进、每组可回滚、每步带测试。

状态：草案 / 待执行（执行前需先有干净基线，见 §1）。

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

## 6. 进度跟踪

- 第 0 步：干净基线（提交在途 pack 工作）—— 由仓库负责人完成
- 第 1 组：knowledge（样板，确立可复用模式）
- 第 2 组：memory
- 第 3 组：skills
- 第 4 组：work
- 第 5 组：cogni-console（多为菜单指向）
- 第 6 组：control-plane（评估是否常开 core pack）
- 第 7 组：workspace（评估是否需后端）
- 填肉阶段：各组实现从网关搬入 pack 模块

