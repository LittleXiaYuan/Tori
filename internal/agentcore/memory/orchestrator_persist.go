package memory

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// OrchestratorPersister handles saving/loading Graph + EditableMemory state.
type OrchestratorPersister struct {
	mu      sync.Mutex
	dataDir string
	graph   *Graph
	editable *EditableMemory
}

// NewOrchestratorPersister creates a persister for orchestrator components.
func NewOrchestratorPersister(dataDir string, g *Graph, em *EditableMemory) *OrchestratorPersister {
	return &OrchestratorPersister{dataDir: dataDir, graph: g, editable: em}
}

// graphData is the serializable form of the knowledge graph.
type graphData struct {
	Entities  []Entity   `json:"entities"`
	Relations []Relation `json:"relations"`
}

// editableData is the serializable form of editable memory.
type editableData struct {
	Blocks []Block `json:"blocks"`
}

// Load restores state from disk.
func (p *OrchestratorPersister) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.graph != nil {
		if err := p.loadGraph(); err != nil {
			slog.Warn("orchestrator: graph load failed", "err", err)
		}
	}
	if p.editable != nil {
		if err := p.loadEditable(); err != nil {
			slog.Warn("orchestrator: editable load failed", "err", err)
		}
	}
	return nil
}

// Save persists state to disk.
func (p *OrchestratorPersister) Save() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	os.MkdirAll(p.dataDir, 0o755)

	if p.graph != nil {
		if err := p.saveGraph(); err != nil {
			return err
		}
	}
	if p.editable != nil {
		if err := p.saveEditable(); err != nil {
			return err
		}
	}
	return nil
}

func (p *OrchestratorPersister) loadGraph() error {
	path := filepath.Join(p.dataDir, "graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var gd graphData
	if err := json.Unmarshal(data, &gd); err != nil {
		return err
	}
	for _, e := range gd.Entities {
		p.graph.PutEntity(e)
	}
	for _, r := range gd.Relations {
		p.graph.PutRelation(r)
	}
	slog.Info("orchestrator: graph loaded", "entities", len(gd.Entities), "relations", len(gd.Relations))
	return nil
}

func (p *OrchestratorPersister) saveGraph() error {
	p.graph.mu.RLock()
	defer p.graph.mu.RUnlock()

	var gd graphData
	for _, e := range p.graph.entities {
		gd.Entities = append(gd.Entities, *e)
	}
	for _, r := range p.graph.relations {
		gd.Relations = append(gd.Relations, *r)
	}

	path := filepath.Join(p.dataDir, "graph.json")
	data, err := json.MarshalIndent(gd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (p *OrchestratorPersister) loadEditable() error {
	path := filepath.Join(p.dataDir, "editable.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var ed editableData
	if err := json.Unmarshal(data, &ed); err != nil {
		return err
	}
	for _, b := range ed.Blocks {
		p.editable.AddBlock(b.Label, b.Content, b.MaxChars)
	}
	slog.Info("orchestrator: editable loaded", "blocks", len(ed.Blocks))
	return nil
}

func (p *OrchestratorPersister) saveEditable() error {
	blocks := p.editable.AllBlocks()
	ed := editableData{}
	for _, b := range blocks {
		ed.Blocks = append(ed.Blocks, *b)
	}

	path := filepath.Join(p.dataDir, "editable.json")
	data, err := json.MarshalIndent(ed, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
