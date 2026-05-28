# OpenAPI spec 演化策略

本文件对应内部任务清单 T28：`docs/openapi.yaml` 已经是云雀四类 SDK / API reference 的杠杆点，但 spec 本身必须有可执行的演化策略，避免“路由能跑、SDK 却被破坏”。

## 当前事实

- 源文件：`docs/openapi.yaml`
- 生成器：`cmd/openapi-gen`
- 生成命令：`make openapi`，等价于：

```powershell
go run ./cmd/openapi-gen
go test ./cmd/openapi-gen
```

- TypeScript SDK 输入：`packages/yunque-client/openapi-ts.config.ts` 指向 `../../docs/openapi.yaml`。
- TypeScript SDK 生成命令：

```powershell
npm run generate --prefix packages/yunque-client
```

- TypeScript SDK 生成脚本会保留手写 focused slices，只刷新 generated OpenAPI client/types。
- Python SDK、Rust SDK 和 manifest SDK entrypoints 也把 `docs/openapi.yaml` 作为事实源或事实边界之一。

## Source of truth

1. **路由事实源**：`internal/controlplane/gateway` 的 `mux.HandleFunc(...)` 注册。
2. **公开契约事实源**：`docs/openapi.yaml`。
3. **SDK 消费事实源**：`packages/yunque-client/openapi-ts.config.ts`、`sdk/python/openapi-config.yaml`、`sdk/rust` build/generator 链路，以及 `sdk/manifest/*.json`。

不能只改 handler 而不刷新 spec；不能只手改 spec 而不确认真实 gateway route 仍存在。

## 变更分类

### Patch：允许直接进入补丁版本

- 修正文档描述、summary、tag description。
- 补充 response description，但不改变 status code / schema 形状。
- 给已有 object schema 增加可选字段。
- 给已有 query parameter 增加可选项。
- 修复明显错误的 example。

### Minor：需要在 release note 标出

- 新增 endpoint。
- 新增 response status code，但保留原成功路径。
- 新增 optional request field。
- 新增 focused SDK slice / manifest SDK entrypoint。
- 给旧 endpoint 增加非破坏性 capability。

### Breaking：必须显式标记

以下任何一项都属于 breaking change：

- 删除或重命名 path。
- 删除、重命名或改变 `operationId`。
- 改变 HTTP method。
- 将 optional request field 改为 required。
- 删除 response field 或改变字段类型。
- 改变默认认证要求或权限边界。
- 改变分页、排序、游标、错误码语义。
- 改变 streaming / SSE / WebSocket event shape。
- 删除 SDK export、subpath export 或 manifest entrypoint。

Breaking change 必须在 PR / release note 中包含：

```md
BREAKING-OPENAPI:
- changed paths/operationIds:
- affected SDKs: TypeScript / Python / Rust / manifest
- migration:
- compatibility fallback or deprecation window:
```

## operationId 规则

- `operationId` 是 SDK 生成的稳定锚点，不是纯展示文本。
- 新 endpoint 必须有唯一且语义稳定的 `operationId`。
- 已发布 endpoint 的 `operationId` 不应为了“更好看”而重命名。
- 如果 handler 内部重构，但 HTTP 契约未变，`operationId` 必须保持不变。
- 如果确实需要重命名，按 breaking change 处理，并给出迁移说明。

## 推荐变更流程

1. 改 gateway route / handler。
2. 运行：

```powershell
make openapi
```

3. 检查 `docs/openapi.yaml` diff：
   - path 是否新增/删除；
   - `operationId` 是否稳定；
   - request/response schema 是否出现破坏性变化；
   - security 是否改变。
4. 刷新 SDK：

```powershell
npm run generate --prefix packages/yunque-client
npm run check:incremental --prefix packages/yunque-client
npm run typecheck --prefix packages/yunque-client
```

5. 如涉及 manifest/focused slices，运行：

```powershell
npm run check:sdk-manifests --prefix packages/yunque-client
```

6. 运行本策略检查：

```powershell
node scripts/check-openapi-evolution.mjs
```

## Review checklist

每次 API PR 至少回答：

- 是否新增、删除或重命名 path？
- 是否改变 method / auth / status code？
- 是否改变 `operationId`？
- 是否新增 required request field？
- 是否删除 response 字段或改变类型？
- 是否影响 streaming event shape？
- TypeScript / Python / Rust / manifest SDK 是否需要同步？
- 是否需要 deprecation window？
- 如果是 breaking change，是否写了 `BREAKING-OPENAPI` 块？

## 与 SDK 版本策略的关系

`docs/SDK-VERSIONING.md` 定义 SDK patch / minor / breaking 的发布语义；本文件定义 OpenAPI spec 层面的触发条件。两者冲突时，以更保守的判断为准：只要 OpenAPI 变化会破坏已发布 SDK 调用方，就按 breaking change 处理。

## 自动检查

`node scripts/check-openapi-evolution.mjs` 当前会检查：

- `docs/openapi.yaml` 仍是 `cmd/openapi-gen` 生成的 OpenAPI 3.1 spec。
- `Makefile` 仍提供 `make openapi`。
- TypeScript SDK generator 仍指向 `../../docs/openapi.yaml`。
- `operationId` 存在且不重复。
- OpenAPI path 覆盖仍是 300+ 级别，避免意外生成空 spec。
- 本文档保留 breaking change / operationId / regeneration / SDK check 规则。

它不是完整 diff-aware breaking-change detector，但足以防止 OpenAPI 演化策略和实际生成链路脱节。
