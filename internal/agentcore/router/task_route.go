package router

import (
	"strings"

	"yunque-agent/internal/agentcore/llm"
)

// TaskType represents the semantic intent of a user task.
type TaskType string

const (
	TaskCoding      TaskType = "coding"
	TaskReasoning   TaskType = "reasoning"
	TaskCheap       TaskType = "cheap"
	TaskLongContext TaskType = "long_context"
	TaskChinese     TaskType = "china"
	TaskFast        TaskType = "fast"
	TaskMultimodal  TaskType = "multimodal"
	TaskGeneral     TaskType = "general"
)

type taskReqs struct {
	required  []llm.Capability
	preferred []llm.Capability
	tierHint  Tier
}

var taskCapRequirements = map[TaskType]taskReqs{
	TaskCoding: {
		required:  []llm.Capability{llm.CapFunctionCalling},
		preferred: []llm.Capability{llm.CapStructuredOutput, llm.CapStreaming},
		tierHint:  TierExpert,
	},
	TaskReasoning: {
		required:  []llm.Capability{llm.CapReasoning},
		preferred: []llm.Capability{llm.CapFunctionCalling, llm.CapStreaming},
		tierHint:  TierExpert,
	},
	TaskCheap: {
		tierHint: TierFast,
	},
	TaskLongContext: {
		required: []llm.Capability{llm.CapLongContext},
		tierHint: TierSmart,
	},
	TaskChinese: {
		preferred: []llm.Capability{llm.CapFunctionCalling, llm.CapStreaming},
		tierHint:  TierSmart,
	},
	TaskFast: {
		tierHint: TierFast,
	},
	TaskMultimodal: {
		required:  []llm.Capability{llm.CapVision},
		preferred: []llm.Capability{llm.CapAudioIn, llm.CapVideoIn},
		tierHint:  TierSmart,
	},
	TaskGeneral: {
		preferred: []llm.Capability{llm.CapFunctionCalling, llm.CapStreaming},
		tierHint:  TierSmart,
	},
}

// providerAffinity ranks provider IDs for specific task types (index 0 = best).
var providerAffinity = map[TaskType][]string{
	TaskCoding:      {"anthropic", "openai", "deepseek"},
	TaskReasoning:   {"openai", "anthropic", "google"},
	TaskCheap:       {"deepseek", "siliconflow", "gitcode", "qwen"},
	TaskLongContext: {"google", "moonshot", "openai", "anthropic"},
	TaskChinese:     {"qwen", "deepseek", "zhipu", "moonshot", "minimax", "doubao"},
	TaskFast:        {"deepseek", "minimax", "qwen", "siliconflow"},
	TaskMultimodal:  {"google", "openai", "anthropic", "qwen"},
}

// ProviderLister provides the list of registered providers.
type ProviderLister interface {
	List() []llm.ProviderStatus
}

// RouteByTask selects the best provider for a given task type from the registry.
// Returns provider ID, model name, and tier hint.
func RouteByTask(lister ProviderLister, taskType TaskType) (providerID, model string, tier Tier) {
	reqs, ok := taskCapRequirements[taskType]
	if !ok {
		reqs = taskCapRequirements[TaskGeneral]
	}

	providers := lister.List()
	if len(providers) == 0 {
		return "", "", reqs.tierHint
	}

	type scored struct {
		p     llm.ProviderStatus
		score int
	}
	var candidates []scored

	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		s := scoreProvider(p, taskType, reqs)
		if s > 0 {
			candidates = append(candidates, scored{p, s})
		}
	}

	if len(candidates) == 0 {
		for _, p := range providers {
			if p.Enabled {
				return p.ID, p.Model, reqs.tierHint
			}
		}
		return "", "", reqs.tierHint
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}

	return best.p.ID, best.p.Model, reqs.tierHint
}

func scoreProvider(p llm.ProviderStatus, taskType TaskType, reqs taskReqs) int {
	caps := capSet(p.Capabilities)

	for _, req := range reqs.required {
		if !caps[req] {
			return 0
		}
	}

	score := 10

	for _, pref := range reqs.preferred {
		if caps[pref] {
			score += 3
		}
	}

	if affinities, ok := providerAffinity[taskType]; ok {
		for rank, pid := range affinities {
			if matchesProvider(p, pid) {
				score += (len(affinities) - rank) * 5
				break
			}
		}
	}

	switch reqs.tierHint {
	case TierFast:
		if p.Tier == "fast" {
			score += 8
		}
	case TierExpert:
		if p.Tier == "expert" {
			score += 8
		} else if p.Tier == "smart" {
			score += 4
		}
	default:
		if p.Tier == "smart" {
			score += 6
		}
	}

	return score
}

func matchesProvider(p llm.ProviderStatus, providerID string) bool {
	if p.PresetID == providerID {
		return true
	}
	id := strings.ToLower(p.ID)
	return strings.Contains(id, providerID) || strings.HasPrefix(id, providerID)
}

func capSet(caps []llm.Capability) map[llm.Capability]bool {
	m := make(map[llm.Capability]bool, len(caps))
	for _, c := range caps {
		m[c] = true
	}
	return m
}
