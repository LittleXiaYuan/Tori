package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/experimental/heartbeat"
)

type heartbeatModule struct {
	service *heartbeat.Service
}

func (m *heartbeatModule) Name() string        { return "heartbeat" }
func (m *heartbeatModule) Description() string { return "周期性自省心跳，审视状态与收件箱" }
func (m *heartbeatModule) Profile() string     { return "full" }

func (m *heartbeatModule) Init(ctx context.Context, app *agentrt.App) error {
	p := app.Planner
	inboxStore := app.MustGet(agentrt.CompInbox).(*inbox.Store)

	hbInterval := 30
	if v := os.Getenv("HEARTBEAT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hbInterval = n
		}
	}

	m.service = heartbeat.New(heartbeat.Config{
		Enabled:  true,
		Interval: time.Duration(hbInterval) * time.Minute,
		MaxLogs:  200,
	}, func(ctx context.Context) (string, error) {
		summary := inboxStore.Summary(5)
		prompt := "这是你的心跳时间。请审视你的状态、回顾收件箱并做任何需要的维护工作。"
		if summary != "" {
			prompt += "\n\n" + summary
		}
		result, err := p.Run(ctx, planner.PlanRequest{
			Messages: []llm.Message{{Role: "user", Content: prompt}},
			TenantID: "system",
		})
		if err != nil {
			return "", err
		}
		return result.Reply, nil
	})
	m.service.SetOnResult(func(log *heartbeat.Log, policy *heartbeat.DeliveryPolicy) {
		app.Metrics.Cognitive().HeartbeatRun.Add(1)
		slog.Info("heartbeat delivered", "status", log.Status, "targets", len(policy.Targets))
	})

	app.Set(agentrt.CompHeartbeat, m.service)
	return nil
}

func (m *heartbeatModule) Start(ctx context.Context) error {
	m.service.Start(ctx)
	return nil
}

func (m *heartbeatModule) Stop() error {
	m.service.Stop()
	return nil
}

func (m *heartbeatModule) Status() agentrt.ModuleStatus {
	running := m.service != nil && m.service.IsRunning()
	return agentrt.ModuleStatus{
		Name:    m.Name(),
		Profile: m.Profile(),
		Enabled: running,
		Running: running,
	}
}
