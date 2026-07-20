package token

import (
	"sort"
	"testing"
)

func TestKeywordsSortedAndComplete(t *testing.T) {
	kw := Keywords()
	if len(kw) != len(keywords) {
		t.Fatalf("Keywords() len = %d, want %d (the keyword table size)", len(kw), len(keywords))
	}
	if !sort.StringsAreSorted(kw) {
		t.Errorf("Keywords() not sorted: %v", kw)
	}
	// Every returned word must map back to a keyword, and every table entry must
	// be present -- no missing, no extra, no drift.
	got := map[string]bool{}
	for _, w := range kw {
		if _, ok := keywords[w]; !ok {
			t.Errorf("Keywords() returned %q which is not a keyword", w)
		}
		got[w] = true
	}
	for w := range keywords {
		if !got[w] {
			t.Errorf("Keywords() missing keyword %q", w)
		}
	}
	// Spot-check representative members across every category.
	for _, w := range []string{"fn", "if", "while", "switch", "true", "int", "bool", "float", "void", "error", "struct", "const", "import"} {
		if !got[w] {
			t.Errorf("Keywords() missing expected member %q", w)
		}
	}
}
