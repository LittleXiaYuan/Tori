package risk

// Level is the shared cross-package risk grade used by review, approval, and
// task constraints. Keep it string-backed so API JSON remains stable and
// human-readable.
type Level string

const (
	Low      Level = "low"
	Medium   Level = "medium"
	High     Level = "high"
	Critical Level = "critical"
)

func (l Level) String() string {
	if l == "" {
		return "unknown"
	}
	return string(l)
}

// AtLeast reports whether l is at or above threshold in the canonical ordering.
func (l Level) AtLeast(threshold Level) bool {
	return rank(l) >= rank(threshold)
}

func rank(l Level) int {
	switch l {
	case Low:
		return 1
	case Medium:
		return 2
	case High:
		return 3
	case Critical:
		return 4
	default:
		return 0
	}
}
