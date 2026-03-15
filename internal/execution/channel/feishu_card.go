package channel

import "encoding/json"

// ──────────────────────────────────────────────
// Feishu Interactive Card Builder
// Ref: https://open.feishu.cn/document/common-capabilities/message-card
// ──────────────────────────────────────────────

// CardColor defines card header color themes.
type CardColor string

const (
	CardBlue   CardColor = "blue"
	CardGreen  CardColor = "green"
	CardOrange CardColor = "orange"
	CardRed    CardColor = "red"
	CardGrey   CardColor = "grey"
	CardPurple CardColor = "purple"
)

// Card builds a Feishu interactive card message.
type Card struct {
	header   *cardHeader
	elements []any
}

type cardHeader struct {
	Title    string    `json:"title"`
	Template CardColor `json:"template,omitempty"`
}

// NewCard creates a card with a title and optional color.
func NewCard(title string, color CardColor) *Card {
	return &Card{
		header: &cardHeader{Title: title, Template: color},
	}
}

// AddMarkdown adds a markdown content block.
func (c *Card) AddMarkdown(content string) *Card {
	c.elements = append(c.elements, map[string]any{
		"tag":     "markdown",
		"content": content,
	})
	return c
}

// AddText adds a plain text block.
func (c *Card) AddText(text string) *Card {
	c.elements = append(c.elements, map[string]any{
		"tag": "div",
		"text": map[string]string{
			"tag":     "plain_text",
			"content": text,
		},
	})
	return c
}

// AddDivider adds a horizontal divider.
func (c *Card) AddDivider() *Card {
	c.elements = append(c.elements, map[string]string{"tag": "hr"})
	return c
}

// AddNote adds a note (small grey text) at the bottom.
func (c *Card) AddNote(texts ...string) *Card {
	elems := make([]map[string]string, len(texts))
	for i, t := range texts {
		elems[i] = map[string]string{"tag": "plain_text", "content": t}
	}
	c.elements = append(c.elements, map[string]any{
		"tag":      "note",
		"elements": elems,
	})
	return c
}

// ButtonStyle defines button appearance.
type ButtonStyle string

const (
	BtnPrimary ButtonStyle = "primary"
	BtnDanger  ButtonStyle = "danger"
	BtnDefault ButtonStyle = ""
)

// AddButton adds an action button.
func (c *Card) AddButton(text string, style ButtonStyle, value map[string]string) *Card {
	btn := map[string]any{
		"tag": "button",
		"text": map[string]string{
			"tag":     "plain_text",
			"content": text,
		},
		"value": value,
	}
	if style != "" {
		btn["type"] = string(style)
	}

	// Find or create action block
	if len(c.elements) > 0 {
		last := c.elements[len(c.elements)-1]
		if m, ok := last.(map[string]any); ok {
			if m["tag"] == "action" {
				if actions, ok := m["actions"].([]any); ok {
					m["actions"] = append(actions, btn)
					return c
				}
			}
		}
	}

	c.elements = append(c.elements, map[string]any{
		"tag":     "action",
		"actions": []any{btn},
	})
	return c
}

// AddImage adds an image block.
func (c *Card) AddImage(imageKey, alt string) *Card {
	c.elements = append(c.elements, map[string]any{
		"tag":     "img",
		"img_key": imageKey,
		"alt": map[string]string{
			"tag":     "plain_text",
			"content": alt,
		},
	})
	return c
}

// Build returns the card as a JSON string ready for Feishu API.
func (c *Card) Build() string {
	card := map[string]any{
		"elements": c.elements,
	}
	if c.header != nil {
		card["header"] = map[string]any{
			"title": map[string]string{
				"tag":     "plain_text",
				"content": c.header.Title,
			},
			"template": string(c.header.Template),
		}
	}
	data, _ := json.Marshal(card)
	return string(data)
}

// ──────────────────────────────────────────────
// Quick card templates for common agent responses
// ──────────────────────────────────────────────

// AgentReplyCard creates a standard agent reply card.
func AgentReplyCard(title, markdown string) *Card {
	return NewCard(title, CardBlue).AddMarkdown(markdown)
}

// ErrorCard creates an error notification card.
func ErrorCard(message string) *Card {
	return NewCard("错误提示", CardRed).AddMarkdown(message)
}

// ConfirmCard creates a confirmation card with Yes/No buttons.
func ConfirmCard(question string, yesValue, noValue map[string]string) *Card {
	return NewCard("确认", CardOrange).
		AddMarkdown(question).
		AddDivider().
		AddButton("确认", BtnPrimary, yesValue).
		AddButton("取消", BtnDefault, noValue)
}
