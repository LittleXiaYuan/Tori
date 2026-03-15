package memory

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Entity represents a node in the knowledge graph.
type Entity struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"` // person, place, concept, event, project, skill, preference
	Properties map[string]string `json:"properties"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Mentions   int               `json:"mentions"` // how many times referenced
}

// Relation represents an edge between entities.
type Relation struct {
	ID        string    `json:"id"`
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	Type      string    `json:"type"` // knows, likes, works_on, located_in, part_of, uses, created, etc.
	Weight    float64   `json:"weight"` // strength of relation (0-1)
	Context   string    `json:"context,omitempty"` // the conversation context where this was established
	CreatedAt time.Time `json:"created_at"`
}

// Graph is an in-memory knowledge graph for entity-relation storage.
// Provides richer semantic queries than KV or vector search.
type Graph struct {
	mu        sync.RWMutex
	entities  map[string]*Entity   // id -> entity
	relations map[string]*Relation // id -> relation
	nameIdx   map[string]string    // lowercase_name -> entity_id (for quick lookup)
	adjOut    map[string][]string  // entity_id -> []relation_id (outgoing)
	adjIn     map[string][]string  // entity_id -> []relation_id (incoming)
}

// NewGraph creates a knowledge graph.
func NewGraph() *Graph {
	return &Graph{
		entities:  make(map[string]*Entity),
		relations: make(map[string]*Relation),
		nameIdx:   make(map[string]string),
		adjOut:    make(map[string][]string),
		adjIn:     make(map[string][]string),
	}
}

// PutEntity adds or updates an entity.
func (g *Graph) PutEntity(e Entity) *Entity {
	g.mu.Lock()
	defer g.mu.Unlock()

	if existing, ok := g.entities[e.ID]; ok {
		existing.Mentions++
		existing.UpdatedAt = time.Now()
		for k, v := range e.Properties {
			existing.Properties[k] = v
		}
		if e.Name != "" {
			delete(g.nameIdx, strings.ToLower(existing.Name))
			existing.Name = e.Name
			g.nameIdx[strings.ToLower(e.Name)] = e.ID
		}
		cp := *existing
		return &cp
	}

	now := time.Now()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
	if e.Mentions == 0 {
		e.Mentions = 1
	}
	if e.Properties == nil {
		e.Properties = make(map[string]string)
	}
	stored := e
	g.entities[e.ID] = &stored
	g.nameIdx[strings.ToLower(e.Name)] = e.ID
	cp := stored
	return &cp
}

// GetEntity returns an entity by ID.
func (g *Graph) GetEntity(id string) (*Entity, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	e, ok := g.entities[id]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

// FindByName looks up an entity by name (case-insensitive).
func (g *Graph) FindByName(name string) (*Entity, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	id, ok := g.nameIdx[strings.ToLower(name)]
	if !ok {
		return nil, false
	}
	e, ok := g.entities[id]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

// SearchEntities finds entities matching a query string in name/type/properties.
func (g *Graph) SearchEntities(query string, limit int) []Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lower := strings.ToLower(query)
	var results []Entity
	for _, e := range g.entities {
		if strings.Contains(strings.ToLower(e.Name), lower) ||
			strings.Contains(strings.ToLower(e.Type), lower) {
			cp := *e
			results = append(results, cp)
			if limit > 0 && len(results) >= limit {
				break
			}
			continue
		}
		for _, v := range e.Properties {
			if strings.Contains(strings.ToLower(v), lower) {
				cp := *e
				results = append(results, cp)
				if limit > 0 && len(results) >= limit {
					return results
				}
				break
			}
		}
	}
	return results
}

// PutRelation adds or strengthens a relation between entities.
func (g *Graph) PutRelation(r Relation) *Relation {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if a similar relation already exists
	for _, rid := range g.adjOut[r.FromID] {
		existing := g.relations[rid]
		if existing.ToID == r.ToID && existing.Type == r.Type {
			// Strengthen existing relation
			existing.Weight = min(1.0, existing.Weight+0.1)
			if r.Context != "" {
				existing.Context = r.Context
			}
			cp := *existing
			return &cp
		}
	}

	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	if r.Weight == 0 {
		r.Weight = 0.5
	}
	stored := r
	g.relations[r.ID] = &stored
	g.adjOut[r.FromID] = append(g.adjOut[r.FromID], r.ID)
	g.adjIn[r.ToID] = append(g.adjIn[r.ToID], r.ID)
	cp := stored
	return &cp
}

// GetRelations returns all relations from an entity.
func (g *Graph) GetRelations(entityID string) []Relation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var results []Relation
	for _, rid := range g.adjOut[entityID] {
		if r, ok := g.relations[rid]; ok {
			cp := *r
			results = append(results, cp)
		}
	}
	for _, rid := range g.adjIn[entityID] {
		if r, ok := g.relations[rid]; ok {
			cp := *r
			results = append(results, cp)
		}
	}
	return results
}

// Neighbors returns all entities directly connected to the given entity.
func (g *Graph) Neighbors(entityID string, maxDepth int) []Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 1
	}

	visited := map[string]bool{entityID: true}
	frontier := []string{entityID}
	var results []Entity

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, eid := range frontier {
			// Outgoing
			for _, rid := range g.adjOut[eid] {
				r := g.relations[rid]
				if !visited[r.ToID] {
					visited[r.ToID] = true
					if e, ok := g.entities[r.ToID]; ok {
						cp := *e
						results = append(results, cp)
						next = append(next, r.ToID)
					}
				}
			}
			// Incoming
			for _, rid := range g.adjIn[eid] {
				r := g.relations[rid]
				if !visited[r.FromID] {
					visited[r.FromID] = true
					if e, ok := g.entities[r.FromID]; ok {
						cp := *e
						results = append(results, cp)
						next = append(next, r.FromID)
					}
				}
			}
		}
		frontier = next
	}
	return results
}

// ContextFor builds a natural language summary of an entity's knowledge context.
// This can be injected into the LLM prompt to provide relational context.
func (g *Graph) ContextFor(entityID string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	e, ok := g.entities[entityID]
	if !ok {
		return ""
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("[%s] %s", e.Type, e.Name))

	for k, v := range e.Properties {
		parts = append(parts, fmt.Sprintf("  %s: %s", k, v))
	}

	for _, rid := range g.adjOut[entityID] {
		r := g.relations[rid]
		if target, ok := g.entities[r.ToID]; ok {
			parts = append(parts, fmt.Sprintf("  → %s %s (%.0f%%)", r.Type, target.Name, r.Weight*100))
		}
	}
	for _, rid := range g.adjIn[entityID] {
		r := g.relations[rid]
		if source, ok := g.entities[r.FromID]; ok {
			parts = append(parts, fmt.Sprintf("  ← %s from %s (%.0f%%)", r.Type, source.Name, r.Weight*100))
		}
	}

	return strings.Join(parts, "\n")
}

// RemoveEntity deletes an entity and all its relations.
func (g *Graph) RemoveEntity(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	e, ok := g.entities[id]
	if !ok {
		return false
	}

	// Remove all outgoing relations
	for _, rid := range g.adjOut[id] {
		r := g.relations[rid]
		g.adjIn[r.ToID] = removeFromSlice(g.adjIn[r.ToID], rid)
		delete(g.relations, rid)
	}
	delete(g.adjOut, id)

	// Remove all incoming relations
	for _, rid := range g.adjIn[id] {
		r := g.relations[rid]
		g.adjOut[r.FromID] = removeFromSlice(g.adjOut[r.FromID], rid)
		delete(g.relations, rid)
	}
	delete(g.adjIn, id)

	delete(g.nameIdx, strings.ToLower(e.Name))
	delete(g.entities, id)
	return true
}

// Stats returns graph statistics.
func (g *Graph) Stats() map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]int{
		"entities":  len(g.entities),
		"relations": len(g.relations),
	}
}

func removeFromSlice(s []string, v string) []string {
	for i, item := range s {
		if item == v {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
