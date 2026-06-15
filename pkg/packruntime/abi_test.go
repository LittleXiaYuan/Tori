package packruntime

import "testing"

// TestBackendRuntimeABICompatible verifies the host fails closed on an ABI it
// cannot run, while treating an unset version as the original v1 ABI.
func TestBackendRuntimeABICompatible(t *testing.T) {
	cases := []struct {
		name string
		rt   *BackendRuntime
		want bool
	}{
		{"nil runtime", nil, true},
		{"unset defaults to current", &BackendRuntime{Type: RuntimeTypeWasm}, true},
		{"current version", &BackendRuntime{Type: RuntimeTypeWasm, ABIVersion: CurrentABIVersion}, true},
		{"too new", &BackendRuntime{Type: RuntimeTypeWasm, ABIVersion: CurrentABIVersion + 1}, false},
		{"below min", &BackendRuntime{Type: RuntimeTypeWasm, ABIVersion: MinABIVersion - 1}, MinABIVersion-1 == 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rt.ABICompatible(); got != tc.want {
				t.Fatalf("ABICompatible() = %v, want %v", got, tc.want)
			}
		})
	}
}
