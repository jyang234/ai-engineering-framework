//go:build fts5

package eval_test

import (
	"math"
	"testing"

	"github.com/anthropics/aef/codex/eval"
)

func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestRecallAtK(t *testing.T) {
	relevant := []string{"a", "b", "c", "d"}

	tests := []struct {
		name      string
		retrieved []string
		k         int
		want      float64
	}{
		{"perfect", []string{"a", "b", "c", "d"}, 4, 1.0},
		{"half", []string{"a", "b", "x", "y"}, 4, 0.5},
		{"none", []string{"x", "y", "z"}, 3, 0.0},
		{"top2 of 4", []string{"a", "b"}, 2, 0.5},
		{"k limits", []string{"a", "b", "c", "d", "e"}, 2, 0.5},
		{"empty retrieved", []string{}, 5, 0.0},
		{"zero k", []string{"a"}, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.RecallAtK(tt.retrieved, relevant, tt.k)
			if !approxEqual(got, tt.want, 0.001) {
				t.Errorf("RecallAtK = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestRecallAtK_EmptyRelevant(t *testing.T) {
	got := eval.RecallAtK([]string{"a"}, nil, 5)
	if got != 0 {
		t.Errorf("expected 0 for empty relevant, got %f", got)
	}
}

func TestPrecisionAtK(t *testing.T) {
	relevant := []string{"a", "b", "c"}

	tests := []struct {
		name      string
		retrieved []string
		k         int
		want      float64
	}{
		{"all relevant", []string{"a", "b", "c"}, 3, 1.0},
		{"half", []string{"a", "x", "b", "y"}, 4, 0.5},
		{"none", []string{"x", "y"}, 2, 0.0},
		{"k=1 hit", []string{"a", "x"}, 1, 1.0},
		{"k=1 miss", []string{"x", "a"}, 1, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.PrecisionAtK(tt.retrieved, relevant, tt.k)
			if !approxEqual(got, tt.want, 0.001) {
				t.Errorf("PrecisionAtK = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNDCG(t *testing.T) {
	relevant := []string{"a", "b", "c"} // a=3, b=2, c=1

	tests := []struct {
		name      string
		retrieved []string
		k         int
	}{
		{"perfect order", []string{"a", "b", "c"}, 3},
		{"reversed", []string{"c", "b", "a"}, 3},
		{"partial", []string{"a", "x", "b"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.NDCG(tt.retrieved, relevant, tt.k)
			if got < 0 || got > 1.0+0.001 {
				t.Errorf("NDCG out of range: %f", got)
			}
		})
	}

	// Perfect order should yield 1.0
	perfect := eval.NDCG([]string{"a", "b", "c"}, relevant, 3)
	if !approxEqual(perfect, 1.0, 0.001) {
		t.Errorf("perfect NDCG = %f, want 1.0", perfect)
	}

	// Reversed should be less than perfect
	reversed := eval.NDCG([]string{"c", "b", "a"}, relevant, 3)
	if reversed >= 1.0 {
		t.Errorf("reversed NDCG should be < 1.0, got %f", reversed)
	}
}

func TestMRR(t *testing.T) {
	relevant := []string{"a", "b"}

	tests := []struct {
		name      string
		retrieved []string
		want      float64
	}{
		{"first position", []string{"a", "x", "y"}, 1.0},
		{"second position", []string{"x", "b", "y"}, 0.5},
		{"third position", []string{"x", "y", "a"}, 1.0 / 3},
		{"not found", []string{"x", "y", "z"}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.MRR(tt.retrieved, relevant)
			if !approxEqual(got, tt.want, 0.001) {
				t.Errorf("MRR = %f, want %f", got, tt.want)
			}
		})
	}
}
