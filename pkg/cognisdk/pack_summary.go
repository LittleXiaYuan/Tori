package cognisdk

import (
	"fmt"
	"sort"
	"strings"
)

// SummarizePackBundle validates a portable bundle and returns a compact view
// suitable for frontends, plugin pages, and automation logs.
func SummarizePackBundle(bundle PackBundle) (PackBundleSummary, error) {
	if err := ValidatePackBundle(bundle); err != nil {
		return PackBundleSummary{}, err
	}
	digest, err := DigestPackBundle(bundle)
	if err != nil {
		return PackBundleSummary{}, err
	}
	enabled := stringSet(bundle.EnabledPacks)
	packs := make([]PackStatus, 0, len(bundle.Packs))
	goldenCount := 0
	for _, pack := range bundle.Packs {
		packs = append(packs, statusFromPack(pack, enabled[pack.ID]))
		goldenCount += len(pack.GoldenTests)
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].ID < packs[j].ID })
	summary := PackBundleSummary{
		ID:              bundle.ID,
		Version:         bundle.Version,
		Digest:          digest,
		PackCount:       len(bundle.Packs),
		EnabledCount:    len(bundle.EnabledPacks),
		GoldenTestCount: goldenCount,
		Packs:           packs,
	}
	summary.DisabledCount = summary.PackCount - summary.EnabledCount
	if summary.DisabledCount < 0 {
		summary.DisabledCount = 0
	}
	return summary, nil
}

// RenderPackBundleSummaryMarkdown renders a bundle inspection summary. It does
// not run golden tests, apply packs, or mutate host state.
func RenderPackBundleSummaryMarkdown(summary PackBundleSummary) string {
	var b strings.Builder
	b.WriteString("## Cogni Pack Bundle Summary\n\n")
	fmt.Fprintf(&b, "- id: %s\n", emptyAs(summary.ID, "unknown"))
	fmt.Fprintf(&b, "- version: %d\n", summary.Version)
	fmt.Fprintf(&b, "- packs: %d\n", summary.PackCount)
	fmt.Fprintf(&b, "- enabled: %d\n", summary.EnabledCount)
	fmt.Fprintf(&b, "- disabled: %d\n", summary.DisabledCount)
	fmt.Fprintf(&b, "- golden_tests: %d\n", summary.GoldenTestCount)
	writePackStatuses(&b, "Packs", summary.Packs)
	return strings.TrimSpace(b.String()) + "\n"
}
