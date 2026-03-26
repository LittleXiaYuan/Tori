# OPP-Go

[![Go Reference](https://pkg.go.dev/badge/github.com/LittleXiaYuan/opp-go.svg)](https://pkg.go.dev/github.com/LittleXiaYuan/opp-go)
[![License: MPL-2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)

**Open Plugin Protocol** 的纯 Go 实现。

OPP 是一个 Agent 间任务委托协议。它定义了：如何通过「意图」发起任务、如何通过状态机追踪执行、以及如何在执行过程中进行交互式协商（QUESTION/ANSWER、PROBLEM/DECIDE）。

## 安装

```bash
go get github.com/LittleXiaYuan/opp-go
```

## 快速上手

```go
package main

import (
    "fmt"
    "github.com/LittleXiaYuan/opp-go"
)

func main() {
    // 1. 发送意图
    msg := opp.NewIntent("caller", "deploy-agent", "session-1",
        opp.IntentEnvelope{Name: "ops.deploy", Version: "1.0", Payload: map[string]string{"app": "myapp"}},
    )

    // 2. 对方接受
    accept := opp.NewAccept("deploy-agent", "caller", "session-1", "task-1")

    // 3. 追踪状态
    state := opp.StatePending
    state, _ = opp.Transition(state, accept)   // → accepted

    progress := opp.NewProgress("deploy-agent", "caller", "session-1", "task-1", "build", 0.5, "编译中...")
    state, _ = opp.Transition(state, progress) // → running

    // 4. 遇到问题，暂停等决策
    problem := opp.NewProblem("deploy-agent", "caller", "session-1", "task-1", opp.ProblemPayload{
        Severity: "error", Category: "port_conflict",
        Description: "端口 8080 已被占用",
        Options: []opp.ProblemOption{
            {Value: "kill", Label: "杀掉旧进程", Risk: "moderate"},
            {Value: "change", Label: "改用 8081", Risk: "safe"},
        },
    })
    state, _ = opp.Transition(state, problem) // → blocked

    // 5. 做出决策
    decide := opp.NewDecide("caller", "deploy-agent", "session-1", "task-1", "", "change", "更安全")
    state, _ = opp.Transition(state, decide)  // → running

    // 6. 完成
    result := opp.NewResult("deploy-agent", "caller", "session-1", "task-1", "success", "已部署到 :8081", nil)
    state, _ = opp.Transition(state, result)  // → completed

    fmt.Println("最终状态:", state)

    // 序列化 / 反序列化
    data, _ := msg.Bytes()
    parsed, _ := opp.ParseMessage(data)
    intent, _ := parsed.DecodeIntent()
    fmt.Println("意图:", intent.Intent.Name)
}
```

## 这个库做什么

- **消息信封** — `Message` 结构体 + `json.RawMessage` 多态载荷
- **类型化载荷** — `IntentPayload`、`ResultPayload`、`ProblemPayload`、`QuestionPayload` 等
- **构造函数** — `NewIntent()`、`NewResult()`、`NewQuestion()`、`NewAnswer()`、`NewProblem()`、`NewDecide()` 等
- **状态机** — 纯函数 `Transition(state, msg)` 执行 OPP 任务生命周期
- **校验** — `Validate()` 拒绝不合规的消息

## 这个库不做什么

- 不做网络传输（WebSocket / HTTP / gRPC）——自己接
- 不做会话管理
- 不做沙盒执行或文件系统操作
- 不做任务调度或 goroutine 管理
- 不包含任何宿主运行时逻辑

## 状态机

```
pending ──ACCEPT──► accepted ──PROGRESS──► running ──RESULT──► completed
   │                    │                     │    │
   │                    │                     │    └──RESULT──► failed
   └──REJECT──► failed  ├──QUESTION──► waiting_input ──ANSWER──► running
                        └──PROBLEM───► blocked ───────DECIDE──► running

任何非终态 ──CANCEL──► cancelled
```

## 消息类型

| 层级 | 类型 |
|------|------|
| 协议层 | HELLO, WELCOME, BYE, PING, PONG, ACK, ERROR, CANCEL, RESUME |
| 业务层 | INTENT, ACCEPT, REJECT, RESULT, QUESTION, ANSWER, PROGRESS, PROBLEM, DECIDE, OBSERVATION, ACTION_TAKEN, HEARTBEAT |
| 网络层 (v3) | NOTIFY, SUBSCRIBE, UNSUBSCRIBE, EVENT, DELEGATE, DELEGATE_RESULT, DISCOVER, CAPABILITIES |

## 协议

[MPL-2.0](LICENSE)
