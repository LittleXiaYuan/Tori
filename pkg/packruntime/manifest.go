package packruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const ManifestFileName = "pack.json"

type Manifest struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Description  string               `json:"description,omitempty"`
	RequiresCore string               `json:"requiresCore,omitempty"`
	Optional     bool                 `json:"optional"`
	DefaultState string               `json:"defaultState,omitempty"`
	Status       string               `json:"status,omitempty"`
	ABI          string               `json:"abi,omitempty"`
	PublishedAt  string               `json:"publishedAt,omitempty"`
	Publisher    PublisherManifest    `json:"publisher,omitempty"`
	Dependencies []ManifestDependency `json:"dependencies,omitempty"`
	Backend      BackendManifest      `json:"backend"`
	Frontend     FrontendManifest     `json:"frontend"`
	SDK          SDKManifest          `json:"sdk,omitempty"`
	Distribution DistributionManifest `json:"distribution,omitempty"`
	Signing      *SigningManifest     `json:"signing,omitempty"`
	Update       UpdateManifest       `json:"update,omitempty"`
	Metadata     map[string]string    `json:"metadata,omitempty"`
}

// PublisherManifest identifies the entity that signs the pack manifest.
// Public-key bytes themselves come from the trust root, never from the manifest.
type PublisherManifest struct {
	ID          string `json:"id,omitempty"`
	PublicKeyID string `json:"publicKeyId,omitempty"`
}

// ManifestDependency declares an explicit pack-id + SemVer range another pack
// requires. Resolution is non-transitive: enforced at install time, not auto-resolved.
type ManifestDependency struct {
	ID       string `json:"id"`
	Requires string `json:"requires,omitempty"`
}

// SigningManifest carries the detached ed25519 signature material for a pack.
// `Signature` is base64(ed25519(canonical(manifest_without_signing))).
// `ManifestSHA256` is sha256 hex of the same canonical bytes.
type SigningManifest struct {
	Algorithm      string `json:"algorithm,omitempty"`
	ManifestSHA256 string `json:"manifestSha256,omitempty"`
	Signature      string `json:"signature,omitempty"`
}

type BackendManifest struct {
	Capabilities []string           `json:"capabilities,omitempty"`
	Routes       []string           `json:"routes,omitempty"`
	RouteSpecs   []BackendRouteSpec `json:"routeSpecs,omitempty"`
	Permissions  []string           `json:"permissions,omitempty"`
	Runtime      *BackendRuntime    `json:"runtime,omitempty"`
	// ToolSpecs declares agent tools (skills) a wasm pack contributes. When the
	// pack is enabled the host builds a sandboxed WasmSkill per spec and registers
	// it into the skill registry, so a downloaded pack gives the agent callable
	// capability (Tier 0 microkernel "tool line"); disabling removes them.
	ToolSpecs []BackendToolSpec `json:"toolSpecs,omitempty"`
}

