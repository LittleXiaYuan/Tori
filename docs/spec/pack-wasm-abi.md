# Pack WASM 路由 ABI (Pack WASM Route ABI)

本规范定义 `backend.runtime.type == "wasm"` 的 Pack 如何通过 WebAssembly 模块提供 HTTP 后端能力。它是 `pack-distribution-spec.md`（分发链路）与 `pack-runtime-blueprint.md`（运行期挂载）的补充：本文档只锁定 **宿主 ↔ WASM 模块** 的调用契约。

> 适用对象：第三方不可信 Pack。第一方 Pack 仍走进程内 Go（`BackendModule` 接口），不经本 ABI。

---

## 1. 执行模型

- 每个声明了 `runtime.type == "wasm"` 的 Pack 在 `.yqpack` 内携带一个 `.wasm` 模块（`runtime.module` 指向 pack 相对路径）。
- Manifest 的每条 `backend.routeSpecs[]` 映射到该模块的一个导出函数（`entrypoint`，缺省 `_start`，遵循 WASI 命令模块约定）。
- 宿主对每个 HTTP 请求：构造请求信封 → 作为 stdin 喂给模块 → 运行 → 从 stdout 读取响应信封。
- 模块在 `wazero` WASI 沙箱中执行：无文件系统、无网络，除非经 host 函数显式授予。每次执行新建 runtime，完全隔离。

## 2. 完整性

- `runtime.sha256`（hex）声明模块字节的 SHA-256。宿主在每次执行前校验，不匹配则拒绝（HTTP 409）。
- 该字段纳入 manifest 签名材料（canonical JSON 覆盖 `backend` 全部字段），篡改模块或其声明 SHA 都会使验签失败。

## 3. 请求信封（stdin，JSON）

```json
{
  "method": "POST",
  "path": "/v1/hello/ping",
  "query": {"k": ["v1", "v2"]},
  "headers": {"Content-Type": ["application/json"]},
  "body": "<原始请求体字符串>"
}
```

- `query` / `headers` 为 `map[string][]string`（保留多值语义）。
- `body` 为原始请求体；二进制内容由模块自行解释。宿主对转发体设 1 MiB 上限，独立于网关自身的体积限制。

## 4. 响应信封（stdout，JSON）

```json
{
  "status": 200,
  "headers": {"X-Pack": ["hello"]},
  "body": "<响应体字符串>"
}
```

- `status` 缺省/为 0 时按 200 处理。
- `headers` 逐项写入响应；宿主未显式设置 `Content-Type` 时缺省 `application/json`。
- 模块必须把完整响应信封作为**唯一** stdout 输出（前后空白会被 trim）。stdout 上限 256 KiB（沙箱 `MaxOutputBytes`）。

## 5. 错误语义（宿主侧）

| 情况 | HTTP 状态 |
|---|---|
| 模块文件缺失 | 404 |
| SHA-256 不匹配 | 409 |
| 沙箱执行错误（编译/实例化失败、超时） | 500 |
| 模块非零退出码 | 502（含 stderr 摘要） |
| stdout 非合法响应信封 | 502 |

模块通过 `os.Exit(0)` / WASI `proc_exit(0)` 正常退出；非零退出码被宿主视为失败。

## 6. Host 函数（当前可用）

沙箱在 `env` 模块下**无条件**导出（每次执行）：
- `kv_set(key_ptr, key_len, val_ptr, val_len) -> i32`
- `kv_get(key_ptr, key_len, buf_ptr, buf_cap) -> i32`（返回写入字节数）
- `log_message(ptr, len) -> i32`

### 6.1 特权 host 函数（权限受限）

特权能力按 Pack 的 `backend.permissions` **逐执行**导出：未声明对应权限时，该 host 函数根本不会出现在 `env` 模块里，模块若 import 它则实例化失败（宿主返回 502）。这就是强制边界——能力缺失即不可达，而非运行时再判断。

所有特权 host 函数共享同一 **ptr/len 约定**：`fn(reqPtr, reqLen, respPtr, respCap) -> i32`，从 `reqPtr/reqLen` 读请求 JSON，把响应 JSON 写入 `respPtr`（容量 `respCap`）；返回值 `>=0` 为写入字节数，`<0` 为缓冲区不足时所需字节数的相反数（guest 扩容重试）。能力仅在「Pack 声明了权限 **且** 宿主侧 provider 已接线」时导出。

| host 函数 | 所需权限 | provider | 请求 / 响应 JSON |
|---|---|---|---|
| `http_fetch` | `net:fetch` | 内置（SSRF 安全客户端） | `{"url","method","body","headers"}` → `{"status","body","error"}` |
| `memory_search` | `memory:read` | `Orchestrator` | `{"query"}` → `{"context"}`（按请求 tenant 召回） |
| `ledger_get` | `ledger:read` | Pack KV（Ledger） | `{"key"}` → `{"found","value"}` |
| `ledger_set` | `ledger:write` | Pack KV（Ledger） | `{"key","value"}` → `{"ok","error"}` |

- `http_fetch`：宿主对 `url` 执行 SSRF 校验（拒绝 loopback/私网/元数据，重定向与连接期二次校验），15s 超时，响应体上限 256 KiB；传输/SSRF 失败以 `{"status":0,"error":...}` 信封返回（仍是成功写入）。
- `memory_search`：tenant 取自请求上下文，调用宿主内存编排器的召回。
- `ledger_*`：键被命名空间化为 `{packID}:{tenant}:{key}`，Pack 之间、Tenant 之间互不可见。

> 更多特权 host 函数沿用同一模式（权限门控 + provider 接线 + ptr/len 约定）按需扩展。

## 7. 示例模块（Go 原生 wasip1）

```go
package main

import ("encoding/json"; "io"; "os")

func main() {
    in, _ := io.ReadAll(os.Stdin)
    var req struct{ Method, Path, Body string }
    _ = json.Unmarshal(in, &req)
    resp := map[string]any{"status": 200, "body": `{"pong":true}`}
    out, _ := json.Marshal(resp)
    os.Stdout.Write(out)
}
```

编译：`GOOS=wasip1 GOARCH=wasm go build -o module.wasm main.go`。
TinyGo 产物体积约小一个数量级（Go 原生最小模块约 2–3 MB，冷启动数百 ms）；宿主未来会缓存 `CompiledModule` 摊销编译开销。
