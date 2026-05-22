package planner

import (
	"errors"
	"testing"
)

func TestSkillTrustGateCheck(t *testing.T) {
	gate := NewSkillTrustGate()
	blocked := errors.New("blocked")
	gate.SetCheck(func(skillName string) error {
		if skillName != "dangerous_skill" {
			t.Fatalf("unexpected skill name: %s", skillName)
		}
		return blocked
	})

	if err := gate.Check("dangerous_skill"); !errors.Is(err, blocked) {
		t.Fatalf("expected blocked error, got %v", err)
	}
}

func TestSkillTrustGateRecord(t *testing.T) {
	gate := NewSkillTrustGate()
	var gotName string
	var gotSuccess bool
	gate.SetRecord(func(skillName string, success bool) {
		gotName = skillName
		gotSuccess = success
	})

	gate.Record("file_read", true)
	if gotName != "file_read" || !gotSuccess {
		t.Fatalf("unexpected record callback values: name=%q success=%v", gotName, gotSuccess)
	}
}

func TestNilSkillTrustGateIsNoop(t *testing.T) {
	var gate *SkillTrustGate
	if err := gate.Check("anything"); err != nil {
		t.Fatalf("nil gate should allow by default, got %v", err)
	}
	gate.Record("anything", false)
}
