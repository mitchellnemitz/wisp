package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// extMathShells are the POSIX shells exercised at the helper level for the
// __wisp_exp/__wisp_ln guards. busybox ash (the no-libm awk target that is the
// documented reason these series are pure-arithmetic) is covered by the act
// golden post-handoff; here we assert the three locally-available shells agree.
var extMathShells = []string{"dash", "bash", "zsh"}

// runHelperShell writes the named helpers + driver to a temp script and runs it
// under sh with a hard timeout, returning stdout, stderr, exit code, and whether
// the timeout fired. The timeout is the no-hang proof for the ln domain guard:
// without the guard, ln(0)/ln(neg) spin the range-reduction loop forever and the
// context deadline trips.
func runHelperShell(t *testing.T, sh string, helpers []string, driver string) (string, string, int, bool) {
	t.Helper()
	shPath, err := exec.LookPath(sh)
	if err != nil {
		t.Skipf("%s not available", sh)
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString(Emit(helpers))
	b.WriteString("\n")
	b.WriteString(driver)
	b.WriteString("\n")
	dir := t.TempDir()
	script := filepath.Join(dir, "snippet.sh")
	if err := os.WriteFile(script, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shPath, script)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	timedOut := ctx.Err() == context.DeadlineExceeded
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if !timedOut {
			t.Fatalf("%s run: %v (stderr=%q)", sh, err, errb.String())
		}
	}
	return out.String(), errb.String(), code, timedOut
}

// TestLnGuardStructure is the structural assertion that the non-positive-domain
// guard PRECEDES the range-reduction loop in the emitted __wisp_ln source -- the
// invariant that prevents ln(0) from hanging. A pure behavioral test could pass
// on a hang-prone helper only because the OS killed it; this pins the source
// order directly.
func TestLnGuardStructure(t *testing.T) {
	src := Emit([]string{Ln})
	guard := `if (x <= 0) { print "nan"; exit }`
	loop := `while (x < 0.6)`
	gi := strings.Index(src, guard)
	li := strings.Index(src, loop)
	if gi < 0 {
		t.Fatalf("__wisp_ln missing non-positive-domain guard %q:\n%s", guard, src)
	}
	if li < 0 {
		t.Fatalf("__wisp_ln missing range-reduction loop %q:\n%s", loop, src)
	}
	if gi >= li {
		t.Fatalf("__wisp_ln domain guard (at %d) must precede the range-reduction loop (at %d)", gi, li)
	}
}

// TestLnDomainAbortsPromptly proves ln(0)/ln(negative) abort located WITHOUT
// hanging, on every available shell. The 10s context timeout is the no-hang
// proof: a missing guard would spin the `while (x < 0.6) x = x*2` loop forever.
func TestLnDomainAbortsPromptly(t *testing.T) {
	for _, sh := range extMathShells {
		for _, in := range []string{"0.0", "-1.0", "-0.5", "0"} {
			drv := `__wisp_ln "p:2:3" "` + in + `"; printf '%s\n' "$__ret"`
			_, errb, code, timedOut := runHelperShell(t, sh, []string{Ln}, drv)
			if timedOut {
				t.Fatalf("%s: __wisp_ln(%q) HUNG (domain guard did not fire before the loop)", sh, in)
			}
			if code != 1 {
				t.Fatalf("%s: __wisp_ln(%q) exit %d, want 1 (stderr=%q)", sh, in, code, errb)
			}
			if !strings.Contains(errb, "ln(") {
				t.Fatalf("%s: __wisp_ln(%q) stderr %q lacks 'ln(' label", sh, in, errb)
			}
			if !strings.HasPrefix(errb, "wisp: p:2:3: ") {
				t.Fatalf("%s: __wisp_ln(%q) stderr %q missing located prefix", sh, in, errb)
			}
		}
	}
}

// TestLnFiniteValues sanity-checks the factored ln series itself: ln(1)==0
// exactly (t==0), and ln(e)~=1 cross-shell.
func TestLnFiniteValues(t *testing.T) {
	for _, sh := range extMathShells {
		drv := `__wisp_ln "p:1:1" "1.0"; printf '%s\n' "$__ret"`
		out, errb, code, _ := runHelperShell(t, sh, []string{Ln}, drv)
		if code != 0 {
			t.Fatalf("%s: ln(1.0) exit %d stderr %q", sh, code, errb)
		}
		if strings.TrimSpace(out) != "0" {
			t.Fatalf("%s: ln(1.0) = %q, want 0", sh, strings.TrimSpace(out))
		}
		drv = `__wisp_ln "p:1:1" "2.718281828459045"; printf '%s\n' "$__ret"`
		out, errb, code, _ = runHelperShell(t, sh, []string{Ln}, drv)
		if code != 0 {
			t.Fatalf("%s: ln(e) exit %d stderr %q", sh, code, errb)
		}
		if !strings.HasPrefix(strings.TrimSpace(out), "0.99999") && !strings.HasPrefix(strings.TrimSpace(out), "1") {
			t.Fatalf("%s: ln(e) = %q, want ~1", sh, strings.TrimSpace(out))
		}
	}
}

