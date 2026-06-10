# Pack 前端 DLC 加载规范 (Pack Frontend DLC Loading)

本规范定义云雀 Pack 的**前端运行时动态加载**：如何把一个 Pack 自带的前端界面（侧栏菜单、路由页面、面板）在不重新构建主应用、不污染主应用安全边界的前提下，于运行时装配进 Web UI。

它是三份既有规范的补充，只锁定 **主应用宿主 ↔ Pack 前端模块** 的装配与通信契约：

- `pack-runtime-blueprint.md`：运行期挂载（registry / capability / gateway 装载 / frontend 装配）
- `pack-distribution-spec.md`：分发链路（`.yqpack` 打包、签名、安装）
- `pack-wasm-abi.md`：后端 WASM 路由 ABI（宿主 ↔ WASM 模块）

> 适用对象：第一方与第三方 Pack 的**前端资产**。后端能力仍分别走进程内 Go（第一方）或 WASM ABI（第三方）。

---

## 1. 背景与现状

### 1.1 已具备（无需重做）

- 后端 WASM DLC 已落地：`internal/execution/sandbox/wasm.go`（wazero、零 CGo、每次执行独立 runtime + 编译缓存 + WASI + host 函数 + 内存/超时上限）；ABI 见 `pack-wasm-abi.md`。
- Manifest 已声明前端结构：`pkg/packruntime/manifest.go` 的 `FrontendManifest{ Menus, Routes, Assets }` 与 `DistributionManifest{ FrontendURL, SHA256 }`。
- 前端已声明式消费：`apps/web/src/lib/pack-sync.tsx` 的 `buildPackNavItems()`、`buildPackRouteBindings()` 已把 `frontend.menus/routes` 转成导航项与路由绑定。

### 1.2 真正的缺口

当前 `frontend.routes[].component` 解析到的是**主应用内预构建**的页面（`apps/web/src/app/packs/*/page.tsx`）。**不存在任何运行时远程 JS 加载**——新增一个 Pack UI 仍需改主应用源码并重新构建。这正是本规范要补的能力。

### 1.3 两条硬约束（设计必须正视）

1. **静态导出**：`apps/web` 以 `output: "export"` 静态导出后由 Go `//go:embed` 进二进制。Next.js Module Federation / 运行时 `import()` 远程 bundle **在静态导出下不被原生支持**。因此宿主**不**使用 Next 的模块联邦，而是自建装载器。

2. **同源信任边界**：auth token 存于 `localStorage`（`yunque_token` / `yunque_api_key`，见 `use-chat-init.ts`、`chat/page.tsx`）。**把不可信第三方 JS 直接 `import()` 进同源主应用 = token 可被窃取**，会作废后端那套精心隔离。

   现行全局响应头（`internal/controlplane/gateway/middleware.go` `securityHeaders`）：
   ```
   X-Frame-Options: DENY
   Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; ... frame-ancestors 'none'
   ```
   即主应用既禁止被他人 iframe，又只允许同源脚本。

---

## 2. 设计原则

- **隔离优先**：第三方前端默认运行在**独立 origin 的沙箱 iframe** 中，而非同源 `import()`。宿主与 Pack 仅通过受控 `postMessage` 桥通信。
- **能力受限**：Pack 前端不持有 token，不直接发后端请求；所有后端访问经桥代理，由宿主按 `manifest.permissions` / `capabilities` 鉴权（默认拒绝）。
- **声明式装配**：复用既有 `frontend.menus/routes`，新增资产类型即可启用，无需改主应用路由表。
- **完整性可验证**：前端 bundle 与后端模块一样纳入 SHA-256 校验。
- **可降级**：第一方信任 Pack 可选「inline 同源」模式；不可信 Pack 强制沙箱模式。

---

## 3. 架构总览

