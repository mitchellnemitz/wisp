package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shellcheckSnippet lints the assembled helpers+driver, failing on a finding.
// Skips when shellcheck is unavailable.
func shellcheckSnippet(t *testing.T, helpers []string, driver string) {
	t.Helper()
	sc, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Skip("shellcheck not available")
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# shellcheck disable=SC3043,SC2050,SC2043\n")
	b.WriteString(Emit(helpers))
	b.WriteString("\n")
	b.WriteString(driver)
	b.WriteString("\n")
	dir := t.TempDir()
	path := filepath.Join(dir, "snippet.sh")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(sc, "--shell", "sh", "--severity", "warning", path)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("shellcheck found issues:\n%s\n--- script ---\n%s", out.String(), b.String())
	}
}

func TestFloatArithmetic(t *testing.T) {
	cases := []struct {
		helper string
		a, b   string
		want   string
	}{
		{FAdd, "1.5", "2.5", "4"},
		{FAdd, "3.14", "2", "5.1400000000000006"},
		{FSub, "5.0", "2.5", "2.5"},
		{FMul, "3.14", "2", "6.2800000000000002"},
		{FDiv, "7", "2", "3.5"},
		{FMul, "3.0", "2.0", "6"},
	}
	for _, tc := range cases {
		fn := tc.helper
		drv := fn + ` "p:1:1" "` + tc.a + `" "` + tc.b + `"; printf '%s\n' "$__ret"`
		shellcheckSnippet(t, []string{tc.helper}, drv)
		out, errb, code := runSnippet(t, []string{tc.helper}, drv)
		if code != 0 {
			t.Fatalf("%s(%s,%s): exit %d stderr %q", fn, tc.a, tc.b, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("%s(%s,%s) = %q, want %q", fn, tc.a, tc.b, strings.TrimSpace(out), tc.want)
		}
	}
}

func TestFloatDivByZeroAborts(t *testing.T) {
	drv := `__wisp_fdiv "p:2:3" "1.0" "0.0"; printf '%s\n' "$__ret"`
	_, errb, code := runSnippet(t, []string{FDiv}, drv)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", code, errb)
	}
	if !strings.Contains(errb, "division by zero") {
		t.Fatalf("stderr %q lacks 'division by zero'", errb)
	}
	if !strings.HasPrefix(errb, "wisp: p:2:3: ") {
		t.Fatalf("stderr %q missing located position prefix", errb)
	}
}

func TestFloatOverflowAborts(t *testing.T) {
	// 1e308-form magnitudes are not valid float literals, but a multiplication of
	// two in-range floats can overflow to inf. Use large in-domain decimals.
	// 99999999999999 * 99999999999999 ~ 1e28, whose %.17g is exponent form -> abort.
	drv := `__wisp_fmul "p:1:1" "99999999999999" "99999999999999"; printf '%s\n' "$__ret"`
	_, errb, code := runSnippet(t, []string{FMul}, drv)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", code, errb)
	}
	if !strings.Contains(errb, "float") {
		t.Fatalf("stderr %q lacks float-domain abort message", errb)
	}
}

func TestFloatInfAborts(t *testing.T) {
	// A literal-magnitude that awk renders as inf. We can't pass an exponent
	// string (rejected upstream), but ffinite itself must reject inf/nan.
	for _, v := range []string{"inf", "-inf", "nan", "1e+17", "1.5e10"} {
		drv := `__wisp_ffinite "p:1:1" "` + v + `"; printf '%s\n' "$__ret"`
		_, errb, code := runSnippet(t, []string{FFinite}, drv)
		if code != 1 {
			t.Fatalf("ffinite(%q): exit %d, want 1", v, code)
		}
		if !strings.HasPrefix(errb, "wisp: p:1:1: ") {
			t.Fatalf("ffinite(%q): stderr %q missing located prefix", v, errb)
		}
	}
}

