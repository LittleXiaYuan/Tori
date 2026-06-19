package gateway

import "context"

// SetDesktopSandboxStatusProvider lets the sandbox pack expose its cloud
// desktop status to other packs (notably computer-use) without gateway owning
// the sandbox routes or mutable desktop state.
func (g *Gateway) SetDesktopSandboxStatusProvider(fn func(context.Context) map[string]any) {
	g.desktopStatusProvider = fn
}

// DesktopSandboxStatus exposes a read-only cloud desktop snapshot for packs
// that need computer-use readiness without gaining create/destroy privileges.
func (g *Gateway) DesktopSandboxStatus(ctx context.Context) map[string]any {
	if g.desktopStatusProvider != nil {
		return g.desktopStatusProvider(ctx)
	}
	return map[string]any{"ok": true, "available": false, "running": false}
}
