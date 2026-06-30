package gateway

import "testing"

// TestCogniIDList verifies the chat composer's single `/智能体` pick is
// normalized into PlanRequest.ForceCogniIDs (nil when unset, so an ordinary
// turn stays score-driven).
func TestCogniIDList(t *testing.T) {
	if got := cogniIDList(""); got != nil {
		t.Fatalf("empty id should yield nil, got %#v", got)
	}
	if got := cogniIDList("   "); got != nil {
		t.Fatalf("blank id should yield nil, got %#v", got)
	}
	got := cogniIDList("office-assistant")
	if len(got) != 1 || got[0] != "office-assistant" {
		t.Fatalf("expected [office-assistant], got %#v", got)
	}
}
