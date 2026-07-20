package codegen

import (
	"strings"
	"testing"
)

// TestSubstring_ByteModelPreludeShape asserts that the emitted __wisp_substring
// body uses the LC_ALL=C awk + ENVIRON[] + trailing-x-sentinel byte model, and
// does NOT contain the old codepoint-based ${#2} length or ${__sb_rest#?} loop.
// Reconstructed with the namespaced string.substring call.
func TestSubstring_ByteModelPreludeShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let s: string = "hello"
  print(string.substring(s, 0, 3))
  return 0
}`, "string"))

	for _, want := range []string{
		"LC_ALL=C awk",
		`ENVIRON["`,
		`printf "%sx"`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("__wisp_substring: missing %q in emitted shell", want)
		}
	}

	for _, forbidden := range []string{
		"${#2}",
		"${__sb_rest#?}",
	} {
		if strings.Contains(sh, forbidden) {
			t.Errorf("__wisp_substring: found forbidden codepoint pattern %q in emitted shell", forbidden)
		}
	}
}

// TestCharAt_ByteModelPreludeShape asserts that the emitted __wisp_char_at body
// uses the LC_ALL=C awk + ENVIRON[] byte model and does NOT contain the old
// codepoint-based ${#2} length or ${__ca_rest#?} loop. Reconstructed with the
// namespaced string.char_at call.
func TestCharAt_ByteModelPreludeShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let s: string = "hello"
  print(string.char_at(s, 1))
  return 0
}`, "string"))

	for _, want := range []string{
		"LC_ALL=C awk",
		`ENVIRON["`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("__wisp_char_at: missing %q in emitted shell", want)
		}
	}

	for _, forbidden := range []string{
		"${#2}",
		"${__ca_rest#?}",
	} {
		if strings.Contains(sh, forbidden) {
			t.Errorf("__wisp_char_at: found forbidden codepoint pattern %q in emitted shell", forbidden)
		}
	}
}

// TestSubstringCharAt_DepsUnchanged asserts that neither helper gained a Length
// dependency. A program using only substring must NOT emit __wisp_length.
func TestSubstringCharAt_DepsUnchanged(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let s: string = "hello"
  print(string.substring(s, 0, 3))
  return 0
}`, "string"))

	if strings.Contains(sh, "__wisp_length") {
		t.Errorf("substring pulled in __wisp_length -- deps must stay {Fail} only")
	}
}
