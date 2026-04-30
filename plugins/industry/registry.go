// Package industry provides a registration mechanism for industry-specific
// plugin extensions. Instead of maintaining full code forks (legal-ai,
// wuyue-collect), industry capabilities are encapsulated as plugins that
// register themselves based on BUILD_FLAVOR.
//
// Migration path from code forks:
//
//  1. Move legal-ai/backend/legal/engine.go logic into plugins/industry/legal/
//     as a plugin implementing pkg/plugin.Plugin.
//  2. Move wuyue-collect's embed_knowledge.go into plugins/industry/collect/
//     as a startup hook.
//  3. Set BUILD_FLAVOR=legal (or collect) instead of maintaining a separate repo.
//  4. Remove the forked yunque-agent directories from legal-ai and wuyue-collect.
//
// To add a new industry flavor:
//
//  1. Create plugins/industry/<flavor>/plugin.go
//  2. Implement the Register(reg) function
//  3. Add a //go:build <flavor> registration stub (or use runtime BUILD_FLAVOR check)
//  4. Build with: go build -tags <flavor> ./cmd/agent
//     Or set BUILD_FLAVOR=<flavor> and use runtime registration.
package industry

import "yunque-agent/pkg/skills"

// Registrar is a function that registers industry-specific skills.
type Registrar func(reg *skills.Registry)

var registrars []Registrar

// Register adds an industry-specific registrar to be called during init.
func Register(r Registrar) {
	registrars = append(registrars, r)
}

// ApplyAll calls all registered industry registrars.
func ApplyAll(reg *skills.Registry) {
	for _, r := range registrars {
		r(reg)
	}
}

// FlavorRegistrars holds runtime-selected registrars keyed by flavor name.
var FlavorRegistrars = map[string]Registrar{}

// ApplyFlavor registers skills for the given flavor (runtime selection via BUILD_FLAVOR).
func ApplyFlavor(flavor string, reg *skills.Registry) {
	if r, ok := FlavorRegistrars[flavor]; ok {
		r(reg)
	}
}
