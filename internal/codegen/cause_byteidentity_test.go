package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCauseEmittedShellByteIdentical is the byte-identity GATE for the error
// cause-chain work (T2). The four throw-path edits (genThrow/bindCatchVar/
// __wisp_fail/genTry) sit in the SHARED error path, so they are gated on a
// program-wide uses-`wrap` predicate. This test pins the emitted .sh of THREE
// programs that use NEITHER `wrap` NOR `cause` -- (1) a no-error-handling
// program, (2) a try/throw/catch program, and (3) a faulting program (caught
// `1 / 0`, which emits __wisp_fail) -- originally against snapshots captured by
// compiling the same three sources with the compiler at the merge-base b26f2e4
// (the pre-feature parent). The gate proved the throw-path edits caused no drift
// on non-`wrap` programs.
//
// Snapshot re-minted ON-BRANCH for the INT_MIN arith-form fix (bare variable refs
// inside $(( )): $(( $x )) -> $(( x ))). The faulting case's `1 / 0` path emits an
// idiv arith expression, so that fix intentionally flips its operands dollar->bare
// and the merge-base anchor no longer holds. The only permitted delta from the
// merge-base snapshot is that dollar->bare flip on arith operands; nothing else.
//
// Regenerate ON-BRANCH:
// UPDATE_CAUSE_SNAPSHOT=1 go test ./internal/codegen -run TestCauseEmittedShellByteIdentical
func TestCauseEmittedShellByteIdentical(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "noeh",
			src: `fn main() -> int {
  let a: int = 1
  let b: int = 2
  print("${a}")
  print("${b}")
  return 0
}`,
		},
		{
			name: "trycatch",
			src: `fn main() -> int {
  try {
    throw error("boom")
  } catch (e) {
    print(e.message)
  }
  return 0
}`,
		},
		{
			name: "fault",
			src: `fn main() -> int {
  let a: int = 10
  let b: int = 0
  try {
 print(to_string(a / b))
  } catch (e) {
    print(e.message)
  }
  return 0
}`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := compile(t, tc.src)
			snap := filepath.Join("testdata", "cause_byteid_"+tc.name+".sh")
			if os.Getenv("UPDATE_CAUSE_SNAPSHOT") == "1" {
				if err := os.MkdirAll(filepath.Dir(snap), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(snap, got, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote snapshot %s (%d bytes)", snap, len(got))
				return
			}
			want, err := os.ReadFile(snap)
			if err != nil {
				t.Fatalf("read snapshot %s: %v (re-mint with UPDATE_CAUSE_SNAPSHOT=1)", snap, err)
			}
			if string(got) != string(want) {
				t.Fatalf("emitted .sh changed (byte-identity gate failed) for %s.\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
					tc.name, len(got), got, len(want), want)
			}
		})
	}
}