```
┌──────────────────────────── 主应用 (同源, 持有 token) ─────────────────────────┐
│  Sidebar (buildPackNavItems)      Router (buildPackRouteBindings)              │
│        │                                  │                                     │
│        ▼                                  ▼                                     │
│  PackDlcHost ──────────────────── 渲染 <iframe sandbox> ────────────────┐      │
│   (apps/web/src/lib/pack-dlc-host.tsx)                                   │      │
│        ▲   postMessage 桥 (能力鉴权 + 后端代理, 注入 token 在宿主侧)      │      │
└────────┼────────────────────────────────────────────────────────────────┼─────┘
         │                                                                  ▼
         │                                            ┌─────────── 沙箱 iframe (独立 origin) ┐
         │                                            │  Pack 前端 bundle (index.html + js/css) │
         │                                            │  仅能 postMessage, 无 token, 无同源 DOM  │
         ▼                                            └──────────────────────────────────────┘
   后端 Gateway: GET /v1/packs/{id}/ui/*  (托管 bundle, 独立 CSP, 允许被宿主 frame)
   后端能力:    既有 REST / WASM ABI (经桥代理调用, 宿主注入鉴权)
```

---

## 4. `.yqpack` 前端 bundle 格式

扩展 `FrontendAssets`（`pkg/packruntime/manifest.go`）：

```go
type FrontendAssets struct {
    Type  string `json:"type,omitempty"`  // "inline" | "iframe-bundle"（新增）
    Entry string `json:"entry,omitempty"` // bundle 入口, 相对 frontend/ 的 HTML, 默认 "index.html"
}
```

- `type == "inline"`：现状语义，`component` 指向主应用预构建组件（仅第一方）。
- `type == "iframe-bundle"`（**本规范新增**）：`.yqpack` 内 `frontend/` 目录携带自包含静态资源（`index.html` + `assets/*.js|css`）。宿主以沙箱 iframe 加载 `entry`。

`.yqpack` 布局（沿用 `pack-distribution-spec.md` §1）：

```
mypack.yqpack
├── pack.json
├── frontend/
│   ├── index.html          # FrontendAssets.Entry
│   └── assets/             # js / css / 图片，路径相对 index.html
└── ...
```

完整性：`DistributionManifest.FrontendURL` + `SHA256` 覆盖 `frontend/` 子树打包后的摘要；宿主装载前校验（失败渲染错误卡片，见 §9）。

---

## 5. 后端托管与响应头（关键集成点）

新增 Gateway 路由（建议归入 packs handler）：

```
GET /v1/packs/{id}/ui/*filepath   →  从该 Pack 安装目录的 frontend/ 静态返回
```

**必须对该路由前缀覆盖全局 `securityHeaders`**，否则 `X-Frame-Options: DENY` + `frame-ancestors 'none'` 会使宿主无法 frame 它：

| 头 | 全局值 | 本路由覆盖为 | 原因 |
|---|---|---|---|
| `X-Frame-Options` | `DENY` | **移除** | 允许被宿主 frame（用 CSP 收敛） |
| `Content-Security-Policy` | `frame-ancestors 'none'` | `frame-ancestors 'self'`；`default-src 'self'`；`connect-src 'none'` | 只许本实例宿主 frame；禁止 bundle 自行联网（联网走桥） |
| `Cross-Origin-Resource-Policy` | — | `same-origin` | 防跨站读取 |

> 实现注记：`securityHeaders` 当前是全局中间件。落地时为 `/v1/packs/{id}/ui/` 前缀提供 header 覆写（在 handler 内 `Set` 覆盖，或中间件按路径短路），并补单测对齐 `middleware_test.go` 的既有断言风格。

**独立 origin**：理想方案是用单独端口/子域承载 bundle 以获得真正跨 origin 隔离。鉴于云雀单二进制 + 本地优先，MVP 采用 `sandbox` 属性产生的 **opaque origin**（见 §6）达到等效的 DOM/存储隔离，无需第二端口。

---

## 6. 宿主侧装载器 `PackDlcHost`

新增 `apps/web/src/lib/pack-dlc-host.tsx`（组件 + 桥）。`buildPackRouteBindings()` 产出的、`assets.type == "iframe-bundle"` 的路由，统一由一个通用 Pack 路由页渲染 `PackDlcHost`，**无需为每个 Pack 建 `app/packs/*/page.tsx`**。

iframe 创建约束：

```html
<iframe
  src="/v1/packs/{id}/ui/index.html"
  sandbox="allow-scripts"   <!-- 关键：不含 allow-same-origin → opaque origin, 无法读宿主 localStorage/cookie -->
  referrerpolicy="no-referrer"
  allow=""                  <!-- 关闭 camera/mic/geo 等特性 -->
/>
```

