package main

import (
	"testing"

	"github.com/LittleXiaYuan/ledger"
)

func TestParseVectorANNBackend(t *testing.T) {
	tests := []struct {
		raw   string
		want  ledger.VectorANNBackend
		valid bool
	}{
		{"", ledger.VectorANNBruteForce, true},
		{"bruteforce", ledger.VectorANNBruteForce, true},
		{"brute_force", ledger.VectorANNBruteForce, true},
		{"ivf", ledger.VectorANNIVF, true},
		{"HNSW", ledger.VectorANNHNSW, true},
		{"unknown", ledger.VectorANNBruteForce, false},
	}
	for _, tt := range tests {
		got, valid := parseVectorANNBackend(tt.raw)
		if got != tt.want || valid != tt.valid {
			t.Fatalf("parseVectorANNBackend(%q) = (%s,%v), want (%s,%v)",
				tt.raw, got, valid, tt.want, tt.valid)
		}
	}
}

func TestEnvIntFallback(t *testing.T) {
	t.Setenv("VECTOR_TEST_INT", "12")
	if got := envInt("VECTOR_TEST_INT", 3); got != 12 {
		t.Fatalf("expected configured int, got %d", got)
	}
	t.Setenv("VECTOR_TEST_INT", "-1")
	if got := envInt("VECTOR_TEST_INT", 3); got != 3 {
		t.Fatalf("expected fallback for negative int, got %d", got)
	}
	t.Setenv("VECTOR_TEST_INT", "bad")
	if got := envInt("VECTOR_TEST_INT", 3); got != 3 {
		t.Fatalf("expected fallback for bad int, got %d", got)
	}
}
