package packruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type officialYqpackCatalog struct {
	Schema  string                       `json:"schema"`
	Count   int                          `json:"count"`
	Entries []officialYqpackCatalogEntry `json:"entries"`
}

type officialYqpackCatalogEntry struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	ManifestPath string   `json:"manifest_path"`
	ArtifactPath string   `json:"artifact_path"`
	SHA256       string   `json:"sha256"`
	SizeBytes    int64    `json:"size_bytes"`
	Capabilities []string `json:"capabilities"`
	Downloadable bool     `json:"downloadable"`
}

func TestOfficialYqpackCatalogArtifacts(t *testing.T) {
	root := findRepoRoot(t)
	catalogPath := filepath.Join(root, "dist", "packs", "official", "catalog.json")
	data, err := os.ReadFile(catalogPath)
	if os.IsNotExist(err) {
		t.Skip("official yqpack catalog not built; run node scripts/build-official-yqpacks.mjs")
	}
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var catalog officialYqpackCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}
	if catalog.Schema != "yunque.official-yqpack-catalog.v1" {
		t.Fatalf("unexpected catalog schema %q", catalog.Schema)
	}

	sourceManifests := officialManifestPaths(t, filepath.Join(root, "packs", "official"))
	if catalog.Count != len(sourceManifests) {
		t.Fatalf("catalog count %d != official manifest count %d", catalog.Count, len(sourceManifests))
	}
	if len(catalog.Entries) != len(sourceManifests) {
		t.Fatalf("catalog entries %d != official manifest count %d", len(catalog.Entries), len(sourceManifests))
	}

	sourceByID := map[string]Manifest{}
	for _, manifestPath := range sourceManifests {
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			t.Fatalf("load source manifest %s: %v", manifestPath, err)
		}
		sourceByID[manifest.ID] = manifest
	}

	seen := map[string]bool{}
	for _, entry := range catalog.Entries {
		if entry.ID == "" {
			t.Fatalf("catalog entry missing id")
		}
		if seen[entry.ID] {
			t.Fatalf("duplicate catalog entry %s", entry.ID)
		}
		seen[entry.ID] = true
		source, ok := sourceByID[entry.ID]
		if !ok {
			t.Fatalf("catalog entry %s has no source manifest", entry.ID)
		}
		if entry.Version != source.Version {
			t.Fatalf("%s version mismatch: catalog %s source %s", entry.ID, entry.Version, source.Version)
		}
		if !entry.Downloadable {
			t.Fatalf("%s should be downloadable", entry.ID)
		}

		artifactPath := filepath.Join(root, filepath.FromSlash(entry.ArtifactPath))
		manifest, digest, size, err := InspectYqpackManifestFile(artifactPath)
		if err != nil {
			t.Fatalf("inspect %s: %v", artifactPath, err)
		}
		if manifest.ID != source.ID || manifest.Version != source.Version {
			t.Fatalf("%s artifact manifest mismatch: got %s@%s want %s@%s", entry.ID, manifest.ID, manifest.Version, source.ID, source.Version)
		}
		if digest != entry.SHA256 {
			t.Fatalf("%s sha mismatch: catalog %s inspect %s", entry.ID, entry.SHA256, digest)
		}
		if size != entry.SizeBytes {
			t.Fatalf("%s size mismatch: catalog %d inspect %d", entry.ID, entry.SizeBytes, size)
		}
		raw, err := os.ReadFile(artifactPath)
		if err != nil {
			t.Fatalf("read artifact %s: %v", artifactPath, err)
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != entry.SHA256 {
			t.Fatalf("%s raw sha mismatch: catalog %s raw %s", entry.ID, entry.SHA256, got)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatalf("repo root with go.mod not found from %s", dir)
		}
		dir = next
	}
}

func officialManifestPaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == ManifestFileName {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk official manifests: %v", err)
	}
	sort.Strings(paths)
	return paths
}
