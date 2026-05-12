package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"yunque-agent/sdk/go/yunque"
)

type stateSummary struct {
	Focus          string   `json:"focus,omitempty"`
	GoalCount      int      `json:"goal_count"`
	ResourceCount  int      `json:"resource_count"`
	RecentActions  []string `json:"recent_actions,omitempty"`
	TotalSkills    int      `json:"total_skills"`
	UnresolvedGaps int      `json:"unresolved_gaps"`
}

func main() {
	ctx := context.Background()
	snap, err := yunque.State.Snapshot(ctx)
	if err != nil {
		log.Fatalf("load state snapshot: %v", err)
	}

	summary := stateSummary{
		Focus:          snap.Focus,
		GoalCount:      len(snap.Goals),
		ResourceCount:  len(snap.Resources),
		TotalSkills:    snap.Capabilities.TotalSkills,
		UnresolvedGaps: snap.Capabilities.UnresolvedGaps,
	}
	for _, action := range snap.RecentActions {
		summary.RecentActions = append(summary.RecentActions, action.Action)
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		log.Fatalf("marshal state summary: %v", err)
	}
	fmt.Println(string(data))
}
