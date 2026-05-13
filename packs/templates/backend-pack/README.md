# Backend Pack Template

这个模板用于新增一个可选后端能力包，而不是继续把功能压进主系统。

## 复制模板

```powershell
Copy-Item -Recurse packs/templates/backend-pack packs/examples/my-pack
```

然后修改：

- `pack.json` 的 `id/name/version/description`；
- `backend.capabilities`；
- `backend.routes`；
- `backend.routeSpecs`（method/path 元数据，用于 method-aware gate 和控制台审计）；
- `frontend.menus`；
- `frontend.routes`；
- `sdk.typescript`。

## 实现后端模块

在 `internal/packs/<name>/handler.go` 实现：

```go
type Handler struct{}

func (h *Handler) PackID() string { return "yunque.pack.example" }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/example-pack/ping", Handler: h.Ping},
	}
}
```

然后在 Gateway 构造时通过 `GatewayConfig.BackendPacks` 注入，或在 Gateway 构造后调用 `RegisterBackendPack` 挂载。

## 验证

```powershell
go test ./pkg/packruntime ./internal/controlplane/gateway -run 'Test(PackRoutes|BackendPack|Manifest|Registry)' -count=1
node scripts/check-pack-contract.mjs
```

完整契约见：

- `packs/AUTHORING.md`
