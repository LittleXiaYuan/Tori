package cogni

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Session represents everything the Evaluator needs to know about the current
// conversation turn to decide Cogni activation.
type Session struct {
	// Message is the user's raw message (may be empty for system events).
	Message string

	// TenantID scopes activation per tenant.
	TenantID string

	// Channel is the messaging channel (e.g. "webchat", "telegram").
	Channel string

	// PriorHandover is the set of handover tags emitted by Cognis that ran
	// earlier in the same turn (used by Activation.HandoverOn).
	PriorHandover []string

	// Tags are free-form hints the host can attach (e.g. "admin", "guest").
	Tags []string

	// Perception carries runtime multi-modal signals for advanced activation.
	Perception *PerceptionSignal
}

// Activation is the result of evaluating a single Cogni against a Session.
type Activation struct {
	// Declaration is the evaluated Cogni (same pointer the caller supplied).
	Declaration *Declaration

	// Activated is true when the score reached MinScore.
	Activated bool

	// Score is the computed activation score (0..1, clamped).
	Score float64

	// Reasons lists human-readable why-strings for the UI / audit.
	Reasons []string
}

// Evaluator computes activation scores against a batch of Cogni declarations.
// It caches compiled regexes internally; safe for concurrent use.
type Evaluator struct {
	mu         sync.RWMutex
	regexCache map[string]*regexp.Regexp
}

// NewEvaluator creates a fresh Evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// Evaluate returns activation results for every declaration provided,
// sorted by descending score then ascending priority.
//
// The caller decides what to do with the results: typically only declarations
// with Activated == true are used, and the host may further reduce them via
// Exclusive groups (ApplyExclusivity).
func (e *Evaluator) Evaluate(decls []*Declaration, session Session) []Activation {
	out := make([]Activation, 0, len(decls))
	for _, d := range decls {
		if d == nil {
			continue
		}
		a := e.evaluateOne(d, session)
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		pi, pj := priority(out[i].Declaration), priority(out[j].Declaration)
		return pi < pj
	})
	return out
}

// evaluateOne scores a single declaration. Activation rules combine
// additively; the result is clamped to [0, 1].
func (e *Evaluator) evaluateOne(d *Declaration, s Session) Activation {
	act := Activation{Declaration: d}

	if d.Activation.AlwaysOn {
		act.Activated = true
		act.Score = 1.0
		act.Reasons = append(act.Reasons, "always_on")
		return act
	}

	if !matchesList(s.Channel, d.Activation.Channels) {
		act.Reasons = append(act.Reasons, "channel mismatch: "+s.Channel)
		return act
	}
	if !matchesList(s.TenantID, d.Activation.Tenants) {
		act.Reasons = append(act.Reasons, "tenant mismatch: "+s.TenantID)
		return act
	}

	score := 0.0
	lowerMsg := strings.ToLower(s.Message)

	kwWeight := d.Activation.KeywordWeight
	if kwWeight == 0 {
		kwWeight = 0.3
	}
	for _, kw := range d.Activation.Keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(lowerMsg, strings.ToLower(kw)) {
			score += kwWeight
			act.Reasons = append(act.Reasons, "keyword: "+kw)
		}
	}

	reWeight := d.Activation.RegexWeight
	if reWeight == 0 {
		reWeight = 0.5
	}
	for _, pattern := range d.Activation.Regex {
		re, err := e.compile(pattern)
		if err != nil {
			continue
		}
		if re.MatchString(s.Message) {
			score += reWeight
			act.Reasons = append(act.Reasons, "regex: "+pattern)
		}
	}

	for _, tag := range d.Activation.HandoverOn {
		for _, prior := range s.PriorHandover {
			if tag == prior {
				score += 0.5
				act.Reasons = append(act.Reasons, "handover: "+tag)
				break
			}
		}
	}

	// Multi-modal perception
	if len(d.Activation.Perception) > 0 {
		pScore, pReasons := evaluatePerception(d.Activation.Perception, s, s.Perception)
		score += pScore
		act.Reasons = append(act.Reasons, pReasons...)
	}

	if score > 1.0 {
		score = 1.0
	}

	min := d.Activation.MinScore
	if min == 0 {
		min = 0.5
	}

	act.Score = score
	act.Activated = score >= min
	return act
}

// ApplyExclusivity keeps only the highest-scoring activation per Exclusive
// group, in input order for deterministic behavior. Activations without an
// Exclusive group are passed through unchanged.
func ApplyExclusivity(activations []Activation) []Activation {
	groupSeen := make(map[string]int)
	out := make([]Activation, 0, len(activations))
	for _, a := range activations {
		if !a.Activated {
			out = append(out, a)
			continue
		}
		g := ""
		if a.Declaration != nil {
			g = a.Declaration.Exclusive
		}
		if g == "" {
			out = append(out, a)
			continue
		}
		if prevIdx, seen := groupSeen[g]; seen {
			prev := out[prevIdx]
			if a.Score > prev.Score {
				out[prevIdx] = a
			}
			continue
		}
		groupSeen[g] = len(out)
		out = append(out, a)
	}
	return out
}

// Filtered returns just the Activated ones, preserving order.
func Filtered(activations []Activation) []Activation {
	out := make([]Activation, 0, len(activations))
	for _, a := range activations {
		if a.Activated {
			out = append(out, a)
		}
	}
	return out
}

func matchesList(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == value {
			return true
		}
	}
	return false
}

func priority(d *Declaration) int {
	if d == nil || d.Priority == 0 {
		return 100
	}
	return d.Priority
}

func (e *Evaluator) compile(pattern string) (*regexp.Regexp, error) {
	e.mu.RLock()
	re, ok := e.regexCache[pattern]
	e.mu.RUnlock()
	if ok {
		return re, nil
	}
	// Auto-inject case-insensitive flag unless the pattern already sets flags.
	p := pattern
	if !strings.HasPrefix(p, "(?") {
		p = "(?i)" + p
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.regexCache[pattern] = re
	e.mu.Unlock()
	return re, nil
}
