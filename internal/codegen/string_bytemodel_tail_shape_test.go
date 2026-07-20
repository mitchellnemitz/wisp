package codegen

import (
	"strings"
	"testing"
)

// TestByteModelTail_Shape asserts that all four rewritten helpers
// (contains, ends_with, replace, replace_first) emit the LC_ALL=C awk +
// ENVIRON[] byte model and do NOT contain any of the old codepoint-scan
// patterns. Negative assertions are scoped to each helper's body slice so
// that legitimate gsub( usage in regex_replace elsewhere does not
// false-match. Reconstructed with namespaced string.* calls; the delegate emits
// the same __wisp_* helpers the pre-removal bare calls did.
func TestByteModelTail_Shape(t *testing.T) {
	// A single program that triggers all four helpers.
	src := `fn main() -> int {
  let s: string = "hello world"
  let c: bool   = string.contains(s, "world")
  let e: bool   = string.ends_with(s, "world")
  let r: string = string.replace(s, "l", "L")
  let f: string = string.replace_first(s, "l", "L")
  print("${c}")
  print("${e}")
  print(r)
  print(f)
  return 0
}`
	sh := string(compileNS(t, src, "string"))

	// Helper: extract the body of a named shell function from the emitted output.
	// Returns the slice starting at the function header and ending at the closing "}" line.
	body := func(name string) string {
		start := strings.Index(sh, name+"()")
		if start < 0 {
			t.Fatalf("helper %s not found in emitted shell", name)
		}
		end := strings.Index(sh[start:], "\n}")
		if end < 0 {
			return sh[start:]
		}
		return sh[start : start+end+2]
	}

	cnBody := body("__wisp_contains")
	ewBody := body("__wisp_ends_with")
	rBody := body("__wisp_replace")
	rfBody := body("__wisp_replace_first")

	// ---- POSITIVE: each body must use the LC_ALL=C awk byte model ----

	for _, tc := range []struct {
		name string
		b    string
	}{
		{"contains", cnBody},
		{"ends_with", ewBody},
		{"replace", rBody},
		{"replace_first", rfBody},
	} {
		for _, want := range []string{"LC_ALL=C awk", `ENVIRON[`} {
			if !strings.Contains(tc.b, want) {
				t.Errorf("__wisp_%s: missing %q in helper body", tc.name, want)
			}
		}
	}

	// ---- NEGATIVE: old codepoint-scan patterns must be gone ----

	// contains: old scan was ${__cn_rest#?}
	if strings.Contains(cnBody, "${__cn_rest#?}") {
		t.Errorf("__wisp_contains: still contains old codepoint scan ${__cn_rest#?}")
	}

	// ends_with: old pattern was ${1%%"$2"} / ${__ew_after or ${1%"$2"}
	for _, old := range []string{`${1%"$2"}`, `${__ew_after`} {
		if strings.Contains(ewBody, old) {
			t.Errorf("__wisp_ends_with: still contains old codepoint pattern %q", old)
		}
	}

	// replace: old scan was ${__r_rest#?}
	if strings.Contains(rBody, "${__r_rest#?}") {
		t.Errorf("__wisp_replace: still contains old codepoint scan ${__r_rest#?}")
	}
	// replace must use index()+substr() loop, NOT awk gsub
	if strings.Contains(rBody, "gsub(") {
		t.Errorf("__wisp_replace: must not use awk gsub( (literal replace only)")
	}

	// replace_first: old scan was ${__rf_rest#?}
	if strings.Contains(rfBody, "${__rf_rest#?}") {
		t.Errorf("__wisp_replace_first: still contains old codepoint scan ${__rf_rest#?}")
	}
	// replace_first must use index()+substr(), NOT awk sub/gsub
	if strings.Contains(rfBody, "gsub(") {
		t.Errorf("__wisp_replace_first: must not use awk gsub( (literal replace only)")
	}

	// ---- AC8: dep discipline ----
	// contains and ends_with must NOT emit __wisp_fail (no Fail dep).
	if strings.Contains(cnBody, "__wisp_fail") {
		t.Errorf("__wisp_contains: must not call __wisp_fail (empty deps required)")
	}
	if strings.Contains(ewBody, "__wisp_fail") {
		t.Errorf("__wisp_ends_with: must not call __wisp_fail (empty deps required)")
	}
	// replace and replace_first MUST emit __wisp_fail (Fail dep).
	if !strings.Contains(rBody, "__wisp_fail") {
		t.Errorf("__wisp_replace: must call __wisp_fail (Fail dep required)")
	}
	if !strings.Contains(rfBody, "__wisp_fail") {
		t.Errorf("__wisp_replace_first: must call __wisp_fail (Fail dep required)")
	}
}
