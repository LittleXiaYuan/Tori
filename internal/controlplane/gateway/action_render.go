package gateway

import (
	"path/filepath"
	"strings"

	"yunque-agent/internal/agentcore/planner"
	channelpkg "yunque-agent/internal/execution/channel"
)

// RenderAgentActions converts structured planner actions into a RichMessage for IM adapters.
func RenderAgentActions(actions []planner.AgentAction) *channelpkg.RichMessage {
	if len(actions) == 0 {
		return nil
	}
	rm := channelpkg.NewRichMessage()
	for _, a := range actions {
		switch a.Kind {
		case planner.ActionAsk:
			switch p := a.Payload.(type) {
			case planner.AskPayload:
				if p.Question != "" {
					rm.AddText(p.Question)
				}
				for _, o := range p.Options {
					lbl, val := o.Label, o.Value
					if lbl == "" {
						continue
					}
					if val == "" {
						val = lbl
					}
					rm.Add(channelpkg.NewButton(lbl, val, "default"))
				}
			case map[string]any:
				q, _ := p["question"].(string)
				if q != "" {
					rm.AddText(q)
				}
				if opts, ok := p["options"].([]any); ok {
					for _, o := range opts {
						om, _ := o.(map[string]any)
						lbl, _ := om["label"].(string)
						val, _ := om["value"].(string)
						if lbl == "" {
							continue
						}
						if val == "" {
							val = lbl
						}
						rm.Add(channelpkg.NewButton(lbl, val, "default"))
					}
				}
			}
		case planner.ActionConfirm:
			var msg, yes, no string
			var destructive bool
			switch p := a.Payload.(type) {
			case planner.ConfirmPayload:
				msg, yes, no, destructive = p.Message, p.YesLabel, p.NoLabel, p.Destructive
			case map[string]any:
				msg, _ = p["message"].(string)
				yes, _ = p["yes_label"].(string)
				no, _ = p["no_label"].(string)
				destructive, _ = p["destructive"].(bool)
			default:
				continue
			}
			if msg != "" {
				rm.AddText(msg)
			}
			if yes == "" {
				yes = "是"
			}
			if no == "" {
				no = "否"
			}
			style := "primary"
			if destructive {
				style = "danger"
			}
			rm.Add(channelpkg.NewButton(yes, "__confirm_yes__", style))
			rm.Add(channelpkg.NewButton(no, "__confirm_no__", "default"))
		case planner.ActionShowFile:
			addFileComponent(rm, a.Payload)
		case planner.ActionSuggest:
			switch p := a.Payload.(type) {
			case planner.SuggestPayload:
				if len(p.Suggestions) == 0 {
					continue
				}
				rm.AddText("你可以试试：")
				for _, s := range p.Suggestions {
					lbl, pr := s.Label, s.Prompt
					if lbl == "" {
						continue
					}
					if pr == "" {
						pr = lbl
					}
					rm.Add(channelpkg.NewButton(lbl, pr, "default"))
				}
			case map[string]any:
				if list, ok := p["suggestions"].([]any); ok {
					rm.AddText("你可以试试：")
					for _, it := range list {
						sm, _ := it.(map[string]any)
						lbl, _ := sm["label"].(string)
						pr, _ := sm["prompt"].(string)
						if lbl == "" {
							continue
						}
						if pr == "" {
							pr = lbl
						}
						rm.Add(channelpkg.NewButton(lbl, pr, "default"))
					}
				}
			}
		case planner.ActionProgress:
			switch p := a.Payload.(type) {
			case planner.ProgressPayload:
				if p.Message != "" {
					rm.AddText(p.Message)
				}
			case map[string]any:
				msg, _ := p["message"].(string)
				if msg != "" {
					rm.AddText(msg)
				}
			}
		case planner.ActionRequestInput:
			switch p := a.Payload.(type) {
			case planner.InputRequestPayload:
				if p.Question != "" {
					rm.AddText(p.Question)
				}
			case map[string]any:
				q, _ := p["question"].(string)
				if q != "" {
					rm.AddText(q)
				}
			}
		default:
			// ignore unknown kinds
		}
	}
	if len(rm.Components) == 0 {
		return nil
	}
	return rm
}

func addFileComponent(rm *channelpkg.RichMessage, payload any) {
	switch fp := payload.(type) {
	case planner.FilePayload:
		attachLocalFile(rm, fp.Path, fp.Name, fp.MimeType, fp.Size)
	case map[string]any:
		path, _ := fp["path"].(string)
		name, _ := fp["name"].(string)
		mime, _ := fp["mime_type"].(string)
		var size int64
		switch v := fp["size"].(type) {
		case float64:
			size = int64(v)
		case int64:
			size = v
		}
		attachLocalFile(rm, path, name, mime, size)
	}
}

func attachLocalFile(rm *channelpkg.RichMessage, path, name, mime string, size int64) {
	if path == "" {
		return
	}
	if name == "" {
		name = filepath.Base(path)
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"),
		strings.HasSuffix(lower, ".gif"), strings.HasSuffix(lower, ".webp"):
		if mime == "" {
			mime = "image/" + strings.TrimPrefix(filepath.Ext(lower), ".")
			if strings.Contains(mime, "jpg") {
				mime = "image/jpeg"
			}
		}
		img := channelpkg.NewImageFromURL("file://"+filepath.ToSlash(path), name)
		rm.Add(img)
	default:
		f := channelpkg.NewFile("file://"+filepath.ToSlash(path), name)
		f.MimeType = mime
		f.FileSize = size
		rm.Add(f)
	}
}
