# Cogni Declaration 公开规范

`docs/spec/cogni-declaration.schema.json` 是 Cogni Declaration 的语言无关 JSON Schema。它面向第三方 Pack / SDK / 管理台 / 自动化脚本，让外部作者可以在不读取 Go 源码的情况下编写、校验和解释 Cogni 声明。

当前规范镜像的是 `pkg/cogni.Declaration` 的公开 JSON/YAML shape；Go 代码仍然是运行时事实源，schema 是对外契约和工具入口。

## 最小声明

```json
{
  "id": "code-reviewer"
}
```

`id` 是唯一必填字段，会被 registry、trace、bundle、API 和 SDK 用作稳定锚点。建议使用可读、稳定、短横线或点号分隔的 ID，例如 `code-reviewer`、`office.writer`、`team-a.review`。

## 完整示例

```json
{
  "id": "code-reviewer",
  "display_name": "Code Reviewer",
  "description": "Activates for code review requests and exposes review tools.",
  "capsule": "code-reviewer",
  "priority": 50,
  "exclusive": "review",
  "activation": {
    "min_score": 0.4,
    "keywords": ["review", "审查"],
    "keyword_weight": 0.3,
    "regex": ["^review\\s+#\\d+"],
    "regex_weight": 0.5,
    "channels": ["webchat"],
    "tenants": ["team-a"],
    "handover_on": ["need-review"],
    "perception": [
      {
        "type": "file_watcher",
        "patterns": ["**/*.go"],
        "events": ["modified"],
        "weight": 0.2
      }
    ]
  },
  "surface": {
    "only": ["github_get_diff"],
    "include": ["github_post_comment"],
    "exclude": ["github_delete_comment"],
    "from_capsules": ["code-reviewer"],
    "max_tools": 8
  },
  "context": {
    "static": "你是一名资深代码审查员。",
    "memory_query": "code review for {message}",
    "memory_top_k": 5,
    "template": "上下文: {{.Message}}"
  },
  "memory": {
    "namespace": "code-reviewer",
    "drop_keys": ["secret"],
    "tag_all": { "source": "review" }
  },
  "checks": [
    {
      "name": "review keyword activates",
      "message": "please review this PR",
      "expect_active": true,
      "expect_score_at_least": 0.4
    }
  ]
}
```

## 字段语义

| 字段 | 语义 | 运行时来源 |
| --- | --- | --- |
| `id` | 稳定唯一 ID。 | `pkg/cogni.Declaration.ID` |
| `display_name` | UI 展示名。 | `DisplayName` |
| `description` | Cogni 用途摘要。 | `Description` |
| `capsule` | 绑定的 Capsule ID；可为空。 | `Capsule` |
| `activation` | 何时激活。关键词、正则、租户、渠道、handover、多模态 perception 都在这里。 | `ActivationRules` |
| `surface` | 激活后暴露哪些工具/技能。 | `ToolSurface` |
| `context` | 激活后注入 planner system prompt 的上下文。 | `ContextInjection` |
| `mcp` | Cogni 专属 MCP server 与工具过滤。 | `MCPConfig` |
| `workflows` | 多步骤工作流定义。 | `WorkflowDef` |
| `experience` | 经验引擎配置。 | `ExperienceConfig` |
| `economics` | 单次/每日预算、优先权重、缓存共享。 | `EconomicsConfig` |
| `memory` | 记忆提取后的丢弃、打标、命名空间策略。 | `MemoryPolicy` |
| `priority` | 多 Cogni 同时激活时的排序，数字越小优先级越高。 | `Priority` |
| `exclusive` | 互斥组，同组只保留最合适的 Cogni。 | `Exclusive` |
| `checks` | 声明级自测，类似 declarative agent 的 CI。 | `ActivationCheck` |

## 设计边界

- Schema 描述的是**声明文件形状**，不是运行时 trace / stats / registry status。
- Schema 不尝试编译正则表达式；Go runtime 的 `Validate()` 仍负责正则合法性检查。
- Schema 不尝试验证 `memory_query`、`template`、`workflow.condition` 的业务语义；这些由运行时 evaluator / workflow engine 负责。
- Schema 对 `perception.type` 和 `mcp.transport` 使用枚举，是为了第三方声明的可移植性；如果 Go 运行时未来支持新值，必须同步更新 schema 和本文档。
- `checks` 至少应声明一个期望：`expect_active`、`expect_score_at_least` 或 `expect_reason_contains`。

## 校验入口

```powershell
node scripts/check-cogni-declaration-schema.mjs
```

该脚本会检查：

- `docs/spec/cogni-declaration.schema.json` 存在并使用 JSON Schema draft 2020-12。
- schema 顶层字段覆盖 `pkg/cogni.Declaration` 的公开 JSON 字段。
- schema 包含 activation / surface / context / mcp / workflows / experience / economics / memory / checks 的 `$defs`。
- 本语义说明保留与 `pkg/cogni.Declaration`、schema 路径和校验命令的链接。

## 演化规则

- 新增 optional 字段：minor，可直接扩展 schema。
- 新增 required 字段：breaking，必须提供迁移方案。
- 重命名字段：breaking，必须保留兼容窗口或转换器。
- 改变 `id` 语义、activation score 语义、surface 过滤顺序、memory 默认行为：breaking。
- 新增 perception / MCP transport 枚举：minor，但需要同步 Go evaluator / connector / schema / 文档。

## 与 OpenAPI / SDK 的关系

OpenAPI 里 `/v1/cognis` 当前仍把 declaration 当作通用 object 处理；本 schema 是更精确的外部作者契约。后续可以把 `docs/openapi.yaml` 中 Cogni declaration body/response `$ref` 到这个 schema，或在 SDK 中生成更强类型的 `CogniDeclaration`。
