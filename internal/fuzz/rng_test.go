// internal/fuzz/rng_test.go
package fuzz

import "testing"

func TestRNGDeterministic(t *testing.T) {
	a, b := NewRNG(42), NewRNG(42)
	for i := 0; i < 1000; i++ {
		if a.Intn(97) != b.Intn(97) {
			t.Fatalf("same seed diverged at i=%d", i)
		}
	}
}

func TestRNGDifferentSeeds(t *testing.T) {
	a, b := NewRNG(1), NewRNG(2)
	same := true
	for i := 0; i < 50; i++ {
		if a.Int64() != b.Int64() {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical sequences")
	}
}

func TestRNGIntnBounds(t *testing.T) {
	r := NewRNG(7)
	for i := 0; i < 1000; i++ {
		if v := r.Intn(5); v < 0 || v >= 5 {
			t.Fatalf("Intn(5)=%d out of range", v)
		}
	}
}
