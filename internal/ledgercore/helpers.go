package ledger

import (
	"encoding/json"
	"time"
)

// TruncateStr truncates a string to maxLen runes, appending "..." if truncated.
func TruncateStr(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}

// MakePayload builds a JSON payload from a map.
func MakePayload(m map[string]interface{}) JSON {
	b, _ := json.Marshal(m)
	return b
}

// QuickReflection creates a simple reflection without LLM (for testing or fallback).
func QuickReflection(satisfied bool, score float64, suggestion string) *Reflection {
	return &Reflection{
		Satisfied:  satisfied,
		Score:      score,
		Suggestion: suggestion,
	}
}

// ReflectionToJSON serializes a Reflection for storage.
func ReflectionToJSON(r *Reflection) JSON {
	b, _ := json.Marshal(r)
	return b
}

// ReflectionFromJSON deserializes a Reflection.
func ReflectionFromJSON(data JSON) (*Reflection, error) {
	r := &Reflection{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, err
	}
	return r, nil
}

// ExtractExperience creates an experience entry from a PER result.
func ExtractExperience(taskID, goal string, result *PERResult) *ExperienceEntry {
	outcome := "failure"
	if result.Success {
		outcome = "success"
	} else if len(result.Reflections) > 0 && result.Reflections[len(result.Reflections)-1].Score > 0.5 {
		outcome = "partial"
	}

	var allLearnings []string
	for _, r := range result.Reflections {
		allLearnings = append(allLearnings, r.Learnings...)
	}

	score := 0.0
	if len(result.Reflections) > 0 {
		score = result.Reflections[len(result.Reflections)-1].Score
	}

	return &ExperienceEntry{
		TaskID:    taskID,
		Goal:      goal,
		Outcome:   outcome,
		Learnings: allLearnings,
		Score:     score,
		Attempts:  result.Attempts,
		CreatedAt: time.Now(),
	}
}

// clamp constrains v to [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
