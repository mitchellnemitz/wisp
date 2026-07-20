package types

import "testing"

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"length", "lenght", 2}, // transposition = 2 single-edits
		{"kitten", "sitting", 3},
		{"abc", "", 3},
		{"Length", "length", 1}, // case-sensitive
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSuggestUniqueWithinTwo(t *testing.T) {
	got := suggestSuffix("lenght", []string{"length", "push", "map"})
	want := `; did you mean "length"?`
	if got != want {
		t.Fatalf("suggestSuffix = %q want %q", got, want)
	}
}

func TestSuggestNoneWhenFar(t *testing.T) {
	// distance from "zzzzzz" to anything here is > 2
	if got := suggestSuffix("zzzzzz", []string{"length", "push"}); got != "" {
		t.Fatalf("expected no suggestion, got %q", got)
	}
}

func TestSuggestTieYieldsNothing(t *testing.T) {
	// "cat" is distance 1 from both "bat" and "car"; a tie at the minimum yields
	// no suggestion.
	if got := suggestSuffix("cat", []string{"bat", "car", "zzz"}); got != "" {
		t.Fatalf("expected no suggestion on a tie, got %q", got)
	}
}

func TestSuggestExactSkipsSelf(t *testing.T) {
	// An exact match (distance 0) is not a "did you mean"; the candidate pool for
	// an unknown name never contains the unknown itself, but guard anyway: a
	// single unique candidate at distance 0 is excluded (you would not suggest the
	// identical name).
	if got := suggestSuffix("length", []string{"length"}); got != "" {
		t.Fatalf("expected no suggestion for an identical candidate, got %q", got)
	}
}
