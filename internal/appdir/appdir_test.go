package appdir

import (
	"path/filepath"
	"testing"
)

func TestIsGoRunBinary(t *testing.T) {
	cases := []struct {
		exe  string
		want bool
	}{
		// go run cache locations (Windows / Unix / custom GOTMPDIR)
		{`C:\Users\me\AppData\Local\Temp\go-build3011004099\b001\exe\agent.exe`, true},
		{"/tmp/go-build123456789/b001/exe/agent", true},
		{"/home/me/gotmp/go-build42/b001/exe/agent", true},
		// Real installs must keep anchoring data next to the executable.
		{`C:\Program Files\Yunque\yunque-agent.exe`, false},
		{"/usr/local/bin/yunque-agent", false},
		{`C:\Code\yunque-agent\dist\yunque-agent.exe`, false},
		// "go-build" as a substring of a larger segment is not a build cache.
		{`C:\projects\my-go-builder\bin\agent.exe`, false},
	}
	for _, tc := range cases {
		if got := isGoRunBinary(tc.exe); got != tc.want {
			t.Errorf("isGoRunBinary(%q) = %v, want %v", tc.exe, got, tc.want)
		}
	}
}

func TestResolvePrefersEnvOverride(t *testing.T) {
	t.Setenv("YUNQUE_DATA_DIR", filepath.Join("custom", "root"))
	if got := resolve(); got != filepath.Join("custom", "root") {
		t.Fatalf("resolve() = %q, want env override", got)
	}
}
