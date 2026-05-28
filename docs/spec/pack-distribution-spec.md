# Pack 分发规范 (Pack Distribution Spec)

本规范覆盖云雀 Pack 作为面向终端用户的增量 DLC 系统的分发协议、文件格式、生命周期与可信链。它和 `pack-runtime-blueprint.md` 互补：
blueprint 描述运行期挂载（registry、capability resolver、gateway 装载、frontend 装配），本 spec 描述把一个 Pack 从 *开发者机器* 安全送达 *用户运行实例* 的完整链路。

> **范围声明**：本 spec 不重复 blueprint 已经形式化的内容，不重复 `Manifest` Go struct 已经定义的字段含义。它锁定的是分发层未代码化的契约。

---

## 1. 设计目标与非目标

### 1.1 目标

1. **可下载**：Pack 可以离开本仓库，作为独立 artifact 通过 HTTPS 分发并被运行时验证。
2. **可信**：用户不需要信任分发镜像，只需信任已嵌入运行时的发行者公钥集合。
3. **可断点**：网络中断、镜像离线、SHA256 校验失败都不应让本地状态进入半安装态。
4. **可回滚**：任何一次更新都必须保留前一版本的 artifact 与 manifest，使 rollback 不依赖网络。
5. **跨平台**：单一 Pack artifact 必须能在 `linux/{amd64,arm64}` × `windows/{amd64,arm64}` × `darwin/{amd64,arm64}` 6 组目标中定向加载，且对未声明的目标显式失败而不是静默跳过。
6. **兼容现状**：保留 `cmd/yunque-plugin` 现有 `init/validate/build/install` UX，向下兼容仓库内 `pack.json` 直装路径。

### 1.2 非目标

- 不做内嵌包管理器（npm/pip 风格的 transitive resolution）；Pack 间依赖通过显式 `requires` 字段，由用户/UI 负责安装顺序。
- 不做执行隔离层（沙箱、cgroups、seccomp）；执行隔离由 *运行时* 而非 *分发* 负责，已有 wasmplugin 与未来 subprocess+gRPC 路径分别处理。
- 不做付费分发与许可证服务器；这些是上层服务，本 spec 只保证 artifact 可被授权流程拒绝/放行，不规定授权语义。

---

## 2. 文件格式：`.yqpack`

### 2.1 容器

`.yqpack` 是一个 **deterministic ZIP**：

- ZIP container（store 或 deflate，不要求 ZIP64 除非 artifact 超过 4 GiB）。
- 文件入口按 path 字典序排列。
- 时间戳全部固定为 `1980-01-01T00:00:00Z`（ZIP epoch），avoid build-host clock skew。
- 不允许 ZIP comment、不允许 extra field（`signing` 字段除外，见 §4.3）。

确定性是签名链的前提：同一 commit + 同一 build matrix 必须生成 byte-identical artifact。

### 2.2 内部目录结构

```
<pack-id>-<version>.yqpack
├── pack.json                       # 入口 manifest，等价 packs/official/*/pack.json
├── manifest.sig                    # detached signature（见 §4）
├── manifest.pub                    # 发行者公钥引用（不是公钥本身，见 §4.2）
├── backend/
│   ├── linux-amd64/yunque-pack-<id>      # 子进程二进制（subprocess+gRPC 路径）
│   ├── linux-arm64/yunque-pack-<id>
│   ├── windows-amd64/yunque-pack-<id>.exe
│   ├── windows-arm64/yunque-pack-<id>.exe
│   ├── darwin-amd64/yunque-pack-<id>
│   ├── darwin-arm64/yunque-pack-<id>
│   └── wasm/yunque-pack-<id>.wasm        # 可选：wasm 装载路径（参考 internal/packs/wasmplugin）
├── frontend/                       # 可选：等价 FrontendManifest.Assets.Type="bundle"
│   ├── index.html
│   └── assets/
├── sdk/                            # 可选：subpath SDK slice 静态描述
│   └── manifest.json
└── README.md                       # 用户可读说明（不参与签名内容判定，但 hash 入 manifest）
```