// BackendToolSpec is manifest-owned metadata for one wasm-backed agent tool.
type BackendToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Entrypoint  string         `json:"entrypoint,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// BackendRuntime declares how a pack's backend routes are executed. A nil
// Runtime (the default) means the routes are served in-process by Go code
// compiled into the host binary (first-party packs). Type "wasm" means the
// routes are served by sandboxed WebAssembly shipped inside the .yqpack — the
// untrusted third-party delivery path.
type BackendRuntime struct {
	Type   string `json:"type"`             // "" = in-process (first-party); "wasm" = sandboxed module
	Module string `json:"module,omitempty"` // pack-relative path to the .wasm, e.g. "module.wasm"
	SHA256 string `json:"sha256,omitempty"` // hex sha256 of the module bytes, enforced before execution
	// ABIVersion is the host↔module ABI the pack was built against (see
	// docs/spec/pack-wasm-abi.md). 0 (unset) means the original v1 ABI. The host
	// refuses to mount a module whose ABI it cannot support, so a downloaded pack
	// built for a newer/older ABI fails closed instead of misbehaving.
	ABIVersion int `json:"abiVersion,omitempty"`
}

// RuntimeTypeWasm is the BackendRuntime.Type value selecting the WASM
// delivery path.
const RuntimeTypeWasm = "wasm"

// WASM host ABI version range supported by this host build. New host functions
// are added only by bumping CurrentABIVersion (never by changing an existing
// signature), so older modules keep working; MinABIVersion drops support for a
// retired ABI after a deprecation window.
const (
	// CurrentABIVersion: v2 adds the llm_chat host function (additive, gated by
	// the llm:call permission). v1 modules keep working since host functions are
	// only ever added, never changed.
	CurrentABIVersion = 2
	MinABIVersion     = 1
)

// IsWasm reports whether this backend is delivered as a sandboxed WASM module.
func (b BackendManifest) IsWasm() bool {
	return b.Runtime != nil && b.Runtime.Type == RuntimeTypeWasm
}

// ABICompatible reports whether this host build can run the module's declared
// ABI. An unset (0) ABIVersion is treated as the original v1 ABI.
func (rt *BackendRuntime) ABICompatible() bool {
	if rt == nil {
		return true
	}
	v := rt.ABIVersion
	if v == 0 {
		v = CurrentABIVersion
	}
	return v >= MinABIVersion && v <= CurrentABIVersion
}

// BackendRouteSpec is manifest-owned backend route metadata. Routes keeps the
// legacy path-only gate for compatibility, while RouteSpecs allows packs and
// external tooling to audit method/path pairs without loading backend code.
type BackendRouteSpec struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	// Entrypoint names the WASM export invoked for this route (wasm runtime
	// only). Empty defaults to "_start" (the WASI command convention).
	Entrypoint string `json:"entrypoint,omitempty"`
}

type FrontendManifest struct {
	Menus  []FrontendMenu  `json:"menus,omitempty"`
	Routes []FrontendRoute `json:"routes,omitempty"`
	Assets FrontendAssets  `json:"assets,omitempty"`
}

type FrontendMenu struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Path  string `json:"path"`
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order,omitempty"`
}

type FrontendRoute struct {
	Path      string `json:"path"`
	Component string `json:"component"`
	Title     string `json:"title,omitempty"`
}

// FrontendAssets describes how a pack's frontend is loaded by the web shell.
//
//   - "inline":        Routes[].Component resolves to a component pre-built into
//     the main app (first-party packs only).
//   - "iframe-bundle": the .yqpack carries a self-contained static bundle under
//     frontend/ (Entry, default "index.html"); the shell loads it in a sandboxed
//     iframe and talks to it over the postMessage bridge. See
//     docs/spec/pack-frontend-dlc.md.
type FrontendAssets struct {
	Type  string `json:"type,omitempty"`
	Entry string `json:"entry,omitempty"`
}

const (
	// FrontendAssetsTypeBuiltin / Inline both mean "component pre-built into the
	// main app" (first-party). "builtin" is the historical value; "inline" is an
	// accepted synonym. "remote" is a reserved (pre-existing) value. "iframe-bundle"
	// loads a self-contained bundle in a sandboxed iframe (docs/spec/pack-frontend-dlc.md).
	FrontendAssetsTypeBuiltin      = "builtin"
	FrontendAssetsTypeInline       = "inline"
	FrontendAssetsTypeRemote       = "remote"
	FrontendAssetsTypeIframeBundle = "iframe-bundle"
)

type SDKManifest struct {
	TypeScript string `json:"typescript,omitempty"`
	Go         string `json:"go,omitempty"`
	Python     string `json:"python,omitempty"`
}

type DistributionManifest struct {
	ManifestURL string             `json:"manifestUrl,omitempty"`
	PackageURL  string             `json:"packageUrl,omitempty"`
	FrontendURL string             `json:"frontendUrl,omitempty"`
	SHA256      string             `json:"sha256,omitempty"`
	SizeBytes   int64              `json:"sizeBytes,omitempty"`
	Platforms   []string           `json:"platforms,omitempty"`
	Mirrors     []DistributionMirror `json:"mirrors,omitempty"`
}

