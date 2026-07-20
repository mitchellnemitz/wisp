package codegen

// AC6 corroboration: a program using in-domain constant arguments at every
// domain-checked site emits shell byte-identical to the merge-base. AC8: out-of-
// scope cases are NOT newly compile-rejected and still abort at runtime.
//
// The chr / repeat sites are now the namespaced string.chr / string.repeat
// (their bare spelling was removed). Because the delegate lowering is
// byte-identical to the pre-removal flat call, the namespaced source compiled
// through compileNS emits exactly the shell the flat source emitted, so the
// merge-base snapshot (testdata/arg_domain_byteidentity.sh) still matches. If a
// front-end-only change intentionally alters emission, re-mint with:
//
//	UPDATE_ARG_DOMAIN_SNAPSHOT=1 go test ./internal/codegen -run TestArgDomain_ByteIdentical

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

func TestArgDomain_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let s: string = string.chr(65)
  let r: string = string.repeat("x", 3)
  let a: int[] = [1, 2, 3]
  let y: int = a[1]
  let q: int = 7 / 2
  print(s)
  print(r)
  print(to_string(y))
  print(to_string(q))
  return 0
}`
	got := compileNS(t, src, "string")
	snap := filepath.Join("testdata", "arg_domain_byteidentity.sh")
	if os.Getenv("UPDATE_ARG_DOMAIN_SNAPSHOT") == "1" {
		if err := os.WriteFile(snap, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote snapshot %s (%d bytes)", snap, len(got))
		return
	}
	want, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("read snapshot %s: %v (mint at the merge-base with UPDATE_ARG_DOMAIN_SNAPSHOT=1)", snap, err)
	}
	if string(got) != string(want) {
		t.Errorf("emitted shell drifted; the change must be front-end-only (AC6). The namespaced string.chr/string.repeat delegate lowering must stay byte-identical to the flat merge-base. Re-mint only if intentional.")
	}
}

// adRejects reports whether the checker rejects src (any error).
func adRejects(src string) bool {
	prog, err := parser.Parse(src, "ac8.wisp")
	if err != nil {
		return true
	}
	return len(types.Check(prog).Errors) != 0
}

func TestArgDomain_NoScopeCreep(t *testing.T) {
	// AC8: a constant NON-negative array index exceeding a known-short array still
	// compiles (the dynamic upper bound is out of scope).
	if adRejects(`fn main() -> int {
  let a: int[] = [1, 2, 3]
  let x: int = a[99]
  print(to_string(x))
  return 0
}`) {
		t.Errorf("constant non-negative out-of-length array index must still compile (AC8)")
	}
	// AC8: a deferred float-value domain case still compiles (math.sqrt(-1.0)
	// aborts at runtime, not at compile time). Checked in a linked module set
	// since sqrt's bare spelling was removed.
	if compileRejectsNS(`fn main() -> int {
  let r: float = math.sqrt(-1.0)
  print(to_string(r))
  return 0
}`, []string{"math"}) {
		t.Errorf("math.sqrt(-1.0) is a deferred float-value domain; it must still compile (AC8)")
	}
}
