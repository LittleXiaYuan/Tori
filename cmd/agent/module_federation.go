package main

import (
	"context"
	"log/slog"
	"os"

	"yunque-agent/internal/agentcore/federation"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/controlplane/gateway"
)

type federationModule struct {
	hub *federation.Hub
}

func (m *federationModule) Name() string        { return "federation" }
func (m *federationModule) Description() string { return "跨实例联邦通信，共享智体能力" }
func (m *federationModule) Profile() string     { return "full" }

func (m *federationModule) Init(ctx context.Context, app *agentrt.App) error {
	cfg := app.Config
	m.hub = federation.NewHub(federation.HubConfig{
		LocalAgent:    "yunque",
		LocalInstance: cfg.Addr,
		Secret:        os.Getenv("FEDERATION_SECRET"),
	})
	app.Set(agentrt.CompFederationHub, m.hub)

	if gwRaw, ok := app.Get(agentrt.CompGateway); ok {
		if gw, ok := gwRaw.(*gateway.Gateway); ok {
			gw.SetFederationHub(m.hub)
		}
	}
	slog.Info("federation hub initialized")
	return nil
}

func (m *federationModule) Start(_ context.Context) error {
	return nil
}

func (m *federationModule) Stop() error {
	return nil
}

func (m *federationModule) Status() agentrt.ModuleStatus {
	return agentrt.ModuleStatus{
		Name:    m.Name(),
		Profile: m.Profile(),
		Enabled: m.hub != nil,
		Running: m.hub != nil,
	}
}
