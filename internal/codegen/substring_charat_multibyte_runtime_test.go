package codegen

// TestSubstringCharAt_MultibyteByteModel is the BINDING multibyte regression
// gate for the byte-model substring/char_at helpers (Task 1 fix). The golden
// harness inherits the ambient locale (no cmd.Env locale override), which CI
// does not guarantee to be UTF-8; under a C locale the pre-fix codepoint bug is
// invisible. This test explicitly sets LC_ALL to a UTF-8 locale so the bug
// (bash/zsh ${#} counting codepoints instead of bytes) is exposed across all
// four shells.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// pickUTF8Locale scans `locale -a` for a UTF-8 locale and returns its EXACT
// system spelling (the value setlocale accepts). Different libcs spell the same
// locale differently -- glibc prints `C.utf8` / `en_US.utf8`, macOS prints
// `en_US.UTF-8` -- so matching is done on a normalized form (lowercased, hyphens
// stripped) and the original spelling is what gets returned. Preference order:
// a C-family UTF-8 locale (locale-independent, always present on glibc), then an
// en_US UTF-8 locale, then any other UTF-8 locale. On Linux, absence is a fatal
// error (a UTF-8 locale is effectively always available -- glibc ships C.utf8
// built in; absence means a broken environment that would silently mask the
// codepoint bug). On other OSes it is a skip.
func pickUTF8Locale(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("locale", "-a").Output()
	if err != nil {
		t.Fatalf("locale -a failed: %v", err)
	}
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", "")
	}
	available := strings.Split(strings.TrimRight(string(out), "\n"), "\n")

	// First pass: preferred C-family, then en_US, by normalized spelling.
	for _, want := range []string{"c.utf8", "en_us.utf8"} {
		for _, l := range available {
			if norm(l) == want {
				return strings.TrimSpace(l)
			}
		}
	}
	// Second pass: any UTF-8 locale at all.
	for _, l := range available {
		if strings.HasSuffix(norm(l), ".utf8") {
			return strings.TrimSpace(l)
		}
	}
	if runtime.GOOS == "linux" {
		t.Fatalf("no UTF-8 locale found in `locale -a`: a UTF-8 locale (e.g. glibc's built-in C.utf8) is effectively always present on Linux; this indicates a broken environment")
	}
	t.Skip("no UTF-8 locale available (non-Linux host with no UTF-8 locale)")
	return ""
}

