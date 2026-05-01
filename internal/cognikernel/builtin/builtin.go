package builtin

import (
	"embed"
	"encoding/json"
	"io/fs"

	"yunque-agent/pkg/cogni"
)

//go:embed *.json
var content embed.FS

// LoadAll returns all built-in Cogni declarations embedded at compile time.
func LoadAll() ([]*cogni.Declaration, error) {
	entries, err := fs.ReadDir(content, ".")
	if err != nil {
		return nil, err
	}
	var decls []*cogni.Declaration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(content, e.Name())
		if err != nil {
			return nil, err
		}
		var d cogni.Declaration
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		decls = append(decls, &d)
	}
	return decls, nil
}