// TestExpOverflowAbortsAcrossShells proves a too-large exp lands on a token
// __wisp_ffinite (the case-glob) rejects -- a LOCATED abort, not a divergent
// mid-loop value -- on every available shell. exp(100) ~= 2.7e43 overflows the
// Taylor accumulation to +inf (caught by the acc==acc/2 guard) or renders in
// exponent notation; either way the glob rejects it. The shells must agree.
func TestExpOverflowAbortsAcrossShells(t *testing.T) {
	for _, sh := range extMathShells {
		for _, in := range []string{"100.0", "40.0", "710.0"} {
			drv := `__wisp_exp "p:5:7" "` + in + `"; printf '%s\n' "$__ret"`
			_, errb, code, timedOut := runHelperShell(t, sh, []string{Exp}, drv)
			if timedOut {
				t.Fatalf("%s: __wisp_exp(%q) HUNG", sh, in)
			}
			if code != 1 {
				t.Fatalf("%s: __wisp_exp(%q) exit %d, want 1 (stderr=%q)", sh, in, code, errb)
			}
			if !strings.Contains(errb, "exp(") {
				t.Fatalf("%s: __wisp_exp(%q) stderr %q lacks 'exp(' label", sh, in, errb)
			}
			if !strings.HasPrefix(errb, "wisp: p:5:7: ") {
				t.Fatalf("%s: __wisp_exp(%q) stderr %q missing located prefix", sh, in, errb)
			}
		}
	}
}

// TestExpFiniteValues checks exp(0)==1 and exp(1)~=e cross-shell.
func TestExpFiniteValues(t *testing.T) {
	for _, sh := range extMathShells {
		drv := `__wisp_exp "p:1:1" "0.0"; printf '%s\n' "$__ret"`
		out, errb, code, _ := runHelperShell(t, sh, []string{Exp}, drv)
		if code != 0 {
			t.Fatalf("%s: exp(0.0) exit %d stderr %q", sh, code, errb)
		}
		if strings.TrimSpace(out) != "1" {
			t.Fatalf("%s: exp(0.0) = %q, want 1", sh, strings.TrimSpace(out))
		}
		drv = `__wisp_exp "p:1:1" "1.0"; printf '%s\n' "$__ret"`
		out, errb, code, _ = runHelperShell(t, sh, []string{Exp}, drv)
		if code != 0 {
			t.Fatalf("%s: exp(1.0) exit %d stderr %q", sh, code, errb)
		}
		if !strings.HasPrefix(strings.TrimSpace(out), "2.718281828") {
			t.Fatalf("%s: exp(1.0) = %q, want ~2.71828", sh, strings.TrimSpace(out))
		}
	}
}

// TestExpLnShellcheckClean lints the assembled helpers + a call so the new
// helper source carries no shellcheck warnings (AC3).
func TestExpLnShellcheckClean(t *testing.T) {
	shellcheckSnippet(t, []string{Ln}, `__wisp_ln "p:1:1" "2.0"; printf '%s\n' "$__ret"`)
	shellcheckSnippet(t, []string{Exp}, `__wisp_exp "p:1:1" "2.0"; printf '%s\n' "$__ret"`)
}

// TestExpLnTreeShake confirms the new helpers are tree-shaken: a program that
// uses neither emits neither, and each pulls in only __wisp_fail.
func TestExpLnTreeShake(t *testing.T) {
	src := Emit([]string{"print"})
	for _, h := range []string{"__wisp_exp()", "__wisp_ln()"} {
		if strings.Contains(src, h) {
			t.Fatalf("Emit([print]) leaked %q", h)
		}
	}
	src = Emit([]string{Ln})
	if !strings.Contains(src, "__wisp_ln()") || !strings.Contains(src, "__wisp_fail()") {
		t.Fatalf("Emit([ln]) missing ln or its fail dep:\n%s", src)
	}
	if strings.Contains(src, "__wisp_pow()") {
		t.Fatalf("Emit([ln]) must not drag in __wisp_pow")
	}
}
