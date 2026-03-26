package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FilePluginMemory implements PluginMemory using a JSON file per plugin.
// Each plugin gets its own namespace at {dataDir}/{pluginName}.json,
// ensuring complete isolation between plugins.
type FilePluginMemory struct {
	mu   sync.RWMutex
	data map[string]string
	path string
}

// NewFilePluginMemory creates a file-backed plugin memory.
// dataDir is the base directory (e.g. "data/plugin_memory"),
// pluginName is used as the filename.
func NewFilePluginMemory(dataDir, pluginName string) *FilePluginMemory {
	os.MkdirAll(dataDir, 0o755)
	m := &FilePluginMemory{
		data: make(map[string]string),
		path: filepath.Join(dataDir, pluginName+".json"),
	}
	m.load()
	return m
}

func (m *FilePluginMemory) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok
}

func (m *FilePluginMemory) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return m.save()
}

func (m *FilePluginMemory) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return m.save()
}

func (m *FilePluginMemory) List(prefix string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string)
	for k, v := range m.data {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out
}

// Search performs substring matching across all values.
// For simple use cases this is sufficient; plugins with heavy search
// needs should implement their own vector store on top of PluginMemory.
func (m *FilePluginMemory) Search(query string, limit int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q := strings.ToLower(query)
	var results []string
	for _, v := range m.data {
		if strings.Contains(strings.ToLower(v), q) {
			results = append(results, v)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}

// Count returns the number of entries.
func (m *FilePluginMemory) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

func (m *FilePluginMemory) load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.data)
}

func (m *FilePluginMemory) save() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o644)
}

// Ensure FilePluginMemory implements PluginMemory.
var _ PluginMemory = (*FilePluginMemory)(nil)

// PluginMemoryManager creates and caches PluginMemory instances for plugins.
type PluginMemoryManager struct {
	mu      sync.Mutex
	dataDir string
	stores  map[string]*FilePluginMemory
}

// NewPluginMemoryManager creates a manager that provisions memory for plugins.
func NewPluginMemoryManager(dataDir string) *PluginMemoryManager {
	return &PluginMemoryManager{
		dataDir: dataDir,
		stores:  make(map[string]*FilePluginMemory),
	}
}

// ForPlugin returns (or creates) the PluginMemory for a specific plugin.
func (mgr *PluginMemoryManager) ForPlugin(pluginName string) *FilePluginMemory {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if store, ok := mgr.stores[pluginName]; ok {
		return store
	}
	store := NewFilePluginMemory(mgr.dataDir, pluginName)
	mgr.stores[pluginName] = store
	return store
}