`backend/<goos>-<goarch>/` 任何一个目录可以缺失，但运行时若加载某目标平台缺失的 Pack，必须 *显式拒绝并提示具体平台*（见 §5.4），不允许 fall-through。

### 2.3 是否强制 6 平台？

**不强制**。`pack.json.distribution.platforms` 是一个 *显式 allowlist*：
开发者可以发布只支持 `linux/amd64 + darwin/arm64` 的 Pack，运行时会用此 allowlist 决定是否在当前 host 上 surface 该 Pack。
没在 allowlist 内的 host 收到的 catalog 条目应标记 `unsupportedPlatform: true`。

### 2.4 与 `cmd/yunque-plugin` zip 的兼容性

当前 `cmd/yunque-plugin install <source.zip>` 接受任意 ZIP（见 `cmd/yunque-plugin/main.go:409` `installFromZip`）。SPEC 引入 `.yqpack` 后保持向下兼容：

- `.zip` 后缀：legacy 路径，保持当前行为，但日志应建议迁移到 `.yqpack`。
- `.yqpack` 后缀：必须走完整 §4 验证链。
- 仓库内 `pack.json` 直装：保持当前行为，跳过签名验证（开发期路径，由 catalog `source` 字段标记 `local-builtin`）。

---

## 3. Manifest 字段约束

### 3.1 已规范字段（来自 `pkg/packruntime/manifest.go`）

下列字段已由 Go struct + `Validate()` 锁定，分发层不再重复定义：

| 字段 | 约束 |
| --- | --- |
| `id` / `name` / `version` | 必需。`id` 必须以 `yunque.pack.` 起始，`version` 必须 SemVer 2.0。 |
| `requiresCore` | SemVer range，例：`">=0.1.0"`。运行时 mismatch 时拒绝装载。 |
| `optional` / `defaultState` / `status` | 见 manifest.go validate。 |
| `backend.routes` / `backend.routeSpecs` / `backend.capabilities` / `backend.permissions` | blueprint 已定义，分发层只检查总长度上限（见 §3.3）。 |
| `frontend.menus` / `frontend.routes` / `frontend.assets` | 同上。 |
| `distribution.manifestUrl` / `packageUrl` / `frontendUrl` / `sha256` / `sizeBytes` | manifest.go 已定义；本 spec 在 §3.2、§4 中加严语义。 |
| `update.channel` / `update.rollback` | manifest.go 已定义。 |
| `metadata` | 任意 string-string map。 |

### 3.2 分发层加严的语义

- `distribution.packageUrl` 为空时表示 *bundled / built-in*，仅来自仓库内 catalog，外部源必须填。
- `distribution.sha256` 必须为 64 字符小写 hex，不允许 `sha256:` 前缀（与 `Registry.CacheDistribution` 现实现一致，但禁止前缀以减少校验分支）。
- `distribution.sizeBytes > 0` 必须与 `packageUrl` 实际长度匹配；下载流量保护门槛由运行时配置（默认 256 MiB）。
- `distribution.manifestUrl` 与 `packageUrl` *允许指向不同 host*，使 catalog index 与重型 artifact 分离镜像（small JSON on GitHub Pages / 大文件在 OSS）。

### 3.3 上限（防御性约束）

| 字段 | 上限 | 理由 |
| --- | ---: | --- |
| `backend.routes` + `routeSpecs` | 256 | 单 Pack 不应承担过大路由表；超过应拆 Pack。 |
| `backend.capabilities` | 64 | capability 是 cross-Pack 协调原语，不是任意 string label。 |
| `frontend.routes` | 64 | 单 Pack 在前端 navbar 上不应占太多坑。 |
| `metadata` 总字节 | 16 KiB | metadata 不是文档存储位。 |
| `description` | 1 KiB UTF-8 | 同上。 |

