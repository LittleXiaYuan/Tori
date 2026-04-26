package cogni

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// BundleSchema is the schema identifier written into every exported Bundle.
// Consumers MUST check this before trusting the payload so older readers
// don't silently misinterpret a future format.
const BundleSchema = "yunque-cogni/v1"

// Bundle is the portable representation of a set of Cogni declarations.
// It is the unit of sharing between instances ("cogni hub"): a user
// exports a Bundle from one agent, commits it to git, imports it on
// another. Everything needed to re-create the cognis is in one JSON file
// — no external deps, no side-channel state.
type Bundle struct {
	Schema     string         `json:"schema"`
	ExportedAt time.Time      `json:"exported_at"`
	ExportedBy string         `json:"exported_by,omitempty"`
	Notes      string         `json:"notes,omitempty"`
	Cognis     []*Declaration `json:"cognis"`
}

// ExportBundle produces a Bundle containing the declarations identified by
// `ids`. An empty ids slice exports every registered declaration. Unknown
// IDs are skipped silently — the caller is expected to present the final
// list back to the user.
func (r *Registry) ExportBundle(ids []string, notes string) *Bundle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	targetIDs := ids
	if len(targetIDs) == 0 {
		targetIDs = make([]string, 0, len(r.entries))
		for id := range r.entries {
			targetIDs = append(targetIDs, id)
		}
		sort.Strings(targetIDs)
	}

	cognis := make([]*Declaration, 0, len(targetIDs))
	for _, id := range targetIDs {
		e, ok := r.entries[id]
		if !ok {
			continue
		}
		cognis = append(cognis, cloneDeclaration(e.decl))
	}
	return &Bundle{
		Schema:     BundleSchema,
		ExportedAt: time.Now().UTC(),
		Notes:      notes,
		Cognis:     cognis,
	}
}

// ImportBundle loads every declaration from `b` into the registry. When
// `overwrite` is false, cognis whose ID already exists are reported in
// the Skipped slice and left untouched; when true, they are replaced
// with the bundled version.
//
// Returns per-declaration outcomes so the admin UI can surface "3
// imported, 1 skipped, 1 failed" without asking again.
func (r *Registry) ImportBundle(b *Bundle, overwrite bool) (ImportSummary, error) {
	if b == nil {
		return ImportSummary{}, fmt.Errorf("cogni: nil bundle")
	}
	if b.Schema != "" && b.Schema != BundleSchema {
		return ImportSummary{}, fmt.Errorf("cogni: unsupported bundle schema %q (expected %q)", b.Schema, BundleSchema)
	}

	sum := ImportSummary{}
	for _, d := range b.Cognis {
		if d == nil {
			continue
		}
		if err := d.Validate(); err != nil {
			sum.Failed = append(sum.Failed, ImportFailure{ID: d.ID, Err: err.Error()})
			continue
		}

		r.mu.Lock()
		_, existed := r.entries[d.ID]
		r.mu.Unlock()

		if existed && !overwrite {
			sum.Skipped = append(sum.Skipped, d.ID)
			continue
		}

		if err := r.Add(cloneDeclaration(d), "bundle"); err != nil {
			sum.Failed = append(sum.Failed, ImportFailure{ID: d.ID, Err: err.Error()})
			continue
		}
		if existed {
			sum.Updated = append(sum.Updated, d.ID)
		} else {
			sum.Added = append(sum.Added, d.ID)
		}
	}
	return sum, nil
}

// ImportSummary is the per-declaration outcome of ImportBundle.
type ImportSummary struct {
	Added   []string        `json:"added,omitempty"`
	Updated []string        `json:"updated,omitempty"`
	Skipped []string        `json:"skipped,omitempty"`
	Failed  []ImportFailure `json:"failed,omitempty"`
}

// ImportFailure records a single declaration that could not be imported.
type ImportFailure struct {
	ID  string `json:"id"`
	Err string `json:"error"`
}

// cloneDeclaration returns a deep copy so later mutations on the bundle
// side don't surprise the registry (and vice versa). The struct is small
// so a JSON round-trip is the simplest correct implementation.
func cloneDeclaration(d *Declaration) *Declaration {
	if d == nil {
		return nil
	}
	data, err := json.Marshal(d)
	if err != nil {
		return d // fallback; any JSON error on marshal is unexpected
	}
	var out Declaration
	if err := json.Unmarshal(data, &out); err != nil {
		return d
	}
	return &out
}
