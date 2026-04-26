package cogni

import (
	"context"
	"log/slog"
	"math"
	"strings"
)

// PerceptionRule declares a multi-modal activation signal.
type PerceptionRule struct {
	Type      string   `json:"type" yaml:"type"`               // semantic | context_chain | file_watcher | schedule | webhook
	Threshold float64  `json:"threshold,omitempty" yaml:"threshold,omitempty"` // min similarity (semantic)
	Window    int      `json:"window,omitempty" yaml:"window,omitempty"`       // context chain lookback
	Topics    []string `json:"topics,omitempty" yaml:"topics,omitempty"`       // context chain topic keywords
	Patterns  []string `json:"patterns,omitempty" yaml:"patterns,omitempty"`   // file watcher glob patterns
	Events    []string `json:"events,omitempty" yaml:"events,omitempty"`       // file watcher event types
	Cron      string   `json:"cron,omitempty" yaml:"cron,omitempty"`           // schedule cron expression
	Path      string   `json:"path,omitempty" yaml:"path,omitempty"`           // webhook endpoint path
	Weight    float64  `json:"weight,omitempty" yaml:"weight,omitempty"`       // score contribution (default 0.5)
}

// PerceptionSignal carries runtime perception data from the host into the
// evaluator. The host populates the relevant fields based on what happened.
type PerceptionSignal struct {
	// Semantic: pre-computed embedding similarity against the cogni's domain.
	SemanticSimilarity float64

	// ContextChain: recent conversation topics (lowercased).
	RecentTopics []string

	// FileEvent: file change that triggered this evaluation.
	FileEvent *FileChangeEvent

	// Schedule: whether this evaluation was triggered by a cron schedule.
	ScheduleTriggered bool
	ScheduleCron      string

	// Webhook: whether this evaluation was triggered by a webhook.
	WebhookTriggered bool
	WebhookPath      string
}

// FileChangeEvent represents a file system change.
type FileChangeEvent struct {
	Path  string `json:"path"`
	Event string `json:"event"` // modified | created | deleted
}

// SemanticProvider is called by the evaluator to compute semantic similarity
// for cognis that declare semantic perception. The host provides this.
type SemanticProvider func(ctx context.Context, query string, domain string) float64

// evaluatePerception scores perception rules against runtime signals.
// Returns additional score contribution and reasons.
func evaluatePerception(rules []PerceptionRule, session Session, signal *PerceptionSignal) (float64, []string) {
	if len(rules) == 0 || signal == nil {
		return 0, nil
	}

	totalScore := 0.0
	var reasons []string

	for _, rule := range rules {
		weight := rule.Weight
		if weight <= 0 {
			weight = 0.5
		}

		switch rule.Type {
		case "semantic":
			if score := evaluateSemantic(rule, signal); score > 0 {
				totalScore += score * weight
				reasons = append(reasons, "perception:semantic")
			}

		case "context_chain":
			if score := evaluateContextChain(rule, session, signal); score > 0 {
				totalScore += score * weight
				reasons = append(reasons, "perception:context_chain")
			}

		case "file_watcher":
			if score := evaluateFileWatcher(rule, signal); score > 0 {
				totalScore += score * weight
				reasons = append(reasons, "perception:file_watcher")
			}

		case "schedule":
			if signal.ScheduleTriggered && signal.ScheduleCron == rule.Cron {
				totalScore += weight
				reasons = append(reasons, "perception:schedule:"+rule.Cron)
			}

		case "webhook":
			if signal.WebhookTriggered && signal.WebhookPath == rule.Path {
				totalScore += weight
				reasons = append(reasons, "perception:webhook:"+rule.Path)
			}

		default:
			slog.Warn("perception: unknown rule type", "type", rule.Type)
		}
	}

	return totalScore, reasons
}

func evaluateSemantic(rule PerceptionRule, signal *PerceptionSignal) float64 {
	threshold := rule.Threshold
	if threshold <= 0 {
		threshold = 0.8
	}
	if signal.SemanticSimilarity >= threshold {
		return signal.SemanticSimilarity
	}
	return 0
}

func evaluateContextChain(rule PerceptionRule, session Session, signal *PerceptionSignal) float64 {
	if len(rule.Topics) == 0 || len(signal.RecentTopics) == 0 {
		return 0
	}

	window := rule.Window
	if window <= 0 {
		window = 3
	}

	topics := signal.RecentTopics
	if len(topics) > window {
		topics = topics[len(topics)-window:]
	}

	matched := 0
	for _, topic := range topics {
		for _, target := range rule.Topics {
			if strings.EqualFold(topic, target) {
				matched++
				break
			}
		}
	}

	if matched == 0 {
		return 0
	}

	// Score based on how many of the target topics appeared in the window.
	coverage := float64(matched) / float64(len(rule.Topics))
	recency := float64(matched) / float64(len(topics))
	return math.Min(1.0, (coverage+recency)/2.0)
}

func evaluateFileWatcher(rule PerceptionRule, signal *PerceptionSignal) float64 {
	if signal.FileEvent == nil {
		return 0
	}

	eventMatch := false
	if len(rule.Events) == 0 {
		eventMatch = true
	} else {
		for _, e := range rule.Events {
			if e == signal.FileEvent.Event {
				eventMatch = true
				break
			}
		}
	}
	if !eventMatch {
		return 0
	}

	if len(rule.Patterns) == 0 {
		return 1.0
	}

	for _, pattern := range rule.Patterns {
		if simpleGlobMatch(signal.FileEvent.Path, pattern) {
			return 1.0
		}
	}
	return 0
}

// simpleGlobMatch handles *.ext patterns without pulling in filepath.Match
// to keep this package dependency-free for the evaluator hot path.
func simpleGlobMatch(path, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // ".go"
		return strings.HasSuffix(path, ext)
	}
	if strings.HasSuffix(pattern, "/*") {
		dir := pattern[:len(pattern)-2]
		return strings.HasPrefix(path, dir+"/") || strings.HasPrefix(path, dir+"\\")
	}
	return strings.Contains(path, pattern)
}
