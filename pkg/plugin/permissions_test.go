package plugin

import (
	"testing"

	"yunque-agent/pkg/manifest"
)

func TestDefaultPolicyAllowsAll(t *testing.T) {
	p := DefaultPolicy()
	m := &manifest.Manifest{
		Name:    "test",
		Version: "0.1.0",
		Permissions: []manifest.Permission{
			{Name: manifest.PermNetwork, Required: true},
			{Name: manifest.PermSandbox, Required: true},
		},
	}
	if err := p.CheckManifest(m); err != nil {
		t.Fatalf("default policy should allow all: %v", err)
	}
	if !p.CheckPermission(manifest.PermNetwork) {
		t.Fatal("default policy should allow any permission")
	}
}

func TestRestrictedPolicyBlocksMissing(t *testing.T) {
	p := RestrictedPolicy(manifest.PermNetwork)
	m := &manifest.Manifest{
		Name:    "test",
		Version: "0.1.0",
		Permissions: []manifest.Permission{
			{Name: manifest.PermNetwork, Required: true},
			{Name: manifest.PermSandbox, Required: true}, // not granted
		},
	}
	if err := p.CheckManifest(m); err == nil {
		t.Fatal("restricted policy should reject missing required permission")
	}
}

func TestRestrictedPolicyAllowsOptional(t *testing.T) {
	p := RestrictedPolicy(manifest.PermNetwork)
	m := &manifest.Manifest{
		Name:    "test",
		Version: "0.1.0",
		Permissions: []manifest.Permission{
			{Name: manifest.PermNetwork, Required: true},
			{Name: manifest.PermSandbox, Required: false}, // optional, ok
		},
	}
	if err := p.CheckManifest(m); err != nil {
		t.Fatalf("should allow optional missing permission: %v", err)
	}
}

func TestCheckPermissionRuntime(t *testing.T) {
	p := RestrictedPolicy(manifest.PermNetwork, manifest.PermLLM)
	if !p.CheckPermission(manifest.PermNetwork) {
		t.Fatal("should allow granted permission")
	}
	if p.CheckPermission(manifest.PermSandbox) {
		t.Fatal("should deny non-granted permission")
	}
}
