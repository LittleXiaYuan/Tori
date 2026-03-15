package version

import (
	"fmt"
	"runtime"
	"time"
)

// Set at build time via -ldflags:
//
//	go build -ldflags "-X yunque-agent/internal/version.Version=1.0.0
//	  -X yunque-agent/internal/version.GitCommit=$(git rev-parse --short HEAD)
//	  -X yunque-agent/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	Version   = "0.1.0-dev"
	GitCommit = "unknown"
	BuildDate = ""
)

// Info returns structured version information.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get returns the current version info.
func Get() Info {
	bd := BuildDate
	if bd == "" {
		bd = time.Now().UTC().Format(time.RFC3339)
	}
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: bd,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// String returns a one-line version string.
func String() string {
	return fmt.Sprintf("Yunque Agent %s (%s) built %s", Version, GitCommit, BuildDate)
}
