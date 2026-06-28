# Cogni / CogniSDK 合并方案

## 背景

2026-06 审计发现 `pkg/cogni`（Declaration 激活）和 `pkg/cognisdk`（Pack 感知）是两套并行系统，各自独立激活、各自往 prompt 注入 context，没有统一仲裁。这导致：

- 同一条消息被两套系统各评估一次，激活判定重复
- prompt 里 `cogni` 和 `belief` 两个 layer 并存，内容可能重叠，破坏 prompt cache
- belief scope gate（#34）只在 cognisdk 那条路生效，cogni Declaration 激活的 context 不受 scope gate 保护
- 对外宣称的「三位一体」在代码层面是断头路各跑一半

## 合并目标

一个激活判定，一个 context layer，一个 CogniRuntime interface。

```
现状（双轨）：                    合并后（单轨）：
┌──────────┐ ┌──────────┐        ┌─────────────────────┐
│ pkg/cogni│ │cognisdk  │        │ 统一 CogniRuntime   │
│ 激活判定1│ │ 激活判定2│   →    │  ├ cogni Declaration │← 能力层（tool/MCP/filter）
│ context 1│ │ context 2│        │  ├ cognisdk Pack    │← 信念层（scope/belief）
└────┬─────┘ └────┬─────┘        │  └ 统一激活+统一注入 │
     ▼            ▼               └──────────┬──────────┘
   prompt layer  prompt layer                ▼
   (cogni)       (belief)              prompt layer (cogni)
```

## 设计原则

- **Declaration 是显式声明，Pack 是隐式感知**——Declaration 决定「这个 cogni 激活吗」，Pack 决定「激活后 belief scope 怎么过滤」
- cogni 的 FilterSkills/mergeCogniTools/SurfaceAuthoritative 是能力层，cognisdk 没有这些——保留 cogni 主导
- cognisdk 的 perception/disposition/belief scope gate 是信念层，cogni 没有这些——保留 cognisdk 主导
- 两者能力互补，不消灭任何一个，只统一注入路径

## 分步执行

### Step 1 — cognisdk HostAdapter 实现 CogniRuntime interface（最安全，先做）

- 给 `HostAdapter` 补 `FilterSkills/Tools/SurfaceAuthoritative/Trace/RecordToolOutcome` 方法
- cognisdk 本身不做 skill 过滤——这些方法委托给持有的 cogni Hook 引用
- HostAdapter 持有 `*cogni.Hook` 字段（可选，nil 时退化为只走 cognisdk Pack）
- 不动 prompt_builder，不动 init，纯加方法
- **验收：** HostAdapter 满足 CogniRuntime interface，编译通过，现有测试不挂

### Step 2 — 统一注入入口

- `CogniRuntime.BuildContext` 签名加 `scope string` 参数（对齐 BeliefContextFunc）
- prompt_builder 的 P3.6 cogni + P3.7 belief **合并成一个 layer**（名 `cogni`）
- BuildContext 内部：先跑 cogni Declaration 激活 → 再跑 cognisdk Pack perception → concat 两者 context
- `BeliefContextFunc` 标记 deprecated，prompt_builder 不再单独调它
- **验收：** prompt 里只有一个 cogni layer，belief scope gate 仍生效（通过 BuildContext 内部）

### Step 3 — 统一激活判定

- cogni 的 `evaluate` 和 cognisdk 的 `detectPerception` 合成一个
- Declaration 激活优先，Pack perception 作为 Declaration 激活后的内部状态补充
- 即：Declaration 决定「激活吗」，Pack 决定「激活后 scope 怎么算」
- **验收：** 同一条消息只走一次激活判定，两条路径结果一致

### Step 4 — 删死代码

- `BeliefContextFunc` 类型 + `SetBeliefContext` setter 删掉
- `ActiveLoop.Handle` 如果 Step 1-3 后仍无人用，删掉或明确标记
- ImmuneBridge 同上（已在 ade80d9f 标记 NOT WIRED）
- **验收：** `grep BeliefContextFunc` 在生产代码零匹配

## 风险点

1. **Step 2 prompt cache 影响**——两个 layer 合一个，短期 cache 命中率可能掉，长期更稳（layer 数少了）
2. **Step 3 激活语义变化**——现在 cogni 和 cognisdk 各自激活互不影响，合并后串行依赖，可能有激活顺序 bug
3. **#34 scope gate 接入点迁移**——scope 从 prompt_builder 推导传给 BeliefContextFunc，改成传给 CogniRuntime.BuildContext

## 当前状态

- 方案已定，待审
- Step 1 待开工