超限不阻断 manifest 加载（避免 silent breakage），但 catalog 必须在 `warnings[]` 显式上报。

### 3.4 即将加入的字段（spec 锁定，待实现）

下述字段尚未出现在 Go struct，本 spec 视为 v0.2 manifest schema 的预留位。运行时遇到未知字段必须 *忽略而不报错*（forward compat）。

```jsonc
{
  "abi": "1",                            // pack runtime ABI 主版本，§5.1
  "publishedAt": "2026-05-28T10:00:00Z",
  "publisher": {
    "id": "yunque-official",
    "publicKeyId": "yunque-official-2026-05"
  },
  "dependencies": [                      // §3.5
    { "id": "yunque.pack.cogni-kernel", "requires": ">=0.1.0" }
  ],
  "distribution": {
    "platforms": ["linux/amd64", "darwin/arm64", "windows/amd64"],
    "mirrors": [                         // §6.2
      { "kind": "github-release", "url": "https://github.com/.../releases/download/v0.1.0/<file>.yqpack" },
      { "kind": "oss",            "url": "https://yunque-packs.oss-cn-shanghai.aliyuncs.com/<file>.yqpack" },
      { "kind": "cos",            "url": "https://yunque-1234.cos.ap-guangzhou.myqcloud.com/<file>.yqpack" }
    ]
  },
  "signing": {
    "algorithm": "ed25519",
    "manifestSha256": "<hex>",           // 整个 pack.json 去掉 signing 段后的 sha256
    "signature": "<base64>"              // ed25519 over manifestSha256
  }
}
```

### 3.5 `dependencies` 语义

- 解析 *显式*，不做 transitive 自动安装。`/v1/packs/install` 在依赖未满足时返回 `409 Conflict` + 缺失列表，由 UI 引导用户先装依赖。
- 禁止循环依赖；catalog 构建期检测，运行时不复检。
- 依赖只看 `id`+`requires` 范围，不绑定具体 sha256。同一依赖 Pack 升级补丁版本时不应让所有依赖方 manifest 重发。

---

## 4. 签名与可信链

### 4.1 信任根

运行时启动时从两处加载发行者公钥：

1. **嵌入根**：`internal/packruntime/trustroot.go` 内编译期常量，包含 `yunque-official` 主密钥。
2. **磁盘扩展**：`<config>/packs/trust/*.pub`，仅当用户 *主动* 通过 UI 添加第三方 publisher 时落盘。

> 设计意图：用户不需要 PKI / CA。只要他们信任安装的云雀本体，就自动信任 `yunque-official` 公钥；想装第三方 Pack，需要在 UI 显式 *Add Publisher* 一次。

### 4.2 manifest 签名规则

签名材料：**整个 `pack.json` 文件序列化为 canonical JSON 后，去掉 `signing` 段**。

- canonical JSON：UTF-8、按 key 字典序、无空白、数字按 RFC 8785。
- 签名算法：`ed25519`（保留 `algorithm` 字段以容纳未来切换）。
- `manifest.sig` 内容：`base64(ed25519(canonical_json_without_signing))`。
- `manifest.pub` 内容：`<publisher.id>:<publisher.publicKeyId>` 单行，*不含公钥字节本身*。公钥字节通过 §4.1 的信任根查找。

### 4.3 ZIP 容器签名

`pack.json` 签名只覆盖 manifest。entry 的完整性靠两条独立链：

1. **artifact-level**：分发服务器在 catalog 中暴露 `distribution.sha256`，运行时 `CacheDistribution()` 已实现 sha256 校验（见 `pkg/packruntime/registry.go:198-214`）。
2. **inner-level**：manifest 的 `entries[].sha256`（v0.2 字段）覆盖 ZIP 内每个文件，使部分文件被替换的攻击在解压期被发现。

inner-level 实现优先级 P1（v0.2），manifest schema 已留位。

### 4.4 验证流程

