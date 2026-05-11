package cognifile

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// LocalRegistry manages installed Cognifiles on the local filesystem.
// It persists state to a JSON index file and provides lookup, install,
// and uninstall operations.
//
// Thread-safe for concurrent use.
type LocalRegistry struct {
	mu      sync.RWMutex
	dir     string // base directory (e.g. data/cognifiles/)
	entries map[string]*InstalledCognifile
}

// NewLocalRegistry creates a registry backed by the given directory.
// The directory and its index file are created on first write.
func NewLocalRegistry(dir string) *LocalRegistry {
	r := &LocalRegistry{
		dir:     dir,
		entries: make(map[string]*InstalledCognifile),
	}
	r.loadIndex()
	return r
}

// Dir returns the registry's base directory.
func (r *LocalRegistry) Dir() string { return r.dir }

// Install adds a Cognifile to the local registry. If one with the same name
// already exists, it is replaced (upgrade semantics).
func (r *LocalRegistry) Install(cf *Cognifile, source string) error {
	if err := cf.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cfDir := filepath.Join(r.dir, cf.Name)
	if err := os.MkdirAll(cfDir, 0o755); err != nil {
		return fmt.Errorf("cognifile: create dir %q: %w", cfDir, err)
	}

	cfPath := filepath.Join(cfDir, "Cognifile.yaml")
	if err := SaveFile(cf, cfPath); err != nil {
		return fmt.Errorf("cognifile: save %q: %w", cfPath, err)
	}

	r.entries[cf.Name] = &InstalledCognifile{
		Cognifile:   cf,
		InstalledAt: time.Now().UTC(),
		Source:      source,
		FilePath:    cfPath,
	}

	return r.saveIndex()
}

// Uninstall removes a Cognifile from the local registry and deletes its directory.
func (r *LocalRegistry) Uninstall(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[name]
	if !ok {
		return fmt.Errorf("cognifile: %q not installed", name)
	}

	cfDir := filepath.Dir(entry.FilePath)
	if err := os.RemoveAll(cfDir); err != nil {
		slog.Warn("cognifile: cleanup failed", "dir", cfDir, "err", err)
	}

	delete(r.entries, name)
	return r.saveIndex()
}

// Get returns an installed Cognifile by name.
func (r *LocalRegistry) Get(name string) (*InstalledCognifile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[name]
	return e, ok
}

// List returns all installed Cognifiles sorted by name.
func (r *LocalRegistry) List() []*InstalledCognifile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*InstalledCognifile, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Cognifile.Name < out[j].Cognifile.Name
	})
	return out
}

// Reload re-scans the registry directory to pick up manually added Cognifiles.
func (r *LocalRegistry) Reload() (added int, errs []ScanError) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return 0, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, exists := r.entries[name]; exists {
			continue
		}

		cfPath := filepath.Join(r.dir, name, "Cognifile.yaml")
		cf, err := LoadFile(cfPath)
		if err != nil {
			cfPath = filepath.Join(r.dir, name, "Cognifile.yml")
			cf, err = LoadFile(cfPath)
		}
		if err != nil {
			cfPath = filepath.Join(r.dir, name, "Cognifile.json")
			cf, err = LoadFile(cfPath)
		}
		if err != nil {
			errs = append(errs, ScanError{Path: filepath.Join(r.dir, name), Err: err})
			continue
		}

		r.entries[cf.Name] = &InstalledCognifile{
			Cognifile:   cf,
			InstalledAt: time.Now().UTC(),
			Source:      "local",
			FilePath:    cfPath,
		}
		added++
	}

	if added > 0 {
		_ = r.saveIndex()
	}
	return added, errs
}

// indexFile is the persisted index of installed Cognifiles.
type indexFile struct {
	Schema  string       `json:"schema"`
	Updated time.Time    `json:"updated"`
	Entries []indexEntry `json:"entries"`
}

type indexEntry struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Source      string    `json:"source"`
	FilePath    string    `json:"file_path"`
	InstalledAt time.Time `json:"installed_at"`
}

func (r *LocalRegistry) indexPath() string {
	return filepath.Join(r.dir, "index.json")
}

func (r *LocalRegistry) loadIndex() {
	data, err := os.ReadFile(r.indexPath())
	if err != nil {
		return
	}
	var idx indexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		slog.Warn("cognifile: corrupt index", "err", err)
		return
	}

	for _, e := range idx.Entries {
		cf, err := LoadFile(e.FilePath)
		if err != nil {
			slog.Warn("cognifile: index entry stale", "name", e.Name, "err", err)
			continue
		}
		r.entries[e.Name] = &InstalledCognifile{
			Cognifile:   cf,
			InstalledAt: e.InstalledAt,
			Source:      e.Source,
			FilePath:    e.FilePath,
		}
	}
}

// saveIndex persists the current in-memory state. Caller must hold r.mu.
func (r *LocalRegistry) saveIndex() error {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return err
	}

	idx := indexFile{
		Schema:  SchemaVersion,
		Updated: time.Now().UTC(),
	}
	for _, e := range r.entries {
		idx.Entries = append(idx.Entries, indexEntry{
			Name:        e.Cognifile.Name,
			Version:     e.Cognifile.Version,
			Source:      e.Source,
			FilePath:    e.FilePath,
			InstalledAt: e.InstalledAt,
		})
	}
	sort.Slice(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].Name < idx.Entries[j].Name
	})

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.indexPath(), data, 0o644)
}
