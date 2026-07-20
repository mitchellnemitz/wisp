package codegen

// TestByteModelTailRuntime is the runtime regression gate for the four
// byte-model string helpers: contains, replace, replace_first, ends_with.
// It runs compiled programs under each shell returned by execShells and asserts
// byte-exact results under both a UTF-8 locale (AC1 locale duality) and LC_ALL=C.
//
// Reconstructed for the modules-only surface: the four ops are spelled through
// the string namespace (string.*); each lowers byte-identically to the
// pre-removal flat call, so the runtime byte guarantees are unchanged.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runBM compiles src, writes it to a temp file, and runs it under sh with the
// given locale. Returns stdout bytes, stderr string, and exit code.
func runBM(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, src, locale string) ([]byte, string, int) {
	t.Helper()
	script := compileNS(t, src, "string")
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{}, sh.args...), path)
	cmd := exec.Command(sh.bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL="+locale)
	var outBuf bytes.Buffer
	var errBuf strings.Builder
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
	return outBuf.Bytes(), errBuf.String(), code
}

func TestByteModelTailRuntime(t *testing.T) {
	utf8locale := pickUTF8Locale(t)
	t.Logf("using UTF-8 locale: %s", utf8locale)

	// "cafe" in UTF-8 with accent: c=0x63 a=0x61 f=0x66 e-acute=0xC3 0xA9 (5 bytes total).
	// char_at("cafe",4) returns the single byte 0xA9; char_at("cafe",3) returns 0xC3.

	for _, sh := range execShells(t) {
		sh := sh
		t.Run(sh.label, func(t *testing.T) {

			// AC1: contains("cafe", char_at("cafe",4)) -> true
			// Must hold under both UTF-8 locale and LC_ALL=C.
			{
				src := `fn main() -> int {
  let b: string = string.char_at("caf` + "\xC3\xA9" + `", 4)
  print(to_string(string.contains("caf` + "\xC3\xA9" + `", b)))
  return 0
}`
				for _, loc := range []string{utf8locale, "C"} {
					out, errb, code := runBM(t, sh, src, loc)
					if code != 0 {
						t.Fatalf("AC1 contains (locale=%s): exit=%d stderr=%q", loc, code, errb)
					}
					if string(out) != "true\n" {
						t.Errorf("AC1 contains (locale=%s): stdout=%q, want %q", loc, out, "true\n")
					}
				}
			}

			// AC2a: replace("cafe", char_at("cafe",4), "X") -> raw bytes 63 61 66 C3 58
			// The byte 0xA9 is replaced by "X" (0x58); 0xC3 is unchanged.
			{
				src := `fn main() -> int {
  let b: string = string.char_at("caf` + "\xC3\xA9" + `", 4)
  print(string.replace("caf` + "\xC3\xA9" + `", b, "X"))
  return 0
}`
				for _, loc := range []string{utf8locale, "C"} {
					out, errb, code := runBM(t, sh, src, loc)
					if code != 0 {
						t.Fatalf("AC2a replace single (locale=%s): exit=%d stderr=%q", loc, code, errb)
					}
					want := []byte{0x63, 0x61, 0x66, 0xC3, 0x58, '\n'}
					if !bytes.Equal(out, want) {
						t.Errorf("AC2a replace single (locale=%s): stdout=%v, want %v (raw bytes)", loc, out, want)
					}
				}
			}

			// AC2b: multi-match replace and replace_first on a string built from
			// raw bytes. s = "a" + 0xA9 + "a" + 0xA9 + "a".
			// replace(s, 0xA9, "Z") -> "aZaZa"
			// replace_first(s, 0xA9, "Z") -> bytes 61 5A 61 A9 61 ("aZa\xA9a")
			{
				// We inject the raw bytes directly into the wisp source string literals.
				// The compiler treats wisp string literals as raw bytes (no escape interpretation).
				src := `fn main() -> int {
  let b: string = string.char_at("caf` + "\xC3\xA9" + `", 4)
  let s: string = "a" + b + "a" + b + "a"
  print(string.replace(s, b, "Z"))
  print(string.replace_first(s, b, "Z"))
  return 0
}`
				for _, loc := range []string{utf8locale, "C"} {
					out, errb, code := runBM(t, sh, src, loc)
					if code != 0 {
						t.Fatalf("AC2b multi-match (locale=%s): exit=%d stderr=%q", loc, code, errb)
					}
					wantReplace := []byte("aZaZa\n")
					wantFirst := []byte{0x61, 0x5A, 0x61, 0xA9, 0x61, '\n'}
					// Output is two print lines: replace result then replace_first result.
					lines := bytes.SplitN(out, []byte{'\n'}, 3)
					if len(lines) < 2 {
						t.Fatalf("AC2b multi-match (locale=%s): expected 2 lines, got %d (%v)", loc, len(lines), out)
					}
					gotReplace := append(lines[0], '\n')
					gotFirst := append(lines[1], '\n')
					if !bytes.Equal(gotReplace, wantReplace) {
						t.Errorf("AC2b replace (locale=%s): got=%v, want=%v", loc, gotReplace, wantReplace)
					}
					if !bytes.Equal(gotFirst, wantFirst) {
						t.Errorf("AC2b replace_first (locale=%s): got=%v, want=%v", loc, gotFirst, wantFirst)
					}
				}
			}

			// AC3: ends_with with byte-level suffixes.
			// ends_with("cafe", char_at("cafe",4)) -> true (single byte 0xA9)
			// ends_with("cafe", char_at("cafe",3)+char_at("cafe",4)) -> true (two-byte suffix "e-acute" = C3 A9)
			// ends_with("a", "aa") -> false
			{
				src := `fn main() -> int {
  let b4: string = string.char_at("caf` + "\xC3\xA9" + `", 4)
  let b3: string = string.char_at("caf` + "\xC3\xA9" + `", 3)
  print(to_string(string.ends_with("caf` + "\xC3\xA9" + `", b4)))
  print(to_string(string.ends_with("caf` + "\xC3\xA9" + `", b3 + b4)))
  print(to_string(string.ends_with("a", "aa")))
  return 0
}`
				for _, loc := range []string{utf8locale, "C"} {
					out, errb, code := runBM(t, sh, src, loc)
					if code != 0 {
						t.Fatalf("AC3 ends_with (locale=%s): exit=%d stderr=%q", loc, code, errb)
					}
					if string(out) != "true\ntrue\nfalse\n" {
						t.Errorf("AC3 ends_with (locale=%s): stdout=%q, want %q", loc, out, "true\ntrue\nfalse\n")
					}
				}
			}

			// AC4: literal/special-character inputs must be matched verbatim.
			// replace("a.b.c",".","-") -> "a-b-c" (dot is not a regex metachar)
			// replace("x","x","&") -> "&" (ampersand is not a backreference)
			// replace("x","x","%s%d") -> "%s%d" (format specs are not interpreted)
			// contains("a+b","+") -> true
			// ends_with("a*","*") -> true
			{
				src := `fn main() -> int {
  print(string.replace("a.b.c", ".", "-"))
  print(string.replace("x", "x", "&"))
  print(string.replace("x", "x", "%s%d"))
  print(to_string(string.contains("a+b", "+")))
  print(to_string(string.ends_with("a*", "*")))
  return 0
}`
				for _, loc := range []string{utf8locale, "C"} {
					out, errb, code := runBM(t, sh, src, loc)
					if code != 0 {
						t.Fatalf("AC4 literals (locale=%s): exit=%d stderr=%q", loc, code, errb)
					}
					want := "a-b-c\n&\n%s%d\ntrue\ntrue\n"
					if string(out) != want {
						t.Errorf("AC4 literals (locale=%s): stdout=%q, want %q", loc, out, want)
					}
				}
			}

			// AC5a: replace(s,"",r) exits non-zero and stderr first line matches
			// locatedAbortRe with message containing "empty search string".
			{
				src := `fn main() -> int {
  print(string.replace("ab", "", "x"))
  return 0
}`
				_, errb, code := runBM(t, sh, src, "C")
				if code == 0 {
					t.Fatalf("AC5a replace empty needle: expected non-zero exit, got 0")
				}
				first := strings.SplitN(strings.TrimRight(errb, "\n"), "\n", 2)[0]
				m := locatedAbortRe.FindStringSubmatch(first)
				if m == nil {
					t.Fatalf("AC5a replace empty needle: stderr first line %q does not match `wisp: file:line:col: msg`", first)
				}
				if !strings.Contains(m[4], "empty search string") {
					t.Errorf("AC5a replace empty needle: message %q must contain %q", m[4], "empty search string")
				}
			}

			// AC5b: replace_first(s,"",r) exits non-zero and stderr matches abort format.
			{
				src := `fn main() -> int {
  print(string.replace_first("ab", "", "x"))
  return 0
}`
				_, errb, code := runBM(t, sh, src, "C")
				if code == 0 {
					t.Fatalf("AC5b replace_first empty needle: expected non-zero exit, got 0")
				}
				first := strings.SplitN(strings.TrimRight(errb, "\n"), "\n", 2)[0]
				m := locatedAbortRe.FindStringSubmatch(first)
				if m == nil {
					t.Fatalf("AC5b replace_first empty needle: stderr first line %q does not match `wisp: file:line:col: msg`", first)
				}
				if !strings.Contains(m[4], "empty search string") {
					t.Errorf("AC5b replace_first empty needle: message %q must contain %q", m[4], "empty search string")
				}
			}

			// AC5c: empty needle inside try is caught (not a fatal abort).
			{
				src := `fn main() -> int {
  try {
    print(string.replace("ab", "", "x"))
    print("no")
  } catch (e) {
    print("caught")
  }
  return 0
}`
				out, errb, code := runBM(t, sh, src, "C")
				if code != 0 {
					t.Fatalf("AC5c replace empty inside try: exit=%d stderr=%q", code, errb)
				}
				if string(out) != "caught\n" {
					t.Errorf("AC5c replace empty inside try: stdout=%q, want %q", out, "caught\n")
				}
			}

			// AC6: replace("a\n","a","b") -> bytes 62 0A ("b\n"), trailing newline
			// preserved. Uses bytes.Equal.
			{
				src := `fn main() -> int {
  print(string.replace("a\n", "a", "b"))
  return 0
}`
				for _, loc := range []string{utf8locale, "C"} {
					out, errb, code := runBM(t, sh, src, loc)
					if code != 0 {
						t.Fatalf("AC6 trailing newline (locale=%s): exit=%d stderr=%q", loc, code, errb)
					}
					// print appends a newline, so result is "b\n\n" (the string "b\n" + print's own \n)
					want := []byte{0x62, 0x0A, 0x0A}
					if !bytes.Equal(out, want) {
						t.Errorf("AC6 trailing newline (locale=%s): stdout=%v, want %v (raw bytes)", loc, out, want)
					}
				}
			}
		})
	}
}
