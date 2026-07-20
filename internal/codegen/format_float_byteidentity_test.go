package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFormatFloat_HelperShape pins the __wisp_format_float helper body shape.
// Reconstructed with the namespaced call string.format_float; the delegate emits
// the same __wisp_format_float helper the pre-removal bare call did.
func TestFormatFloat_HelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let s: string = string.format_float(3.14, 2)
  return 0
}`, "string"))
	for _, want := range []string{
		"__wisp_format_float() {",
		`awk -v x="$2" -v d="$3"`,  // x/decimals via -v (injection-safe, AC5)
		`printf "%." d "f", (x+0)`, // the pinned lowering (AC4: no custom rounding; NOT %.*f -- busybox rejects star-precision)
		`__wisp_ffinite "$1"`,      // reused non-finite guard
		`format_float: decimals must be >= 0`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
	// AC5: the awk PROGRAM text must be a single-quoted constant -- x/decimals reach awk
	// ONLY via -v. Belt-and-suspenders negative check, SCOPED to the
	// __wisp_format_float helper body (NOT the whole program -- another helper may
	// legitimately contain `($2`): the body must not interpolate a shell positional
	// into the awk program (`($2`/`($3`).
	body := sh[strings.Index(sh, "__wisp_format_float() {"):]
	if end := strings.Index(body, "\n}"); end >= 0 {
		body = body[:end]
	}
	if strings.Contains(body, `($2`) || strings.Contains(body, `($3`) {
		t.Errorf("format_float must pass x/decimals via -v, not interpolate them into the awk program")
	}
}

// TestFormatFloat_NoUse_ByteIdentical: a program NOT calling format_float emits shell
// byte-identical to before this feature (AC6). The no-use program is spelled with
// math.sqrt (byte-identical delegate lowering), so the pre-removal snapshot still
// matches. Regenerate with:
//
//	UPDATE_FORMAT_FLOAT_SNAPSHOT=1 go test ./internal/codegen -run TestFormatFloat_NoUse_ByteIdentical
func TestFormatFloat_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let r: float = math.sqrt(2.0)
  print(to_string(r))
  return 0
}`
	got := compileNS(t, src, "math")
	snap := filepath.Join("testdata", "format_float_byteidentity.sh")
	if os.Getenv("UPDATE_FORMAT_FLOAT_SNAPSHOT") == "1" {
		if err := os.WriteFile(snap, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote snapshot %s (%d bytes)", snap, len(got))
		return
	}
	want, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("read snapshot %s: %v", snap, err)
	}
	if string(got) != string(want) {
		t.Errorf("emitted shell drifted; re-mint with UPDATE_FORMAT_FLOAT_SNAPSHOT=1 if intentional")
	}
	if strings.Contains(string(got), "__wisp_format_float") {
		t.Errorf("__wisp_format_float leaked into a program that does not call format_float")
	}
}