```
download .yqpack → 校验 sizeBytes ≤ 配置上限
                → 流式 sha256，命中 distribution.sha256 才落盘到 cached/
                → 解压到 staging/<id>-<version>/
                → 读取 pack.json + manifest.sig + manifest.pub
                → 用 §4.1 信任根解析公钥
                → ed25519 verify
                → 通过：原子 rename staging → installed/<id>-<version>/
                → 失败：删除 staging，触发 ChangeReason 'install-failed'（待加入）
```

任何一步失败都不允许部分写入 `installed.json`。

### 4.5 公钥撤销

未实现。SPEC 的姿态：撤销靠 *运行时升级时替换嵌入根*，不引入在线 OCSP/CRL 链路。这是有意的简化——撤销窗口 = 用户更新云雀的频率。

---

## 5. 生命周期 FSM

### 5.1 ABI 版本

`abi` 字段独立于 `version`：

- `abi=1`：当前所有内置 Pack。Pack 与 Core 通过 HTTP route + `BackendModule` 接口对接。
- `abi=2`：subprocess + gRPC 路径（计划中，见 §7）。

Core 加载 Pack 时 *先看 abi*：abi 不匹配立即拒绝，错误信息指明需要的 abi 范围。这避免老 Core 加载新 Pack 时进入运行期不可预测状态。

### 5.2 状态机

```
       install            enable
NotInstalled ──────► Disabled ──────► Enabled
       │                ▲   │           │
       │                │   │ disable   │
       │      install   │   ◄───────────┘
       │     (re-add)   │
       │                │
       │  uninstall     │ rollback
       └────────────────┘ (target version becomes Enabled
                          if it was Enabled before update)

Enabled / Disabled  ──update──► Enabled / Disabled  (与升级前同)
                                             │
                                             └─rollback─► 上一版本（Artifacts 互换）
```

`Registry.Rollback()` 已实现 Artifacts 与 Version 的对调（见 `pkg/packruntime/registry.go:358-381`）。SPEC 锁定的额外要求：

- `update.rollback=false` 的 Pack 不允许调用 rollback API；尝试调用返回 `409 Conflict`。
- rollback 之后 `previousVersion` 与 `previousArtifacts` 被新的 swap 覆盖；不保留更深 history。多级回滚不支持。
- enable/disable 状态在 update / rollback 中保持。即"升级前是 Enabled，升级后仍 Enabled"。

### 5.3 ChangeReason 扩展

`pkg/packruntime/registry.go` 当前定义：`install / update / enable / disable / rollback`。本 spec 要求加入：

- `install-failed`：staging 通过但 verify 失败时发出，让 audit 链不丢事件。
- `uninstall`：当前未实现（registry 没有 Remove API）。SPEC 锁定 v0.2 必须实现，并发 `uninstall` 事件，artifacts 进入 GC 候选。

### 5.4 平台 mismatch

如果 host 平台不在 `distribution.platforms` 内：

- `/v1/packs/catalog` 仍展示该 Pack，但加 `unsupportedPlatform: true`。
- `/v1/packs/install` 拒绝，错误码 `pack.unsupported_platform`，message 包含 host 与可用平台列表。
- 已 installed 的 Pack 升级后变为 unsupported（开发者删平台支持）：保留磁盘 artifacts，但运行时 enable 时拒绝并提示。

---

## 6. 分发拓扑

### 6.1 双源分发

发布渠道分两层：

1. **catalog index**（轻量 JSON）：发布到 GitHub Pages / OSS 静态站点，`distribution.manifestUrl` 指向。catalog 客户端定期拉取做 diff，不依赖镜像列表。
2. **artifact 重型文件**（`.yqpack`）：通过 `distribution.mirrors[]` 列出多个等价镜像。

### 6.2 镜像选择策略

运行时的 mirror 选择策略（v0.2 实现，由 SPEC 锁定）：

