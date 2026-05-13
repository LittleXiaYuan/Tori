package cognisdk

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DiffPackBundles compares two portable bundle snapshots without applying
// either one. Callers can render or persist the diff as a rollback review note.
func DiffPackBundles(from, to PackBundle) (PackBundleDiff, error) {
	if err := ValidatePackBundle(from); err != nil {
		return PackBundleDiff{}, fmt.Errorf("from bundle: %w", err)
	}
	if err := ValidatePackBundle(to); err != nil {
		return PackBundleDiff{}, fmt.Errorf("to bundle: %w", err)
	}

	diff := PackBundleDiff{FromID: from.ID, ToID: to.ID}
	fromPacks := packMap(from.Packs)
	toPacks := packMap(to.Packs)

	ids := unionKeys(fromPacks, toPacks)
	for _, id := range ids {
		oldPack, hadOld := fromPacks[id]
		newPack, hasNew := toPacks[id]
		switch {
		case !hadOld && hasNew:
			diff.AddedPacks = append(diff.AddedPacks, statusFromPack(newPack, containsString(to.EnabledPacks, id)))
		case hadOld && !hasNew:
			diff.RemovedPacks = append(diff.RemovedPacks, statusFromPack(oldPack, containsString(from.EnabledPacks, id)))
		case hadOld && hasNew:
			if packFingerprint(oldPack) != packFingerprint(newPack) {
				diff.ChangedPacks = append(diff.ChangedPacks, PackChange{
					ID:          id,
					FromVersion: oldPack.Version,
					ToVersion:   newPack.Version,
					Reason:      packChangeReason(oldPack, newPack),
				})
			}
		}
	}

	fromEnabled := stringSet(from.EnabledPacks)
	toEnabled := stringSet(to.EnabledPacks)
	for _, id := range unionSetKeys(fromEnabled, toEnabled) {
		if !fromEnabled[id] && toEnabled[id] {
			diff.EnabledPacks = append(diff.EnabledPacks, id)
		}
		if fromEnabled[id] && !toEnabled[id] {
			diff.DisabledPacks = append(diff.DisabledPacks, id)
		}
	}
	return diff, nil
}

// RenderPackBundleDiffMarkdown renders a compact review block for bundle
// changes. It is safe for UI previews and automation logs; it does not apply the diff.
func RenderPackBundleDiffMarkdown(diff PackBundleDiff) string {
	var b strings.Builder
	b.WriteString("## Cogni Pack Bundle Diff\n\n")
	fmt.Fprintf(&b, "- from: %s\n", emptyAs(diff.FromID, "unknown"))
	fmt.Fprintf(&b, "- to: %s\n", emptyAs(diff.ToID, "unknown"))

	writePackStatuses(&b, "Added Packs", diff.AddedPacks)
	writePackStatuses(&b, "Removed Packs", diff.RemovedPacks)
	if len(diff.ChangedPacks) > 0 {
		b.WriteString("\n### Changed Packs\n")
		for _, change := range diff.ChangedPacks {
			fmt.Fprintf(&b, "- `%s`: %s -> %s", change.ID, emptyAs(change.FromVersion, "unknown"), emptyAs(change.ToVersion, "unknown"))
			if change.Reason != "" {
				fmt.Fprintf(&b, " (%s)", change.Reason)
			}
			b.WriteString("\n")
		}
	}
	writeList(&b, "Enabled Packs", diff.EnabledPacks)
	writeList(&b, "Disabled Packs", diff.DisabledPacks)
	if len(diff.AddedPacks) == 0 && len(diff.RemovedPacks) == 0 && len(diff.ChangedPacks) == 0 && len(diff.EnabledPacks) == 0 && len(diff.DisabledPacks) == 0 {
		b.WriteString("\nNo pack changes.\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func packMap(packs []PackManifest) map[string]PackManifest {
	out := make(map[string]PackManifest, len(packs))
	for _, pack := range packs {
		out[pack.ID] = pack
	}
	return out
}

func unionKeys(left, right map[string]PackManifest) []string {
	set := make(map[string]bool, len(left)+len(right))
	for id := range left {
		set[id] = true
	}
	for id := range right {
		set[id] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out[value] = true
		}
	}
	return out
}

func unionSetKeys(left, right map[string]bool) []string {
	set := make(map[string]bool, len(left)+len(right))
	for id := range left {
		set[id] = true
	}
	for id := range right {
		set[id] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func packFingerprint(pack PackManifest) string {
	data, err := json.Marshal(pack)
	if err != nil {
		return pack.ID + ":" + pack.Version
	}
	return string(data)
}

func packChangeReason(from, to PackManifest) string {
	if from.Version != to.Version {
		return "version changed"
	}
	return "manifest changed"
}

func statusFromPack(pack PackManifest, enabled bool) PackStatus {
	return PackStatus{
		ID:          pack.ID,
		Version:     pack.Version,
		Type:        pack.Type,
		DisplayName: pack.DisplayName,
		Enabled:     enabled,
		Provides:    append([]string(nil), pack.Provides...),
	}
}

func writePackStatuses(b *strings.Builder, title string, packs []PackStatus) {
	if len(packs) == 0 {
		return
	}
	fmt.Fprintf(b, "\n### %s\n", title)
	for _, pack := range packs {
		fmt.Fprintf(b, "- `%s` %s", pack.ID, pack.Version)
		if pack.Type != "" {
			fmt.Fprintf(b, " (%s)", pack.Type)
		}
		if pack.Enabled {
			b.WriteString(" [enabled]")
		}
		b.WriteString("\n")
	}
}
