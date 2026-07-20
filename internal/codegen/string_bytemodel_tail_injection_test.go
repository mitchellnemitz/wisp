package codegen

// TestByteModelTailInjection is AC7 for the four byte-model string helpers:
// contains, replace, replace_first, ends_with. Each hostile vector is pushed
// through every data-bearing argument of every op; the test asserts:
//   (a) no sentinel file is created (no shell evaluation of the hostile bytes)
//   (b) the hostile data round-trips inert in stdout where the op returns the
//       subject or replacement (contains/ends_with return a boolean so only
//       the round-trip form is tested; replace/replace_first pass data through
//       to stdout).
//
// Reconstructed for the modules-only surface: the four ops are spelled through
// the string namespace (string.contains / string.ends_with / string.replace /
// string.replace_first). Delegation lowers each byte-identically to the
// pre-removal flat call, so the same __wisp_* helpers are exercised and the
// hostile-vector guarantees are identical to the pre-removal matrix.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hostileVectors is the set of injection payloads mandated by AC7.
// Each vector is named for clarity in sub-test output.
// For command-execution vectors the payloadFmt field is non-empty: it is a
// fmt.Sprintf format string taking exactly one argument (the sentinel path).
// The actual payload is built per-subtest so the sentinel path embedded in the
// shell command matches exactly what os.Stat checks -- making the detector
// non-vacuous. For non-command vectors payloadFmt is empty and val is used
// directly.
var hostileVectors = []struct {
	name       string
	val        string // used directly when payloadFmt == ""
	payloadFmt string // non-empty for command-exec vectors; %s = sentinel path
}{
	{"cmd_sub_dollar", "", "$(touch %s)"},
	{"cmd_sub_backtick", "", "`touch %s`"},
	{"semicolon", "", "; touch %s"},
	{"glob_star", "*", ""},
	{"newline", "hello\nworld", ""},
	{"leading_dash", "-n", ""},
	{"double_quote", `say "hello"`, ""},
}

// sentinelForTest returns the unique sentinel path for the given test name and
// vector name. Uses t.TempDir so the sentinel lives in an isolated directory;
// the file must NOT exist after a safe run.
func sentinelForTest(t *testing.T, vector string) string {
	t.Helper()
	// Use a fixed name inside the per-test temp dir so there is no collision.
	return filepath.Join(t.TempDir(), fmt.Sprintf("PWNED_%s", strings.ReplaceAll(vector, " ", "_")))
}

// runBMInj compiles src (with the string namespace bound), runs it under sh with
// no locale override, and returns stdout (string), stderr (string), and exit
// code. Mirrors runBM but accepts a pre-built sentinel env var so the injected
// path is unique.
func runBMInj(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, src string) (string, string, int) {
	t.Helper()
	script := compileNS(t, src, "string")
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{}, sh.args...), path)
	cmd := exec.Command(sh.bin, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%s: run: %v", sh.label, err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// quoteForWisp wraps a Go string in a wisp double-quoted string literal,
// escaping backslashes and double quotes so the result embeds correctly into
// a wisp source snippet. Newlines are passed as literal bytes (the compiler
// treats wisp string literals as raw bytes).
func quoteForWisp(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteByte(s[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}

func TestByteModelTailInjection(t *testing.T) {
	for _, sh := range execShells(t) {
		sh := sh
		t.Run(sh.label, func(t *testing.T) {
			for _, vec := range hostileVectors {
				vec := vec
				t.Run(vec.name, func(t *testing.T) {
					sentinel := sentinelForTest(t, vec.name)

					// Determine the actual payload for this subtest.
					// Command-exec vectors embed the sentinel path so that if a
					// shell evaluates the payload it creates exactly the file that
					// os.Stat checks -- making the no-execution assertion non-vacuous.
					payload := vec.val
					if vec.payloadFmt != "" {
						payload = fmt.Sprintf(vec.payloadFmt, sentinel)
					}
					qv := quoteForWisp(payload)

					// --- contains: push hostile data through s and search ---
					// contains(hostile, hostile) -> true; hostile is not executed.
					{
						src := fmt.Sprintf(`fn main() -> int {
  let v: string = %s
  print(to_string(string.contains(v, v)))
  return 0
}`, qv)
						out, _, code := runBMInj(t, sh, src)
						if code != 0 {
							t.Errorf("contains(%s): exit=%d", vec.name, code)
						}
						if strings.TrimRight(out, "\n") != "true" {
							t.Errorf("contains(%s): stdout=%q, want true", vec.name, out)
						}
						if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
							t.Errorf("contains(%s): sentinel %s exists -- injection executed", vec.name, sentinel)
						}
					}

					// --- ends_with: push hostile data through s and suf ---
					// ends_with(hostile, hostile) -> true.
					{
						src := fmt.Sprintf(`fn main() -> int {
  let v: string = %s
  print(to_string(string.ends_with(v, v)))
  return 0
}`, qv)
						out, _, code := runBMInj(t, sh, src)
						if code != 0 {
							t.Errorf("ends_with(%s): exit=%d", vec.name, code)
						}
						if strings.TrimRight(out, "\n") != "true" {
							t.Errorf("ends_with(%s): stdout=%q, want true", vec.name, out)
						}
						if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
							t.Errorf("ends_with(%s): sentinel %s exists -- injection executed", vec.name, sentinel)
						}
					}

					// --- replace: push hostile data through s, search, and with ---
					// replace(hostile, hostile, hostile) -> hostile (the replacement
					// replaces every match; since search == s, the whole string is
					// one match, so output == the replacement == hostile).
					{
						src := fmt.Sprintf(`fn main() -> int {
  let v: string = %s
  print(string.replace(v, v, v))
  return 0
}`, qv)
						out, _, code := runBMInj(t, sh, src)
						if code != 0 {
							t.Errorf("replace(%s): exit=%d", vec.name, code)
						}
						// print() appends exactly one trailing newline; strip it and
						// compare exactly. Works for the newline vector too:
						// "hello\nworld\n" -> "hello\nworld" == payload.
						got := strings.TrimSuffix(out, "\n")
						if got != payload {
							t.Errorf("replace(%s) round-trip: got=%q, want=%q", vec.name, got, payload)
						}
						if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
							t.Errorf("replace(%s): sentinel %s exists -- injection executed", vec.name, sentinel)
						}
					}

					// --- replace_first: same hostile data through s, search, with ---
					{
						src := fmt.Sprintf(`fn main() -> int {
  let v: string = %s
  print(string.replace_first(v, v, v))
  return 0
}`, qv)
						out, _, code := runBMInj(t, sh, src)
						if code != 0 {
							t.Errorf("replace_first(%s): exit=%d", vec.name, code)
						}
						got := strings.TrimSuffix(out, "\n")
						if got != payload {
							t.Errorf("replace_first(%s) round-trip: got=%q, want=%q", vec.name, got, payload)
						}
						if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
							t.Errorf("replace_first(%s): sentinel %s exists -- injection executed", vec.name, sentinel)
						}
					}
				})
			}
		})
	}
}
