package gateway

import (
	"log/slog"
	"net/http"
	"strings"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/internal/packs/wasmroute"
	"yunque-agent/pkg/packruntime"
)

// dynRoute is one runtime-mounted pack route. Unlike the boot-time
// backendPack routes (which live forever on the append-only http.ServeMux),
// dynamic routes are held in a map we own, so install/enable can mount them
// and disable/uninstall can remove them.
type dynRoute struct {
	packID  string
	methods []string
	path    string
	handler http.HandlerFunc // already wrapped with auth + enabled gate
}

// dynamicDispatch returns a handler that checks the dynamic route table first
// and falls through to next (the static mux) on a miss. It is inserted inside
// the middleware chain so dynamic routes get the same cross-cutting middleware
// as everything else.
func (g *Gateway) dynamicDispatch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.routesMu.RLock()
		route := g.dynamicRoutes[r.URL.Path]
		g.routesMu.RUnlock()
		if route != nil {
			route.handler(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// mountWasmPack builds and registers dynamic routes for a wasm-backed pack.
// Existing routes for the same pack are replaced. installedDir is the
// extracted pack directory (root/installed/<id>-<version>).
func (g *Gateway) mountWasmPack(pack packruntime.InstalledPack, installedDir string) {
	rt := pack.Manifest.Backend.Runtime
	if rt == nil || rt.Type != packruntime.RuntimeTypeWasm {
		return
	}
	packID := pack.Manifest.ID
	sb := g.wasmSandbox()

	g.routesMu.Lock()
	defer g.routesMu.Unlock()
	if g.dynamicRoutes == nil {
		g.dynamicRoutes = make(map[string]*dynRoute)
	}
	// Drop any prior routes from this pack before remounting (under the lock).
	for p, dr := range g.dynamicRoutes {
		if dr.packID == packID {
			delete(g.dynamicRoutes, p)
		}
	}
	mounted := 0
	for _, spec := range pack.Manifest.Backend.RouteSpecs {
		p := strings.TrimSpace(spec.Path)
		if p == "" {
			continue
		}
		// A dynamic pack route must not shadow a static (boot-time) pack route.
		if owner, ok := g.backendPackRoutes[p]; ok {
			slog.Warn("wasm pack route conflicts with static pack route; skipping",
				"pack", packID, "route", p, "owner", owner)
			continue
		}
		methods := []string{strings.ToUpper(strings.TrimSpace(spec.Method))}
		gated := g.backendPackAuth(packruntime.BackendRouteAuthDefault,
			g.requirePackRoute(packID, methods, p, wasmroute.BuildRouteHandler(installedDir, *rt, spec, sb)))
		g.dynamicRoutes[p] = &dynRoute{packID: packID, methods: methods, path: p, handler: gated}
		mounted++
	}
	slog.Info("mounted wasm pack routes", "pack", packID, "routes", mounted)
}

// unmountPack removes all dynamic routes belonging to a pack. This is the
// capability the append-only http.ServeMux cannot provide; it is the path a
// future pack-uninstall will use. enable/disable do NOT unmount — they leave
// routes mounted and let requirePackRoute gate the disabled state to 404.
func (g *Gateway) unmountPack(packID string) {
	g.routesMu.Lock()
	defer g.routesMu.Unlock()
	for p, dr := range g.dynamicRoutes {
		if dr.packID == packID {
			delete(g.dynamicRoutes, p)
		}
	}
}

// wasmSandbox lazily builds the shared sandbox used by all wasm pack routes.
func (g *Gateway) wasmSandbox() *sandbox.WasmSandbox {
	g.wasmSandboxOnce.Do(func() {
		g.wasmSandboxInstance = sandbox.NewWasmSandbox(sandbox.DefaultWasmConfig())
	})
	return g.wasmSandboxInstance
}

// wireWasmPacks subscribes to registry change events so wasm-backed packs are
// mounted on install/enable and unmounted on disable, and pre-mounts any
// already-enabled wasm packs at startup. In-process (first-party) packs are
// untouched — they continue to mount at boot via registerBuiltinBackendPacks.
//
// Safe to call more than once (NewFromConfig + SetPackRegistry); the OnChange
// subscription is installed at most once per gateway.
func (g *Gateway) wireWasmPacks() {
	if g.packRegistry == nil {
		return
	}
	g.routesMu.Lock()
	if g.wasmWired {
		g.routesMu.Unlock()
		return
	}
	g.wasmWired = true
	g.routesMu.Unlock()

	for _, pack := range g.packRegistry.List() {
		if pack.Manifest.Backend.IsWasm() {
			g.mountWasmPack(pack, g.packRegistry.InstalledDir(pack.Manifest.ID, pack.Manifest.Version))
		}
	}
	g.packRegistry.OnChange(func(ev packruntime.ChangeEvent) {
		if !ev.Pack.Manifest.Backend.IsWasm() {
			return
		}
		packID := ev.Pack.Manifest.ID
		// Routes stay mounted across enable/disable; the requirePackRoute gate
		// returns 404 while disabled, mirroring static first-party packs. We
		// (re)mount on install/update/rollback so route changes take effect,
		// and on enable as a safety net for manifest-only installs that later
		// get their module extracted.
		switch ev.Reason {
		case packruntime.ChangeReasonInstall, packruntime.ChangeReasonUpdate, packruntime.ChangeReasonEnable, packruntime.ChangeReasonRollback:
			g.mountWasmPack(ev.Pack, g.packRegistry.InstalledDir(packID, ev.Pack.Manifest.Version))
		case packruntime.ChangeReasonDisable:
			// Keep routes mounted; the gate handles the disabled state.
		}
	})
}
