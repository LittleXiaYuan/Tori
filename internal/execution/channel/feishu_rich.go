package channel

import "strings"

// buildCardFromFeishuRich builds an interactive card when Rich contains buttons.
func buildCardFromFeishuRich(reply Reply) *Card {
	if reply.Rich == nil || !reply.Rich.HasType(ComponentButton) {
		return nil
	}
	var md strings.Builder
	md.WriteString(reply.Content)
	for _, c := range reply.Rich.Components {
		if t, ok := c.(*TextComponent); ok && t.Content != "" {
			if md.Len() > 0 {
				md.WriteString("\n\n")
			}
			md.WriteString(t.Content)
		}
	}
	card := NewCard("云雀助手", CardBlue)
	if strings.TrimSpace(md.String()) != "" {
		card.AddMarkdown(md.String())
	}
	for _, c := range reply.Rich.Components {
		if b, ok := c.(*ButtonComponent); ok && b.URL == "" {
			val := b.Value
			if val == "" {
				val = b.Label
			}
			card.AddButton(b.Label, BtnPrimary, map[string]string{"reply": val})
		}
	}
	return card
}
