package sbom

import (
	"embed"
	"io/fs"
)

//go:embed sbom.cdx.json
var content embed.FS

// Get returns the embedded CycloneDX SBOM bytes.
// Returns nil if no SBOM was embedded at build time.
func Get() []byte {
	data, err := fs.ReadFile(content, "sbom.cdx.json")
	if err != nil {
		return nil
	}
	return data
}
