package planner

// RichReplyHints returns short hints for clients that cannot render AgentAction natively.
// Primary rich rendering lives in the gateway (RenderAgentActions + AttachFilesToRich).
func (r *PlanResult) RichReplyHints() []string {
	if r == nil || len(r.Actions) == 0 {
		return nil
	}
	var out []string
	for _, a := range r.Actions {
		switch a.Kind {
		case ActionAsk:
			if p, ok := a.Payload.(AskPayload); ok {
				out = append(out, p.Question)
			}
		case ActionConfirm:
			if p, ok := a.Payload.(ConfirmPayload); ok {
				out = append(out, p.Message)
			}
		case ActionSuggest:
			if p, ok := a.Payload.(SuggestPayload); ok {
				for _, s := range p.Suggestions {
					out = append(out, s.Label+": "+s.Prompt)
				}
			}
		}
	}
	return out
}
