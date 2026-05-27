package risk

import "testing"

func TestLevelString(t *testing.T) {
	if Low.String() != "low" {
		t.Fatalf("Low.String() = %q", Low.String())
	}
	if Level("").String() != "unknown" {
		t.Fatalf("empty risk string = %q", Level("").String())
	}
}

func TestLevelAtLeast(t *testing.T) {
	tests := []struct {
		level     Level
		threshold Level
		want      bool
	}{
		{Low, Low, true},
		{Medium, Low, true},
		{Medium, High, false},
		{High, Medium, true},
		{Critical, High, true},
		{Level("unknown"), Low, false},
	}
	for _, tt := range tests {
		if got := tt.level.AtLeast(tt.threshold); got != tt.want {
			t.Fatalf("%s.AtLeast(%s) = %v, want %v", tt.level, tt.threshold, got, tt.want)
		}
	}
}
