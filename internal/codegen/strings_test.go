package codegen

import (
	"strings"
	"testing"
)

// Most string builtin behavioral coverage moved to internal/golden
// (str_substring_caught, str_count_empty, substring_multibyte, char_at_multibyte,
// replace_first_multibyte, strings_tail, and the per-op namespaced fixtures),
// with byte-shape coverage in core_byteidentity_test.go (TestCoreStringsByteIdentical)
// and the string byte-model tail tests. The empty-search / empty-fill top-level
// aborts and the substring/char_at fault-catch arms (which the golden harness
// cannot express) are reconstructed below with the namespaced string.* spelling;
// delegation lowers each byte-identically to the pre-removal flat call, so the
// runtime behavior is unchanged.

// strOut compiles the program (with the string namespace bound), runs it, and
// returns stdout.
func strOut(t *testing.T, body string) string {
	t.Helper()
	out, errb, code := runNS(t, "fn main() -> int {\n"+body+"\nreturn 0\n}\n", "string")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	return out
}

// TestStrSubstringFaultCaught: substring / char_at out-of-range faults are
// catchable (not fatal aborts) inside try/catch.
func TestStrSubstringFaultCaught(t *testing.T) {
	out := strOut(t, `
try {
  print(string.substring("ab", 0, 9))
  print("no")
} catch (e) {
  print("caught")
}
try {
  print(string.char_at("ab", 5))
  print("no")
} catch (e) {
  print("caught2")
}`)
	if out != "caught\ncaught2\n" {
		t.Errorf("substring/char_at faults: got %q", out)
	}
}

// TestStrEmptyNeedleFaultsTopLevel: empty-search (count / replace_first) and
// empty-fill (pad_start / pad_end) abort at the top level with a located message.
func TestStrEmptyNeedleFaultsTopLevel(t *testing.T) {
	for _, c := range []struct{ body, msg string }{
		{`print(to_string(string.count("ab", "")))`, "count(): empty search string"},
		{`print(string.replace_first("ab", "", "x"))`, "replace_first(): empty search string"},
		{`print(string.pad_start("a", 5, ""))`, "pad_start(): empty fill"},
		{`print(string.pad_end("a", 5, ""))`, "pad_end(): empty fill"},
	} {
		_, errb, code := runNS(t, "fn main() -> int {\n"+c.body+"\nreturn 0\n}\n", "string")
		if code == 0 {
			t.Errorf("%q should abort", c.body)
		}
		if !strings.Contains(errb, c.msg) {
			t.Errorf("%q stderr = %q, want %q", c.body, errb, c.msg)
		}
	}
}
