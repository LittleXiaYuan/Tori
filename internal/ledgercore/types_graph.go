package ledger

// ──────────────────────────────────────────────
// Context Graph
// ──────────────────────────────────────────────

// GraphNodeKind classifies a node in the context graph.
type GraphNodeKind string

const (
	NodeUser     GraphNodeKind = "user"
	NodeAgent    GraphNodeKind = "agent"
	NodeTask     GraphNodeKind = "task"
	NodeMemory   GraphNodeKind = "memory"
	NodeArtifact GraphNodeKind = "artifact"
	NodeTopic    GraphNodeKind = "topic"
)

// GraphEdgeKind classifies a relationship between two nodes.
type GraphEdgeKind string

const (
	EdgeCreatedBy   GraphEdgeKind = "created_by"
	EdgeUsedIn      GraphEdgeKind = "used_in"
	EdgeRelatedTo   GraphEdgeKind = "related_to"
	EdgeDependsOn   GraphEdgeKind = "depends_on"
	EdgeMentionedIn GraphEdgeKind = "mentioned_in"
	EdgeDerivedFrom GraphEdgeKind = "derived_from"
)

// GraphNode is a vertex in the context graph.
type GraphNode struct {
	ID       string        `json:"id"        db:"id"`
	Kind     GraphNodeKind `json:"kind"      db:"kind"`
	Label    string        `json:"label"     db:"label"`
	RefID    string        `json:"ref_id"    db:"ref_id"`
	TenantID string        `json:"tenant_id" db:"tenant_id"`
	Metadata JSON          `json:"metadata"  db:"metadata"`
}

// GraphEdge is a directed relationship between two nodes.
type GraphEdge struct {
	ID       string        `json:"id"        db:"id"`
	FromID   string        `json:"from_id"   db:"from_id"`
	ToID     string        `json:"to_id"     db:"to_id"`
	Kind     GraphEdgeKind `json:"kind"      db:"kind"`
	Weight   float64       `json:"weight"    db:"weight"`
	Metadata JSON          `json:"metadata"  db:"metadata"`
}