- **不设 `allow-same-origin`**：iframe 获得 opaque origin，无法访问宿主同源存储/DOM，即使 bundle 含恶意 JS 也偷不到 token。
- 宿主只在 `message` 事件里用 `event.source === iframe.contentWindow` 与预期 origin 校验来路。

---

## 7. postMessage 桥 ABI

### 7.1 信封

```jsonc
{
  "v": 1,                       // 协议版本
  "id": "c1",                   // 关联 id（请求/响应配对）
  "kind": "req" | "res" | "event",
  "method": "backend.call",     // kind==req 时必填
  "payload": { ... },
  "error": { "code": "...", "message": "..." }  // kind==res 失败时
}
```

### 7.2 宿主提供的方法（默认拒绝，按权限放行）

| method | 作用 | 鉴权 |
|---|---|---|
| `host.handshake` | 协商版本、返回 Pack 元数据/主题变量 | 无条件 |
| `backend.call` | 代理一次后端调用（method+path+body）| 路径必须命中本 Pack 的 `backend.routeSpecs`；宿主注入 token；禁止越权访问他 Pack/核心 API |
| `nav.push` | 请求宿主跳转到允许的路由 | 仅限本 Pack 的 `frontend.routes` |
| `ui.toast` / `ui.resize` | 提示、自适应高度 | 无条件 |
| `storage.get/set` | Pack 私有 KV（命名空间 `pack:{id}:`）| 隔离到本 Pack 命名空间 |
| `events.subscribe` | 订阅宿主 SSE 流（SSE-over-bridge），返回 `{sub_id}` | path 须由 `backend.permissions` 中 `events:subscribe:<path>` 声明 |
| `events.unsubscribe` | 退订（`{sub_id}`）| 仅本 iframe 的订阅 |

### 7.2.1 SSE-over-bridge

沙箱 bundle 因 CSP `connect-src 'none'` 无法自行联网，实时面板经桥订阅：宿主持有 SSE 连接（同源、注入鉴权），把每个事件以 `kind:"event"` 信封推入 iframe：

```jsonc
{ "v": 1, "kind": "event", "method": "events.message", "payload": { "sub_id": "sub-1", "event": "trace", "data": "<原始 data>" } }
{ "v": 1, "kind": "event", "method": "events.closed",  "payload": { "sub_id": "sub-1", "reason": "closed" } }
```

- 订阅目标用权限声明（如 `"events:subscribe:/v1/events/stream"`），**不要**写进 wasm `routeSpecs`——后者会被 mount 成 pack 路由，遮蔽宿主端点。
- 每 Pack 并发流上限默认 4；iframe 卸载时宿主 `closeAll()` 终止全部流。
- 流结束/出错发 `events.closed`，guest 可自行重订。

关键安全点：**token 永不过桥**。`backend.call` 在宿主侧（同源、持 token）补 `Authorization`/`X-API-Key` 后转发，响应体回传 iframe。Pack 只表达「我要调用我声明过的这个路由」。

### 7.3 校验与健壮性

- 宿主对每条入站消息校验：`event.source` 命中本 iframe、信封结构、`method` 在白名单、`backend.call` 的 path 命中本 Pack 路由表（复用 `manifest.AllowsRoute` 同等语义）。
- 每个 `req` 设超时（默认 30s）；超时回 `res{error: timeout}`。
- 未知 `method` / 越权 → `res{error: forbidden}`，并计一次审计事件（接入 `internal/observe`）。

---

## 8. 生命周期

```
install → (enable) → 路由命中 → PackDlcHost 挂载
   → 校验 frontend bundle SHA-256
   → 注入 iframe(sandbox) + 建桥 + host.handshake
   → 运行期：能力受限的 backend.call / nav / ui
disable / 路由离开 → 卸载 iframe + 拆桥 + 清理监听
完整性失败 / 桥握手超时 → 渲染错误卡片（可重试 / 反馈），不影响主应用
```

Pack 崩溃（iframe 内异常）天然被 origin 隔离，不波及宿主。

---

## 9. 失败与降级语义

