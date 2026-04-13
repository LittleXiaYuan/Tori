package ledger

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"github.com/LittleXiaYuan/ledger"

	"yunque-agent/internal/agentcore/memory"
)

// LedgerOrchPersister replaces the JSON-file OrchestratorPersister.
// It stores the knowledge graph (entities+relations) and editable memory (blocks)
// as Ledger Memory entries, using MemorySummary kind with special keys.
type LedgerOrchPersister struct {
	mu       sync.Mutex
	ldg      *ledger.Ledger
	graph    *memory.Graph
	editable *memory.EditableMemory
}

// NewLedgerOrchPersister creates a Ledger-backed orchestrator persister.
//
// If a legacy "data/graph.json" or "data/editable.json" exists, it will be
// migrated to Ledger on first Load.
func NewLedgerOrchPersister(ldg *ledger.Ledger, g *memory.Graph, em *memory.EditableMemory) *LedgerOrchPersister {
	return &LedgerOrchPersister{ldg: ldg, graph: g, editable: em}
}

// Keys used to store graph / editable in Ledger Memory.
const (
	orchGraphKey    = "__system:graph"
	orchEditableKey = "__system:editable"
	orchTenantID    = "__system__"
)

// graphSnapshot is the serializable form of the knowledge graph.
type graphSnapshot struct {
	Entities  []memory.Entity   `json:"entities"`
	Relations []memory.Relation `json:"relations"`
}

// editableSnapshot is the serializable form of editable memory.
type editableSnapshot struct {
	Blocks []memory.Block `json:"blocks"`
}

// Load restores Graph + Editable from Ledger Memory.
// If Ledger is empty but legacy JSON files exist, auto-migrates them.
func (p *LedgerOrchPersister) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	ctx := context.Background()

	// --- Graph ---
	if p.graph != nil {
		loaded := false
		entry, err := p.searchByKey(ctx, orchGraphKey)
		if err == nil && entry != nil {
			var snap graphSnapshot
			if err := json.Unmarshal([]byte(entry.Content), &snap); err == nil {
				for _, e := range snap.Entities {
					p.graph.PutEntity(e)
				}
				for _, r := range snap.Relations {
					p.graph.PutRelation(r)
				}
				loaded = true
				slog.Info("orchestrator: graph loaded from Ledger",
					"entities", len(snap.Entities), "relations", len(snap.Relations))
			}
		}
		if !loaded {
			p.migrateGraphLegacy(ctx)
		}
	}

	// --- Editable ---
	if p.editable != nil {
		loaded := false
		entry, err := p.searchByKey(ctx, orchEditableKey)
		if err == nil && entry != nil {
			var snap editableSnapshot
			if err := json.Unmarshal([]byte(entry.Content), &snap); err == nil {
				for _, b := range snap.Blocks {
					p.editable.AddBlock(b.Label, b.Content, b.MaxChars)
				}
				loaded = true
				slog.Info("orchestrator: editable loaded from Ledger",
					"blocks", len(snap.Blocks))
			}
		}
		if !loaded {
			p.migrateEditableLegacy(ctx)
		}
	}

	return nil
}

// Save persists Graph + Editable to Ledger Memory.
func (p *LedgerOrchPersister) Save() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	ctx := context.Background()

	// --- Graph ---
	if p.graph != nil {
		snap := p.exportGraph()
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if err := p.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
			ID:         orchGraphKey,
			TenantID:   orchTenantID,
			Kind:       ledger.MemorySummary,
			Key:        orchGraphKey,
			Content:    string(data),
			Source:     "system",
			Confidence: 1.0,
		}); err != nil {
			slog.Error("orchestrator: graph save to Ledger failed", "err", err)
			return err
		}
	}

	// --- Editable ---
	if p.editable != nil {
		blocks := p.editable.AllBlocks()
		snap := editableSnapshot{}
		for _, b := range blocks {
			snap.Blocks = append(snap.Blocks, *b)
		}
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if err := p.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
			ID:         orchEditableKey,
			TenantID:   orchTenantID,
			Kind:       ledger.MemorySummary,
			Key:        orchEditableKey,
			Content:    string(data),
			Source:     "system",
			Confidence: 1.0,
		}); err != nil {
			slog.Error("orchestrator: editable save to Ledger failed", "err", err)
			return err
		}
	}

	return nil
}

// --- Helpers ---

func (p *LedgerOrchPersister) searchByKey(ctx context.Context, key string) (*ledger.MemoryEntry, error) {
	results, err := p.ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: orchTenantID,
		Query:    key,
		Limit:    1,
	})
	if err != nil || len(results) == 0 {
		return nil, err
	}
	// Verify exact key match (Search does LIKE matching)
	for _, r := range results {
		if r.Key == key {
			return r, nil
		}
	}
	return nil, nil
}

func (p *LedgerOrchPersister) exportGraph() graphSnapshot {
	entities, relations := p.graph.ExportAll()
	return graphSnapshot{Entities: entities, Relations: relations}
}

// --- Legacy migration ---

func (p *LedgerOrchPersister) migrateGraphLegacy(ctx context.Context) {
	data, err := os.ReadFile("data/graph.json")
	if err != nil {
		return
	}
	var snap graphSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		slog.Warn("orchestrator: legacy graph.json parse failed", "err", err)
		return
	}
	for _, e := range snap.Entities {
		p.graph.PutEntity(e)
	}
	for _, r := range snap.Relations {
		p.graph.PutRelation(r)
	}

	// Save to Ledger immediately
	jsonData, _ := json.Marshal(snap)
	p.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		ID: orchGraphKey, TenantID: orchTenantID,
		Kind: ledger.MemorySummary, Key: orchGraphKey,
		Content: string(jsonData), Source: "migration", Confidence: 1.0,
	})

	_ = os.Rename("data/graph.json", "data/graph.json.migrated")
	slog.Info("orchestrator: migrated graph.json to Ledger",
		"entities", len(snap.Entities), "relations", len(snap.Relations))
}

func (p *LedgerOrchPersister) migrateEditableLegacy(ctx context.Context) {
	data, err := os.ReadFile("data/editable.json")
	if err != nil {
		return
	}
	var snap editableSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		slog.Warn("orchestrator: legacy editable.json parse failed", "err", err)
		return
	}
	for _, b := range snap.Blocks {
		p.editable.AddBlock(b.Label, b.Content, b.MaxChars)
	}

	jsonData, _ := json.Marshal(snap)
	p.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		ID: orchEditableKey, TenantID: orchTenantID,
		Kind: ledger.MemorySummary, Key: orchEditableKey,
		Content: string(jsonData), Source: "migration", Confidence: 1.0,
	})

	_ = os.Rename("data/editable.json", "data/editable.json.migrated")
	slog.Info("orchestrator: migrated editable.json to Ledger",
		"blocks", len(snap.Blocks))
}
