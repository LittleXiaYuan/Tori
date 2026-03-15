package adaptive

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// persistData is the serializable form of the adaptive loop state.
type persistData struct {
	Feedbacks   []Feedback                    `json:"feedbacks"`
	Corrections map[string]*CorrectionPattern `json:"corrections"`
	Rules       map[string]*AdaptationRule    `json:"rules"`
	Profile     BehaviorProfile               `json:"profile"`
}

// SaveTo persists the loop state to a JSON file.
func (l *Loop) SaveTo(path string) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	d := persistData{
		Feedbacks:   l.feedbacks,
		Corrections: l.corrections,
		Rules:       l.rules,
		Profile:     l.profile,
	}

	os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadFrom restores the loop state from a JSON file.
func (l *Loop) LoadFrom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var d persistData
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if d.Feedbacks != nil {
		l.feedbacks = d.Feedbacks
	}
	if d.Corrections != nil {
		l.corrections = d.Corrections
	}
	if d.Rules != nil {
		l.rules = d.Rules
	}
	if d.Profile.Settings != nil {
		l.profile = d.Profile
	}

	slog.Info("adaptive: loaded", "feedbacks", len(l.feedbacks), "rules", len(l.rules), "version", l.profile.Version)
	return nil
}
