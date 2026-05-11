package cognisdk

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders Result into a compact context block a host can pass
// to a planner or prompt builder.
func RenderMarkdown(result Result) string {
	var b strings.Builder

	b.WriteString("## 内心状态\n\n")
	fmt.Fprintf(&b, "- intent: %s\n", emptyAs(result.InnerState.Intent, "general"))
	fmt.Fprintf(&b, "- risk: %s\n", emptyAs(string(result.InnerState.Risk), string(RiskLow)))
	fmt.Fprintf(&b, "- mode: %s\n", emptyAs(result.Disposition.Mode, "balanced"))
	fmt.Fprintf(&b, "- tone: %s\n", emptyAs(result.Disposition.Tone, "clear"))
	fmt.Fprintf(&b, "- tool_policy: %s\n", emptyAs(string(result.Disposition.ToolPolicy), string(ToolPolicyAllow)))

	if result.InnerState.Summary != "" {
		b.WriteString("\n### Inner State\n")
		b.WriteString(result.InnerState.Summary)
		b.WriteString("\n")
	}
	writeList(&b, "Must Say", result.Disposition.MustSay)
	writeList(&b, "Must Avoid", result.Disposition.MustAvoid)

	if len(result.InnerState.ActivePacks) > 0 {
		writeList(&b, "Active Packs", result.InnerState.ActivePacks)
	}
	if len(result.InnerState.ActiveBeliefs) > 0 {
		b.WriteString("\n### Active Beliefs\n")
		for _, belief := range result.InnerState.ActiveBeliefs {
			fmt.Fprintf(&b, "- [%s] %s", belief.Kind, belief.Statement)
			if belief.SourcePack != "" {
				fmt.Fprintf(&b, " (%s)", belief.SourcePack)
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func writeList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n### %s\n", title)
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		fmt.Fprintf(b, "- %s\n", item)
	}
}

func emptyAs(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