| 情况 | 宿主行为 |
|---|---|
| bundle 缺失 / 404 | 错误卡片：「该能力包前端资源缺失」 |
| SHA-256 不匹配 | 拒绝加载 + 审计；提示重新安装 |
| 握手超时 | 错误卡片 + 重试按钮 |
| `backend.call` 越权 | 桥回 `forbidden`，不发后端请求 |
| iframe 脚本异常 | 仅该面板失效，主应用照常 |

---

## 10. 安全模型小结（威胁 → 缓解）

| 威胁 | 缓解 |
|---|---|
| 窃取 token | iframe 无 `allow-same-origin`（opaque origin）+ token 永不过桥 |
| 读宿主 DOM / 改主应用 | 沙箱 origin 隔离 + 仅 postMessage |
| 任意联网 / 数据外泄 | bundle CSP `connect-src 'none'`；联网只能经 `backend.call` 且受路由白名单 |
| 越权调用核心/他 Pack API | 桥按本 Pack `routeSpecs` 鉴权，默认拒绝 |
| 供应链篡改 | `frontend` 子树 SHA-256 + manifest 签名（`pack-distribution-spec.md`）|
| 点击劫持 / 被第三方 frame | bundle `frame-ancestors 'self'`；主应用仍 `DENY` |

---

## 11. 实施切片（MVP → 完整）

- **M0 契约**：manifest 增 `FrontendAssets.Type=="iframe-bundle"`；本规范评审定稿。
- **M1 后端托管**：`GET /v1/packs/{id}/ui/*` + 路由级 header 覆盖 + SHA 校验 + 单测。
- **M2 宿主装载器**：`pack-dlc-host.tsx`（沙箱 iframe + 通用 Pack 路由页）+ `host.handshake` + `ui.toast/resize` + Vitest。
- **M3 能力桥**：`backend.call`（token 注入 + 路由白名单鉴权）+ `nav.push` + `storage.*` + 审计 + 超时。
- **M4 Demo Pack**：一个 `.yqpack`（含 `frontend/index.html` 调用 `backend.call` 命中其 WASM 路由），端到端验证「安装→启用→侧栏注入→面板渲染→受控后端调用」。

---

## 12. 验证计划

1. 构造含 `frontend/`（iframe-bundle）+ `.wasm` 后端的 `test.yqpack`。
2. 经 UI 安装 → 校验后端挂载 WASM 路由（`pack-wasm-abi.md`）。
3. 校验前端：`GET /v1/packs/{id}/ui/index.html` 返回正确 CSP / 无 `X-Frame-Options: DENY`。
4. 校验宿主：侧栏出现菜单（`buildPackNavItems`）、路由渲染沙箱 iframe、握手成功。
5. 安全断言：iframe 内 `window.localStorage`/`document.cookie` 读不到宿主 token；越权 `backend.call` 被拒。
6. 禁用 Pack → iframe 卸载、监听清理、菜单移除。

---

## 13. 未决问题与已决记录

- ~~真正跨 origin~~ **已实现（opt-in）**：设 `PACK_UI_ADDR`（如 `127.0.0.1:0`）启动独立 pack-UI 监听器，bundle 获得真实跨 origin 边界（叠加在 sandbox opaque-origin 之上）。该监听器**只**服务 `/v1/packs/{id}/ui/*`，CSP `frame-ancestors` 限本机 shell 源（localhost/127.0.0.1/tauri）；宿主经 `GET /v1/packs/ui-origin` 发现源，未启用时回退同源。注意：打包桌面端需在 webview CSP 中放行该额外源，故默认关闭。
- **WS 转发：暂不做（已决）**。理由：① 主前端零 WebSocket 消费（实时全走 SSE，`/v1/ws` 无前端调用方）；② 浏览器 WebSocket 无法携带 header，而 `requireAuth` 仅接受 header 鉴权——支持它需开放 query-token（token 进 URL/日志，安全退化）；③ SSE-over-bridge（§7.2.1）已覆盖实时面板需求。未来若出现双向交互需求再评估。
- **第一方 inline 模式**：是否保留同源 `import()` 给完全可信的内置 Pack（性能/体验更好，需信任分级 gate）。
- ~~桥能力面：事件订阅~~ 已实现：`events.subscribe/unsubscribe`（SSE-over-bridge，见 §7.2.1）。
- **主题与 i18n**：经 `host.handshake` 下发主题变量与语言，保证 Pack UI 视觉一致。