// DistributionMirror is one equivalent download location for the same .yqpack
// artifact. All mirrors must serve the byte-identical artifact; kind only
// affects UX-level ordering and labelling.
type DistributionMirror struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type UpdateManifest struct {
	Channel  string `json:"channel,omitempty"`
	Rollback bool   `json:"rollback"`
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("pack manifest id is required")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("pack manifest name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("pack manifest version is required")
	}
	if m.DefaultState != "" && m.DefaultState != "enabled" && m.DefaultState != "disabled" {
		return fmt.Errorf("pack manifest defaultState must be enabled or disabled")
	}
	for i, route := range m.Backend.Routes {
		if !strings.HasPrefix(route, "/") {
			return fmt.Errorf("backend.routes[%d] must start with /", i)
		}
	}
	for i, route := range m.Backend.RouteSpecs {
		if strings.TrimSpace(route.Method) == "" {
			return fmt.Errorf("backend.routeSpecs[%d].method is required", i)
		}
		if !strings.HasPrefix(strings.TrimSpace(route.Path), "/") {
			return fmt.Errorf("backend.routeSpecs[%d].path must start with /", i)
		}
	}
	if rt := m.Backend.Runtime; rt != nil {
		if rt.Type != "" && rt.Type != RuntimeTypeWasm {
			return fmt.Errorf("backend.runtime.type must be empty or %q", RuntimeTypeWasm)
		}
		if rt.Type == RuntimeTypeWasm {
			if strings.TrimSpace(rt.Module) == "" {
				return fmt.Errorf("backend.runtime.module is required for wasm runtime")
			}
			if len(m.Backend.RouteSpecs) == 0 {
				return fmt.Errorf("backend.runtime wasm requires at least one backend.routeSpecs entry")
			}
		}
	}
	for i, menu := range m.Frontend.Menus {
		if strings.TrimSpace(menu.Key) == "" || strings.TrimSpace(menu.Label) == "" || strings.TrimSpace(menu.Path) == "" {
			return fmt.Errorf("frontend.menus[%d] requires key, label and path", i)
		}
	}
	for i, route := range m.Frontend.Routes {
		if strings.TrimSpace(route.Path) == "" || strings.TrimSpace(route.Component) == "" {
			return fmt.Errorf("frontend.routes[%d] requires path and component", i)
		}
	}
	switch strings.TrimSpace(m.Frontend.Assets.Type) {
	case "", FrontendAssetsTypeBuiltin, FrontendAssetsTypeInline, FrontendAssetsTypeRemote, FrontendAssetsTypeIframeBundle:
		// ok
	default:
		return fmt.Errorf("frontend.assets.type %q is invalid", m.Frontend.Assets.Type)
	}
	if m.Distribution.PackageURL != "" && strings.TrimSpace(m.Distribution.SHA256) == "" {
		return fmt.Errorf("distribution.sha256 is required when distribution.packageUrl is set")
	}
	if m.Distribution.SizeBytes < 0 {
		return fmt.Errorf("distribution.sizeBytes must be greater than or equal to 0")
	}
	for i, mirror := range m.Distribution.Mirrors {
		if strings.TrimSpace(mirror.Kind) == "" {
			return fmt.Errorf("distribution.mirrors[%d].kind is required", i)
		}
		if strings.TrimSpace(mirror.URL) == "" {
			return fmt.Errorf("distribution.mirrors[%d].url is required", i)
		}
	}
	for i, plat := range m.Distribution.Platforms {
		if !strings.Contains(plat, "/") {
			return fmt.Errorf("distribution.platforms[%d] must be in <goos>/<goarch> form", i)
		}
	}
	for i, dep := range m.Dependencies {
		if strings.TrimSpace(dep.ID) == "" {
			return fmt.Errorf("dependencies[%d].id is required", i)
		}
	}
	if m.Signing != nil {
		if m.Signing.Algorithm != "" && m.Signing.Algorithm != "ed25519" {
			return fmt.Errorf("signing.algorithm must be ed25519 (got %q)", m.Signing.Algorithm)
		}
		if m.Signing.Signature != "" && strings.TrimSpace(m.Signing.ManifestSHA256) == "" {
			return fmt.Errorf("signing.manifestSha256 is required when signing.signature is set")
		}
	}
	return nil
}

// AllowsRoute reports whether a mounted backend route is declared by the
// manifest. Path-only backend.routes remain supported for older packs. When a
// routeSpecs entry exists for the same path, the method must also match.
func (b BackendManifest) AllowsRoute(method string, path string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	matchedPathOnly := false
	for _, route := range b.Routes {
		if strings.TrimSpace(route) == path {
			matchedPathOnly = true
			break
		}
	}
	matchedSpecPath := false
	for _, route := range b.RouteSpecs {
		if strings.TrimSpace(route.Path) != path {
			continue
		}
		matchedSpecPath = true
		if strings.ToUpper(strings.TrimSpace(route.Method)) == method {
			return true
		}
	}
	return matchedPathOnly && !matchedSpecPath
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read pack manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse pack manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func SaveManifest(path string, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pack manifest: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