// runMultibyte compiles src, writes it to a temp file, and runs it under sh
// with LC_ALL=utf8locale set. Returns stdout bytes, stderr string, exit code.
func runMultibyte(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, src, utf8locale string) ([]byte, string, int) {
	t.Helper()
	script := compileNS(t, src, "string")
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{}, sh.args...), path)
	cmd := exec.Command(sh.bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL="+utf8locale)
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

// locatedAbortReMultibyte mirrors locatedAbortRe from located_abort_test.go
// (same package; reproduced here for clarity in the test output).
var locatedAbortReMultibyte = regexp.MustCompile(`^wisp: ([^:]+):(\d+):(\d+): (.*)$`)

// TestSubstringCharAt_MultibyteByteModel is reconstructed for the modules-only
// surface: substring / char_at are spelled through the string namespace, while
// length stays flat. Delegation lowers string.substring / string.char_at
// byte-identically to the pre-removal flat call, so the forced-UTF-8-locale
// multibyte byte model and the OOB located aborts are exercised exactly as before.
func TestSubstringCharAt_MultibyteByteModel(t *testing.T) {
	utf8locale := pickUTF8Locale(t)
	t.Logf("using UTF-8 locale: %s", utf8locale)

	// "café" in UTF-8: c=0x63 a=0x61 f=0x66 é=0xC3 0xA9 -- 5 bytes, 4 codepoints.
	// Pre-fix, bash/zsh under a UTF-8 locale counted codepoints for length and
	// slicing, so substring("café", 0, length("café")) would compute length=4 and
	// abort (range out of bounds when slicing 5-byte string with end=4 via
	// codepoint logic). The byte model fix makes all four operations correct.

	for _, sh := range execShells(t) {
		sh := sh
		t.Run(sh.label, func(t *testing.T) {

			// --- substring: full slice (the pre-fix abort case on bash/zsh) ---
			{
				src := `fn main() -> int {
  print(string.substring("café", 0, length("café")))
  return 0
}`
				out, errb, code := runMultibyte(t, sh, src, utf8locale)
				if code != 0 {
					t.Fatalf("substring full-slice: exit=%d stderr=%q", code, errb)
				}
				// "café" + newline: 5 UTF-8 bytes + '\n'
				want := "caf\xC3\xA9\n"
				if string(out) != want {
					t.Errorf("substring full-slice: stdout=%q, want %q", out, want)
				}
			}

			// --- substring: caf (bytes 0..3) ---
			{
				src := `fn main() -> int {
  print(string.substring("café", 0, 3))
  return 0
}`
				out, errb, code := runMultibyte(t, sh, src, utf8locale)
				if code != 0 {
					t.Fatalf("substring [0,3): exit=%d stderr=%q", code, errb)
				}
				if string(out) != "caf\n" {
					t.Errorf("substring [0,3): stdout=%q, want %q", out, "caf\n")
				}
			}

			// --- substring: empty slice (trailing-x sentinel) ---
			{
				src := `fn main() -> int {
  print(string.substring("café", 3, 3))
  return 0
}`
				out, errb, code := runMultibyte(t, sh, src, utf8locale)
				if code != 0 {
					t.Fatalf("substring [3,3) empty: exit=%d stderr=%q", code, errb)
				}
				if string(out) != "\n" {
					t.Errorf("substring [3,3) empty: stdout=%q, want bare newline", out)
				}
			}

			// --- char_at: byte at offset 3 -> 0xC3 (raw byte comparison) ---
			{
				src := `fn main() -> int {
  print(string.char_at("café", 3))
  return 0
}`
				out, errb, code := runMultibyte(t, sh, src, utf8locale)
				if code != 0 {
					t.Fatalf("char_at(3): exit=%d stderr=%q", code, errb)
				}
				// print writes the raw byte followed by a newline.
				want := []byte{0xC3, '\n'}
				if !bytes.Equal(out, want) {
					t.Errorf("char_at(3): stdout=%v, want %v (raw bytes)", out, want)
				}
			}

			// --- char_at: byte at offset 4 -> 0xA9 (raw byte comparison) ---
			{
				src := `fn main() -> int {
  print(string.char_at("café", 4))
  return 0
}`
				out, errb, code := runMultibyte(t, sh, src, utf8locale)
				if code != 0 {
					t.Fatalf("char_at(4): exit=%d stderr=%q", code, errb)
				}
				want := []byte{0xA9, '\n'}
				if !bytes.Equal(out, want) {
					t.Errorf("char_at(4): stdout=%v, want %v (raw bytes)", out, want)
				}
			}

			// --- located abort: char_at out of bounds ---
			{
				src := `fn main() -> int {
  print(string.char_at("café", 5))
  return 0
}`
				_, errb, code := runMultibyte(t, sh, src, utf8locale)
				if code == 0 {
					t.Fatalf("char_at out-of-bounds: expected non-zero exit, got 0")
				}
				first := strings.SplitN(strings.TrimRight(errb, "\n"), "\n", 2)[0]
				m := locatedAbortReMultibyte.FindStringSubmatch(first)
				if m == nil {
					t.Fatalf("char_at OOB stderr first line %q does not match `wisp: file:line:col: msg`", first)
				}
				msg := m[4]
				if !strings.Contains(msg, "index out of bounds") {
					t.Errorf("char_at OOB message %q must contain %q", msg, "index out of bounds")
				}
			}

			// --- located abort: substring range out of bounds ---
			{
				src := `fn main() -> int {
  print(string.substring("café", 0, 6))
  return 0
}`
				_, errb, code := runMultibyte(t, sh, src, utf8locale)
				if code == 0 {
					t.Fatalf("substring OOB: expected non-zero exit, got 0")
				}
				first := strings.SplitN(strings.TrimRight(errb, "\n"), "\n", 2)[0]
				m := locatedAbortReMultibyte.FindStringSubmatch(first)
				if m == nil {
					t.Fatalf("substring OOB stderr first line %q does not match `wisp: file:line:col: msg`", first)
				}
				msg := m[4]
				if !strings.Contains(msg, "range out of bounds") {
					t.Errorf("substring OOB message %q must contain %q", msg, "range out of bounds")
				}
			}
		})
	}
}