```
对 mirrors[] 中每条 mirror：
  并发发起 HTTP HEAD（带 200ms 时间戳基线）
  收集前 3 个返回的 mirror，按 (status==200) DESC, latency ASC 排序
选出 head；流式下载时遇到任意 5xx / 网络中断 → 切下一个 head
全部失败 → 报错并保留已下载部分供下次断点续传
```

不允许跑测速 benchmark（绕远）。HEAD 探测是为了 *规避明显 dead mirror*，不是为了选最快。

### 6.3 镜像 kind 约束

| `kind` | 约束 |
| --- | --- |
| `github-release` | 公网 HTTPS，免认证。海外用户首选。 |
| `oss` | 阿里云对象存储 (`*.aliyuncs.com`)。国内首选。 |
| `cos` | 腾讯云对象存储 (`*.myqcloud.com`)。国内备选。 |
| `qiniu` | 七牛云。预留。 |
| `mirror` | 通用 HTTPS。允许社区镜像，但前端 UI 需要标记 *第三方镜像，artifact 仍走签名验证*。 |

镜像 kind 不影响校验链：所有 mirror 都拉同一 sha256 artifact，校验链不松。kind 只服务 UX（按地理决定排序）。

### 6.4 catalog source 隔离

`internal/controlplane/gateway/handlers_packs.go:319-323` 的 `packCatalogSourceDirs()` 现已支持多 source。本 spec 锁定 v0.2 加入 *远程 source*：

- `PACK_CATALOG_SOURCES` 支持 `https://...` URL，运行时拉取 JSON 数组并视作只读 catalog。
- 每个远程 source 必须有自己的发行者公钥配置；远程 source 提供的 manifest 仅来自该 source 的 publisher。
- 远程 source 不影响 *已安装* Pack 的可信链——已安装的 manifest 签名仍按 §4 验证。

---

## 7. 子进程 + gRPC 路径（abi=2）

当前 `pkg/packruntime` 与 `internal/packs/*` 的 BackendModule 是 in-process Go 接口。SPEC 锁定 abi=2 的目标形态，保持 manifest schema 不变，只新增 *如何把 backend 二进制 wire 起来* 的契约。

### 7.1 进程模型

- 启用 Pack → Core fork `backend/<goos>-<goarch>/yunque-pack-<id>` 作为子进程。
- 子进程必须接受 `--socket <path>` 参数，绑定 Unix domain socket / Windows named pipe，*不绑定 TCP*。
- 子进程必须实现 `yunque.plugin.v1.Plugin` gRPC service（接口 stub 在 `sdk/go/yunque` 中，已部分构建）。
- Core 与子进程通过 `Plugin.Health()` 心跳，超过 `5 × heartbeatInterval` 无响应即视为崩溃，触发 disable。

### 7.2 协议表面（v0.2 锁定）

```protobuf
service Plugin {
  rpc Initialize(InitRequest) returns (InitResponse);
  rpc HandleRoute(RouteRequest) returns (stream RouteChunk);    // backend.routes 落到这里
  rpc InvokeCapability(CapabilityRequest) returns (CapabilityResponse);
  rpc Health(google.protobuf.Empty) returns (HealthResponse);
  rpc Shutdown(google.protobuf.Empty) returns (google.protobuf.Empty);
}
```

`HandleRoute` 用 server-streaming 是为了把现有 SSE/WebSocket 路由直接 forward。目前内置 Pack 没有 streaming 路由，但 SPEC 不能假设永远没有。

### 7.3 与现有 BackendModule 的关系

- abi=1 Pack：仍走 in-process `BackendModule`。
- abi=2 Pack：Core 注册一个 *gRPC 桥接 BackendModule*，把 `Routes()` 指向远端，对 gateway 而言透明。

迁移路径：内置 Pack 按需迁移到 abi=2，没有截止日期。abi=1 永久支持。

---

## 8. CI 与发布流程

### 8.1 构建矩阵

`Makefile` 必须提供 `make pack PACK=<dir>` target：