func TestFloatFinitePasses(t *testing.T) {
	for _, v := range []string{"3.14", "-2", "0", "0.000", "-0.0", "100.001", "+5"} {
		drv := `__wisp_ffinite "p:1:1" "` + v + `"; printf '%s\n' "$__ret"`
		out, errb, code := runSnippet(t, []string{FFinite}, drv)
		if code != 0 {
			t.Fatalf("ffinite(%q): exit %d stderr %q", v, code, errb)
		}
		if strings.TrimSpace(out) != v {
			t.Fatalf("ffinite(%q) = %q, want passthrough", v, strings.TrimSpace(out))
		}
	}
}

func TestFloatCompare(t *testing.T) {
	cases := []struct {
		op, a, b, want string
	}{
		{"lt", "1.0", "2.0", "true"},
		{"lt", "2.0", "1.0", "false"},
		{"le", "2.0", "2.0", "true"},
		{"gt", "3.0", "2.0", "true"},
		{"ge", "3.0", "3.0", "true"},
		{"eq", "1.5", "1.5", "true"},
		{"eq", "1.5", "1.6", "false"},
		{"ne", "1.5", "1.6", "true"},
		{"eq", "3.14", "3.14", "true"},
	}
	for _, tc := range cases {
		drv := `__wisp_fcmp "p:1:1" "` + tc.op + `" "` + tc.a + `" "` + tc.b + `"; printf '%s\n' "$__ret"`
		shellcheckSnippet(t, []string{FCmp}, drv)
		out, errb, code := runSnippet(t, []string{FCmp}, drv)
		if code != 0 {
			t.Fatalf("fcmp(%s,%s,%s): exit %d stderr %q", tc.op, tc.a, tc.b, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("fcmp(%s,%s,%s) = %q, want %q", tc.op, tc.a, tc.b, strings.TrimSpace(out), tc.want)
		}
	}
}

func TestFloatBool(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0", "false"},
		{"0.0", "false"},
		{"0.000", "false"},
		{"-0.0", "false"},
		{"1.0", "true"},
		{"-3.14", "true"},
		{"0.0001", "true"},
	}
	for _, tc := range cases {
		drv := `__wisp_fbool "p:1:1" "` + tc.in + `"; printf '%s\n' "$__ret"`
		out, errb, code := runSnippet(t, []string{FBool}, drv)
		if code != 0 {
			t.Fatalf("fbool(%q): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("fbool(%q) = %q, want %q", tc.in, strings.TrimSpace(out), tc.want)
		}
	}
}

func TestFloatString(t *testing.T) {
	cases := []struct{ in, want string }{
		{"3.14", "3.1400000000000001"},
		{"2.0", "2"},
		{"-2", "-2"},
		{"0", "0"},
		{"0.5", "0.5"},
	}
	for _, tc := range cases {
		drv := `__wisp_fstr "p:1:1" "` + tc.in + `"; printf '%s\n' "$__ret"`
		out, errb, code := runSnippet(t, []string{FStr}, drv)
		if code != 0 {
			t.Fatalf("fstr(%q): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("fstr(%q) = %q, want %q", tc.in, strings.TrimSpace(out), tc.want)
		}
	}
}

func TestFloatOfInt(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2", "2"},
		{"-2", "-2"},
		{"0", "0"},
		{"1000000", "1000000"},
	}
	for _, tc := range cases {
		drv := `__wisp_ffloat_i "p:1:1" "` + tc.in + `"; printf '%s\n' "$__ret"`
		out, errb, code := runSnippet(t, []string{FFloatI}, drv)
		if code != 0 {
			t.Fatalf("ffloat_i(%q): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("ffloat_i(%q) = %q, want %q", tc.in, strings.TrimSpace(out), tc.want)
		}
	}
}

func TestFloatOfString(t *testing.T) {
	ok := []struct{ in, want string }{
		{"3.14", "3.1400000000000001"},
		{"-2", "-2"},
		{"2.0", "2"},
		{"0", "0"},
		{"+5", "5"},
	}
	for _, tc := range ok {
		drv := `__wisp_ffloat_s "p:1:1" "` + tc.in + `"; printf '%s\n' "$__ret"`
		shellcheckSnippet(t, []string{FFloatS}, drv)
		out, errb, code := runSnippet(t, []string{FFloatS}, drv)
		if code != 0 {
			t.Fatalf("ffloat_s(%q): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("ffloat_s(%q) = %q, want %q", tc.in, strings.TrimSpace(out), tc.want)
		}
	}
	bad := []string{"x", "3.", ".5", "1e5", "inf", "nan", "1.2.3", "", "12abc", " 3.14"}
	for _, in := range bad {
		drv := `__wisp_ffloat_s "p:3:4" "` + in + `"; printf '%s\n' "$__ret"`
		_, errb, code := runSnippet(t, []string{FFloatS}, drv)
		if code != 1 {
			t.Fatalf("ffloat_s(%q): exit %d, want 1 (bad input)", in, code)
		}
		if !strings.Contains(errb, "float(") {
			t.Fatalf("ffloat_s(%q): stderr %q lacks 'float(' label", in, errb)
		}
		if !strings.HasPrefix(errb, "wisp: p:3:4: ") {
			t.Fatalf("ffloat_s(%q): stderr %q missing located prefix", in, errb)
		}
	}
}

func TestIntOfFloat(t *testing.T) {
	cases := []struct{ in, want string }{
		{"3.9", "3"},
		{"-3.9", "-3"},
		{"2.0", "2"},
		{"0.0", "0"},
		{"-0.5", "0"},
		{"99999999999999.99", "99999999999999"},
	}
	for _, tc := range cases {
		drv := `__wisp_fint "p:1:1" "` + tc.in + `"; printf '%s\n' "$__ret"`
		out, errb, code := runSnippet(t, []string{FIntT}, drv)
		if code != 0 {
			t.Fatalf("fint(%q): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("fint(%q) = %q, want %q", tc.in, strings.TrimSpace(out), tc.want)
		}
	}
}

// TestFloatInjectionInert proves the awk program text is constant and operands
// flow only via -v: a crafted float-looking string with shell/awk-active bytes
// is either inert or a clean located abort, never executed.
func TestFloatInjectionInert(t *testing.T) {
	// These strings are NOT valid floats, so float(string) must abort located
	// without executing the embedded command.
	crafted := []string{
		`1; system("touch /tmp/wisp_pwn")`,
		"$(touch /tmp/wisp_pwn)",
		"`touch /tmp/wisp_pwn`",
		`a"]; system("id"); x="`,
	}
	for _, in := range crafted {
		drv := `__wisp_ffloat_s "p:1:1" ` + shellSingleQuote(in) + `; printf '%s\n' "$__ret"`
		_, errb, code := runSnippet(t, []string{FFloatS}, drv)
		if code != 1 {
			t.Fatalf("ffloat_s(%q): exit %d, want 1 (clean abort)", in, code)
		}
		if !strings.Contains(errb, "float(") {
			t.Fatalf("ffloat_s(%q): stderr %q lacks float label", in, errb)
		}
	}
	if _, err := os.Stat("/tmp/wisp_pwn"); err == nil {
		os.Remove("/tmp/wisp_pwn")
		t.Fatal("injection executed: /tmp/wisp_pwn was created")
	}
}

func TestFloatTreeShakeDeps(t *testing.T) {
	// fadd pulls in ffinite and fail; fint pulls in __wisp_int (and fail).
	src := Emit([]string{FAdd})
	if !strings.Contains(src, "__wisp_ffinite()") || !strings.Contains(src, "__wisp_fail()") {
		t.Fatalf("Emit([fadd]) missing deps:\n%s", src)
	}
	src = Emit([]string{FIntT})
	if !strings.Contains(src, "__wisp_int()") {
		t.Fatalf("Emit([fint]) missing __wisp_int dep:\n%s", src)
	}
	// A no-float request must not emit any float helper.
	src = Emit([]string{"print"})
	for _, h := range []string{"__wisp_fadd()", "__wisp_ffinite()", "__wisp_fcmp()", "__wisp_fstr()"} {
		if strings.Contains(src, h) {
			t.Fatalf("Emit([print]) leaked float helper %q", h)
		}
	}
}
