package packruntime

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func wasmManifest(id, version string) Manifest {
	m := minimalManifest(id, version)
	m.Backend = BackendManifest{
		Capabilities: []string{"hello.ping"},
		RouteSpecs: []BackendRouteSpec{
			{Method: "GET", Path: "/v1/hello/ping", Entrypoint: "_start"},
		},
		Runtime: &BackendRuntime{
			Type:   RuntimeTypeWasm,
			Module: "module.wasm",
			SHA256: "abc123",
		},
	}
	return m
}

func TestBackendRuntimeWasmValidates(t *testing.T) {
	m := wasmManifest("yunque.pack.hello", "0.1.0")
	if err := m.Validate(); err != nil {
		t.Fatalf("valid wasm manifest rejected: %v", err)
	}
	if !m.Backend.IsWasm() {
		t.Fatal("IsWasm should report true")
	}
}

func TestBackendRuntimeWasmRequiresModule(t *testing.T) {
	m := wasmManifest("yunque.pack.hello", "0.1.0")
	m.Backend.Runtime.Module = ""
	if err := m.Validate(); err == nil {
		t.Fatal("expected error when wasm runtime has no module")
	}
}

func TestBackendRuntimeWasmRequiresRouteSpec(t *testing.T) {
	m := wasmManifest("yunque.pack.hello", "0.1.0")
	m.Backend.RouteSpecs = nil
	if err := m.Validate(); err == nil {
		t.Fatal("expected error when wasm runtime has no routeSpecs")
	}
}

func TestBackendRuntimeRejectsUnknownType(t *testing.T) {
	m := wasmManifest("yunque.pack.hello", "0.1.0")
	m.Backend.Runtime.Type = "subprocess"
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for unknown runtime type")
	}
}

func TestNilRuntimeIsInProcess(t *testing.T) {
	m := minimalManifest("yunque.pack.firstparty", "0.1.0")
	if m.Backend.IsWasm() {
		t.Fatal("nil runtime must not report wasm")
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("first-party manifest rejected: %v", err)
	}
}

// The signature must cover the new runtime fields: tampering with Runtime
// after signing must break verification, proving canonical bytes include them.
func TestSignatureCoversRuntimeFields(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := wasmManifest("yunque.pack.hello", "0.1.0")
	if err := SignManifest(&m, priv, "test-publisher", "test-key-1"); err != nil {
		t.Fatalf("sign: %v", err)
	}
	tr := NewTrustRoot(t.TempDir())
	if err := tr.AddDiskKey("test-publisher", "test-key-1", pub); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifest(m, tr); err != nil {
		t.Fatalf("verify clean: %v", err)
	}

	// Flip the module SHA the signature was supposed to bind.
	tampered := m
	tampered.Backend.Runtime = &BackendRuntime{
		Type:   RuntimeTypeWasm,
		Module: "module.wasm",
		SHA256: "deadbeef",
	}
	if err := VerifyManifest(tampered, tr); err == nil {
		t.Fatal("expected verify failure after tampering with runtime.sha256")
	}
}
