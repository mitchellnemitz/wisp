package runtime

import (
	"strings"
	"testing"
)

// Regex milestone: source-level invariant assertions. Two load-bearing
// guarantees cannot be proven by a runtime fixture (the golden harness runs in
// the C locale, which masks a missing LC_ALL=C, and the located message already
// hides awk's stderr at runtime), so they are asserted directly against the
// helper bodies:
//
//   - LC_ALL=C: matching is byte-based (acceptance 6b).
//   - 2>/dev/null: awk's own stderr never leaks; only wisp's located message
//     surfaces on a malformed pattern (acceptance 6).
func TestRegex_HelperBodies_HoldByteAndNonLeakInvariants(t *testing.T) {
	for _, id := range []string{Matches, RegexFind, RegexFindAll, RegexReplace} {
		h, ok := registry[id]
		if !ok {
			t.Fatalf("regex helper %q missing from registry", id)
		}
		if !strings.Contains(h.src, "LC_ALL=C") {
			t.Errorf("%s body missing LC_ALL=C (byte-semantics guarantee)", id)
		}
		if !strings.Contains(h.src, "2>/dev/null") {
			t.Errorf("%s body missing 2>/dev/null (awk-stderr non-leak guarantee)", id)
		}
	}
}

// regex_find_all decodes a byte-length-prefixed stream with a shell-side
// byte-at-a-time `?` peel. Under a non-C caller locale (e.g. bash in UTF-8) `?`
// peels a whole multibyte character, desyncing the shell from awk's byte-based
// length(). The golden harness runs in the C locale and cannot catch that, so the
// fix -- a function-local LC_ALL=C around the decode, like the dict key codec --
// is asserted at the source level.
func TestRegex_FindAll_DecodeLoopScopesLocalByteLocale(t *testing.T) {
	h := registry[RegexFindAll]
	if !strings.Contains(h.src, "local LC_ALL") {
		t.Errorf("%s body missing `local LC_ALL` (byte-correct shell-side decode under a non-C caller locale)", RegexFindAll)
	}
}
