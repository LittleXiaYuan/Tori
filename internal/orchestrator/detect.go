package orchestrator

import (
	"os/exec"
	"runtime"
)

type IDEInfo struct {
	Name      string `json:"name"`
	Binary    string `json:"binary"`
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
}

func DetectIDEs() []IDEInfo {
	candidates := []struct {
		name   string
		binary string
	}{
		{"Cursor", cursorDetectBinary()},
		{"Claude Code", claudeDetectBinary()},
		{"Windsurf", windsurfDetectBinary()},
		{"Trae", traeDetectBinary()},
		{"VS Code", vscodeDetectBinary()},
	}

	var result []IDEInfo
	for _, c := range candidates {
		info := IDEInfo{Name: c.name, Binary: c.binary}
		path, err := exec.LookPath(c.binary)
		if err == nil {
			info.Available = true
			info.Path = path
		}
		result = append(result, info)
	}
	return result
}

func cursorDetectBinary() string {
	if runtime.GOOS == "windows" {
		return "cursor.exe"
	}
	return "cursor"
}

func claudeDetectBinary() string {
	if runtime.GOOS == "windows" {
		return "claude.exe"
	}
	return "claude"
}

func windsurfDetectBinary() string {
	if runtime.GOOS == "windows" {
		return "windsurf.exe"
	}
	return "windsurf"
}

func traeDetectBinary() string {
	if runtime.GOOS == "windows" {
		return "trae.exe"
	}
	return "trae"
}

func vscodeDetectBinary() string {
	if runtime.GOOS == "windows" {
		return "code.exe"
	}
	return "code"
}