```
对每个 platform in distribution.platforms：
  GOOS=... GOARCH=... go build -trimpath -ldflags '-s -w -buildid='
       -o <staging>/backend/<goos>-<goarch>/yunque-pack-<id>[.exe] <pack>/cmd/...
ts/wasm/frontend 资产分别按 pack.json 指引 build
按 §2.1 deterministic ZIP 打包 → <pack-id>-<version>.yqpack
计算 sha256，写回 manifest.json（catalog 用）
对 manifest.json 做 ed25519 签名（KEY 来自 CI secret）
```

`-trimpath -buildid=` 是 deterministic build 的关键。

### 8.2 GitHub Release 自动化

`scripts/release-pack.mjs`（v0.2 创建）：

- 输入：tag `pack/<id>/v<x.y.z>`。
- 动作：调 `make pack`、生成 catalog patch、上传 `.yqpack` 到 GitHub Release、上传到 OSS/COS 镜像。
- 失败回滚：所有镜像同步未成功前不 publish catalog。catalog 是 *最后一步*，使 `manifestUrl` 永远指向 *已经齐全* 的 artifacts。

### 8.3 SDK 同步

Pack 发布若改变 `sdk.typescript / sdk.go / sdk.python`，必须触发 `npm run check:sdk-manifests --prefix packages/yunque-client`（已有 `sdk/scripts/check-packs-sdk-manifest.mjs`）。CI 失败必须阻断发布。

---

## 9. 与 OpenAPI / SDK 演化的边界

`docs/spec/openapi-evolution.md` 管 *control plane API* 的演化。本 spec 管 *Pack 容器 + manifest + 信任链*。重叠点只有一处：

- 如果 Pack 在 `backend.routeSpecs` 引入了新 route，且这些 route 被 `cmd/openapi-gen` 抓走暴露到 OpenAPI，那么 OpenAPI breaking-change 规则同时适用。

冲突时按更严格的判定：只要任一规范认为 breaking，就按 breaking 走。

---

## 10. Review checklist

每次 Pack 发布 PR 至少回答：

- 是否更新了 `version`？是否符合 SemVer 含义（patch / minor / breaking）？
- `requiresCore` 是否还正确？升级 Core 是否会让该 Pack 无法装载？
- `distribution.platforms` 是否覆盖目标用户？被删除的平台是否做了 deprecation 通知？
- `distribution.sha256` 是否对应实际 artifact？CI 重放 build 是否得到相同 sha256？
- 新增 `dependencies` 是否会引入循环？
- manifest 签名是否由 §4.1 信任根中的 publisher 签出？
- 是否触发了 `check:sdk-manifests` / `check:openapi-evolution` 等下游同步检查？
- 如果引入 abi=2 子进程：心跳间隔、shutdown timeout、socket 路径是否在每个目标平台都验证？

---

## 11. 已建 vs 待建对照

| 章节 | 状态 | 锚点 |
| --- | --- | --- |
| §2 `.yqpack` 容器 | 待建 | 现 `cmd/yunque-plugin` 用 `.zip` |
| §3.1 已规范字段 | 已建 | `pkg/packruntime/manifest.go` |
| §3.4 v0.2 字段（abi/publisher/dependencies/mirrors/signing） | 待建 | manifest struct 需要扩展 |
| §4 签名与可信链 | 待建 | `manifest.sig`、`trustroot.go` 未实现 |
| §5.2 状态机（install/enable/disable/update/rollback） | 已建 | `pkg/packruntime/registry.go` |
| §5.3 `install-failed` / `uninstall` ChangeReason | 待建 | registry 需扩展 |
| §6.2 镜像 failover | 待建 | `CacheDistribution` 当前只用单 URL |
| §7 abi=2 subprocess+gRPC | 部分建 | `cmd/yunque-plugin` + `sdk/go/yunque` 提供基础 |
| §8.1 deterministic build matrix | 待建 | Makefile 当前无 `make pack` |

> 这张表是后续 PR 的工作面，不是承诺时间点。Pack 分发是渐进式工程，不是大版本重写。
