package runtime

import (
	"strings"
	"testing"
)

// The string measurement/search/pad family is documented as byte-oriented and
// shell-independent. `${#var}` and `${var#?}` measure and step BYTES in dash but
// CODEPOINTS in bash/zsh under a UTF-8 locale, and busybox ash counts codepoints
// regardless of a runtime LC_ALL assignment (it is decided at startup and a
// shell-variable LC_ALL=C does not change it). The golden harness runs in the C
// locale, which masks that divergence, so the byte guarantee is asserted at the
// source level: each helper must do its measurement/stepping inside an
// `LC_ALL=C awk` (an external process, the only primitive that is byte-oriented
// on all four shells -- the reverse_string model), not with a shell `${#}` /
// `${#?}` expansion.
func TestStringBytes_HelperBodies_AreByteOrientedViaAwk(t *testing.T) {
	for _, id := range []string{Length, IndexOf, LastIndexOf, Count, PadStart, PadEnd} {
		h, ok := registry[id]
		if !ok {
			t.Fatalf("string helper %q missing from registry", id)
		}
		if !strings.Contains(h.src, "LC_ALL=C awk") {
			t.Errorf("%s body missing `LC_ALL=C awk` (byte-semantics guarantee on bash/zsh/busybox)", id)
		}
		// A surviving shell length/step would reintroduce the codepoint divergence.
		// The measurement must not use ${#...}; the stepping must not use ${...#?}.
		if strings.Contains(h.src, "${#") {
			t.Errorf("%s body uses a shell ${#...} length (codepoint-oriented under bash/zsh/busybox UTF-8)", id)
		}
		if strings.Contains(h.src, "#?}") {
			t.Errorf("%s body uses a shell ${var#?} byte-step (codepoint-oriented under bash/zsh/busybox UTF-8)", id)
		}
	}
}
