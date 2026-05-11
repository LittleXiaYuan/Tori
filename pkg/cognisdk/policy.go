package cognisdk

import "strings"

func detectPerception(input Input) PerceptionState {
	msg := strings.TrimSpace(input.Message)
	lower := strings.ToLower(msg)

	p := PerceptionState{
		Message:             msg,
		Intent:              "general",
		Risk:                RiskLow,
		Hints:               input.Hints,
		RequestedToolAction: input.RequestedToolAction,
	}

	if hint := strings.TrimSpace(input.Hints["intent"]); hint != "" {
		p.Intent = hint
		p.Signals = append(p.Signals, "hint:intent")
	}
	if hint := strings.TrimSpace(input.Hints["risk"]); hint != "" {
		p.Risk = RiskLevel(hint)
		p.Signals = append(p.Signals, "hint:risk")
	}

	if containsAny(lower, "永远陪", "一直陪", "不要离开", "别离开", "不会离开", "forever", "always be with me") {
		p.Intent = "seek_reassurance"
		p.Risk = RiskDependency
		p.Signals = append(p.Signals, "dependency_reassurance")
	} else if containsAny(lower, "安全感", "不安", "害怕", "焦虑", "安慰", "陪陪我", "reassure", "comfort") {
		p.Intent = "seek_reassurance"
		if p.Risk == RiskLow {
			p.Risk = RiskMedium
		}
		p.Signals = append(p.Signals, "emotional_reassurance")
	}

	if containsAny(lower, "实现", "修复", "代码", "测试", "文件", "项目", "任务", "交付", "重构", "debug", "bug", "implement", "fix", "test", "workspace") {
		p.Intent = "work_task"
		p.Signals = append(p.Signals, "work_task")
	}

	if input.RequestedToolAction != nil && riskRank(input.RequestedToolAction.Risk) > riskRank(p.Risk) {
		p.Risk = input.RequestedToolAction.Risk
		p.Signals = append(p.Signals, "tool_risk")
	}

	return p
}

func buildDisposition(p PerceptionState, merged MergedPack) ResponseDisposition {
	d := ResponseDisposition{
		Mode:       "balanced",
		Tone:       "clear",
		Priority:   100,
		ToolPolicy: ToolPolicyAllow,
		MustSay:    append([]string(nil), merged.Boundary.MustSay...),
		MustAvoid:  append([]string(nil), merged.Boundary.MustAvoid...),
	}

	matched := make([]DispositionRule, 0)
	for _, rule := range merged.DispositionRules {
		if ruleMatches(rule, p) {
			matched = append(matched, rule)
		}
	}
	sortRules(matched)

	for i, rule := range matched {
		if i == 0 {
			if rule.Mode != "" {
				d.Mode = rule.Mode
			}
			if rule.Tone != "" {
				d.Tone = rule.Tone
			}
			if rule.Priority != 0 {
				d.Priority = rule.Priority
			}
		}
		d.MustSay = appendUnique(d.MustSay, rule.MustSay...)
		d.MustAvoid = appendUnique(d.MustAvoid, rule.MustAvoid...)
		if rule.ToolPolicy == ToolPolicyRequireConfirmation {
			d.ToolPolicy = ToolPolicyRequireConfirmation
		}
		if rule.ID != "" {
			d.Reasons = append(d.Reasons, "rule:"+rule.ID)
		}
	}

	if p.RequestedToolAction != nil && requiresConfirmation(p.RequestedToolAction, merged.Boundary) {
		d.ToolPolicy = ToolPolicyRequireConfirmation
		d.Reasons = append(d.Reasons, "tool_policy:high_risk")
	}

	d.MustSay = appendUnique(nil, d.MustSay...)
	d.MustAvoid = appendUnique(nil, d.MustAvoid...)
	d.Reasons = appendUnique(nil, d.Reasons...)
	return d
}

func ruleMatches(rule DispositionRule, p PerceptionState) bool {
	when := rule.When
	if when.Intent != "" && when.Intent != p.Intent {
		return false
	}
	if when.Risk != "" && when.Risk != p.Risk {
		return false
	}
	if when.ToolRiskAtLeast != "" {
		if p.RequestedToolAction == nil {
			return false
		}
		if riskRank(p.RequestedToolAction.Risk) < riskRank(when.ToolRiskAtLeast) {
			return false
		}
	}
	if len(when.MessageContainsAny) > 0 && !containsAny(strings.ToLower(p.Message), when.MessageContainsAny...) {
		return false
	}
	return true
}

func requiresConfirmation(action *ToolAction, boundary BoundaryPolicy) bool {
	if action == nil {
		return false
	}
	if action.Risk == RiskHigh || action.Risk == RiskDependency {
		return true
	}
	kind := strings.ToLower(action.Kind)
	name := strings.ToLower(action.Name)
	for _, high := range boundary.HighRiskActions {
		h := strings.ToLower(high)
		if h == kind || h == name || strings.Contains(name, h) {
			return true
		}
	}
	return false
}

func riskRank(r RiskLevel) int {
	switch r {
	case RiskDependency, RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(s, strings.ToLower(n)) {
			return true
		}
	}
	return false
}
