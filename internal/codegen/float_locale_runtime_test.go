package codegen

// TestFloatLocaleRuntime is the runtime regression gate for the two failure
// modes fixed by pinning LC_ALL=C on the float awk helpers: silently wrong
// float output under a comma-decimal locale, and spurious abort on float
// arithmetic under the same locale (the comma trips __wisp_ffinite's
// [0-9.]-only validation glob). It also proves float_or (a validate-and-
// canonicalize helper, not an arithmetic op) no longer falls back to its
// default value for an otherwise-valid numeric string under the same locale.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pickCommaDecimalLocale returns the first installed comma-decimal locale
// from a short preference list, or ("", false) if none are installed.
// Unlike pickUTF8Locale (which t.Fatals on Linux, since every mainstream
// Linux distribution ships a UTF-8 locale), this never fatals: a
// comma-decimal locale (de_DE, fr_FR) is not guaranteed present even on
// Linux CI images, so its absence is not a broken test environment -- it is
// an expected, skippable gap.
func pickCommaDecimalLocale(t *testing.T) (string, bool) {
	t.Helper()
	out, err := exec.Command("locale", "-a").Output()
	if err != nil {
		t.Fatalf("locale -a failed: %v", err)
	}
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", "")
	}
	available := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	for _, want := range []string{"de_de.utf8", "fr_fr.utf8"} {
		for _, l := range available {
			if norm(l) == want {
				return strings.TrimSpace(l), true
			}
		}
	}
	return "", false
}

func TestFloatLocaleRuntime(t *testing.T) {
	commaLocale, ok := pickCommaDecimalLocale(t)
	if !ok {
		t.Skip("no comma-decimal locale (de_DE/fr_FR) available on this host")
	}
	t.Logf("using comma-decimal locale: %s", commaLocale)

	for _, sh := range execShells(t) {
		sh := sh
		t.Run(sh.label, func(t *testing.T) {
			// Failure mode 1: to_string(float) must print "1.5", not "1,5".
			t.Run("to_string_not_comma", func(t *testing.T) {
				src := `fn get() -> float { return 1.5 }
fn main() -> int {
  print(to_string(get()))
  return 0
}`
				script := compileNS(t, src)
				out, stderr, code := runFloatLocale(t, sh, script, commaLocale)
				if code != 0 {
					t.Fatalf("exit %d, stderr: %s", code, stderr)
				}
				got := strings.TrimSpace(string(out))
				if got != "1.5" {
					t.Errorf("got %q, want \"1.5\" (locale must not leak a comma into float output)", got)
				}
			})

			// Failure mode 2: float arithmetic must not abort on the comma.
			t.Run("arithmetic_no_abort", func(t *testing.T) {
				src := `fn main() -> int {
  let b: float = 1.5 + 2.25
  print(to_string(b))
  return 0
}`
				script := compileNS(t, src)
				out, stderr, code := runFloatLocale(t, sh, script, commaLocale)
				if code != 0 {
					t.Fatalf("float arithmetic aborted under %s locale: exit %d, stderr: %s", commaLocale, code, stderr)
				}
				got := strings.TrimSpace(string(out))
				if got != "3.75" {
					t.Errorf("got %q, want \"3.75\"", got)
				}
			})

			// float_or is a validate-then-canonicalize helper, not an arithmetic
			// op: under the bug, a valid numeric string's awk-canonicalized form
			// comes back comma-separated, fails the [0-9.]-only case-glob
			// re-validation, and float_or silently returns its fallback instead
			// of the parsed value.
			t.Run("float_or_not_fallback", func(t *testing.T) {
				src := `fn main() -> int {
  let v: float = string.float_or("1.5", 0.0)
  print(to_string(v))
  return 0
}`
				script := compileNS(t, src, "string")
				out, stderr, code := runFloatLocale(t, sh, script, commaLocale)
				if code != 0 {
					t.Fatalf("exit %d, stderr: %s", code, stderr)
				}
				got := strings.TrimSpace(string(out))
				if got != "1.5" {
					t.Errorf("got %q, want \"1.5\" (float_or must not fall back to the default for a valid numeric string)", got)
				}
			})
		})
	}
}

// runFloatLocale compiles nothing itself; it writes the already-compiled
// script to a temp file and runs it under sh with the given locale.
// Mirrors the existing runBM helper's shape in
// internal/codegen/string_bytemodel_tail_runtime_test.go.
func runFloatLocale(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, script []byte, locale string) ([]byte, string, int) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{}, sh.args...), path)
	cmd := exec.Command(sh.bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL="+locale)
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
	return []byte(outBuf.String()), errBuf.String(), code
}
