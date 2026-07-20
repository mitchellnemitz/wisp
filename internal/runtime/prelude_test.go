package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shellAvailable reports whether dash is on PATH; tests that exec a shell skip
// when it is absent (CI guarantees dash + busybox).
func dashPath(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash not available")
	}
	return p
}

// runSnippet writes the named helpers (with deps resolved) plus a driver into a
// temp script, runs it under dash, and returns stdout, stderr, exit code.
func runSnippet(t *testing.T, helpers []string, driver string) (string, string, int) {
	t.Helper()
	dash := dashPath(t)
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
	cmd := exec.Command(dash, script)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v (stderr=%q)", err, errb.String())
		}
	}
	return out.String(), errb.String(), code
}

func TestFailHelper(t *testing.T) {
	// __wisp_fail takes a leading <pos> argument (M2 section 4); the format string
	// is the constant `wisp: %s: %s\n` with both pos and msg as %s data.
	out, errb, code := runSnippet(t, []string{"__wisp_fail"}, `__wisp_fail "prog.wisp:4:11" "boom"`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty", out)
	}
	if errb != "wisp: prog.wisp:4:11: boom\n" {
		t.Fatalf("stderr = %q, want %q", errb, "wisp: prog.wisp:4:11: boom\n")
	}
}

// TestFailHelperPosAndMsgInert verifies neither the position nor the message is
// ever treated as a printf format or shell code: a pos/msg containing % and
// shell-active text passes through as inert %s data.
func TestFailHelperPosAndMsgInert(t *testing.T) {
	_, errb, code := runSnippet(t, []string{"__wisp_fail"}, `__wisp_fail 'a$b.wisp:1:1' '100% $(touch /tmp/wisp_x) `+"`echo no`"+`'`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	want := "wisp: a$b.wisp:1:1: 100% $(touch /tmp/wisp_x) `echo no`\n"
	if errb != want {
		t.Fatalf("stderr = %q, want %q", errb, want)
	}
}

func TestPrintStdoutStderr(t *testing.T) {
	out, errb, code := runSnippet(t, []string{Print}, `__wisp_print "to-out" 1; __wisp_print "to-err" 2`)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "to-out\n" {
		t.Fatalf("stdout = %q", out)
	}
	if errb != "to-err\n" {
		t.Fatalf("stderr = %q", errb)
	}
}

func TestPrintInertData(t *testing.T) {
	// A message containing shell-active text must be inert.
	out, _, code := runSnippet(t, []string{Print}, `__wisp_print '$(touch /tmp/wisp_should_not_exist); `+"`echo no`"+`; "x"' 1`)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	want := "$(touch /tmp/wisp_should_not_exist); `echo no`; \"x\"\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

func TestPrintHelperIsNamespaced(t *testing.T) {
	if Print != "__wisp_print" {
		t.Fatalf("Print = %q, want %q", Print, "__wisp_print")
	}
	h, ok := registry[Print]
	if !ok {
		t.Fatalf("no helper registered for id %q", Print)
	}
	if !strings.Contains(h.src, "__wisp_print() {") {
		t.Errorf("helper src does not define __wisp_print():\n%s", h.src)
	}
	if strings.Contains(h.src, "\nprint() {") || strings.HasPrefix(h.src, "print() {") {
		t.Errorf("helper src still defines a bare print() function:\n%s", h.src)
	}
}

func TestStringIdentity(t *testing.T) {
	out, _, _ := runSnippet(t, []string{"string"}, `__wisp_string "42"; printf '%s\n' "$__ret"; __wisp_string "true"; printf '%s\n' "$__ret"`)
	if out != "42\ntrue\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestLength(t *testing.T) {
	out, _, _ := runSnippet(t, []string{"length"}, `__wisp_length "hello"; printf '%s\n' "$__ret"; __wisp_length ""; printf '%s\n' "$__ret"; __wisp_length "a*b"; printf '%s\n' "$__ret"`)
	if out != "5\n0\n3\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestBoolIntTable(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0", "false"},
		{"1", "true"},
		{"-5", "true"},
		{"42", "true"},
	}
	for _, tc := range cases {
		out, errb, code := runSnippet(t, []string{"__wisp_bool_int"}, `__wisp_bool_int `+tc.in+`; printf '%s\n' "$__ret"`)
		if code != 0 {
			t.Fatalf("bool_int(%s): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("bool_int(%s) = %q, want %q", tc.in, out, tc.want)
		}
	}
}

func TestBoolStrTable(t *testing.T) {
	pass := []struct{ in, want string }{
		{"true", "true"},
		{"false", "false"},
	}
	for _, tc := range pass {
		out, errb, code := runSnippet(t, []string{"__wisp_bool_str"}, `__wisp_bool_str "p:1:1" "`+tc.in+`"; printf '%s\n' "$__ret"`)
		if code != 0 {
			t.Fatalf("bool_str(%q): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("bool_str(%q) = %q, want %q", tc.in, out, tc.want)
		}
	}
	abort := []string{"", "1", "0", "True", " true ", "yes", "string", "TRUE"}
	for _, in := range abort {
		_, errb, code := runSnippet(t, []string{"__wisp_bool_str"}, `__wisp_bool_str "p:1:1" "`+in+`"; printf '%s\n' "$__ret"`)
		if code != 1 {
			t.Fatalf("bool_str(%q): exit %d, want 1", in, code)
		}
		if errb == "" {
			t.Fatalf("bool_str(%q): empty stderr, want abort message", in)
		}
		if !strings.Contains(errb, "bool(") {
			t.Fatalf("bool_str(%q): stderr %q lacks context label", in, errb)
		}
		// The located position is forwarded verbatim.
		if !strings.HasPrefix(errb, "wisp: p:1:1: ") {
			t.Fatalf("bool_str(%q): stderr %q missing located position prefix", in, errb)
		}
	}
}

func TestIntValidAndRange(t *testing.T) {
	ok := []struct{ in, want string }{
		{"42", "42"},
		{"-7", "-7"},
		{"+5", "5"},
		{"0", "0"},
		{"007", "7"},
		{"-0", "0"},
		{"9223372036854775807", "9223372036854775807"},
		{"-9223372036854775808", "-9223372036854775808"},
	}
	for _, tc := range ok {
		out, errb, code := runSnippet(t, []string{"int"}, `__wisp_int "p:1:1" "`+tc.in+`"; printf '%s\n' "$__ret"`)
		if code != 0 {
			t.Fatalf("int(%q): exit %d stderr %q", tc.in, code, errb)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("int(%q) = %q, want %q", tc.in, out, tc.want)
		}
	}
	bad := []string{"", " 5", "5 ", "abc", "1.5", "--", "+", "-", "1e3", "0x10"}
	for _, in := range bad {
		_, errb, code := runSnippet(t, []string{"int"}, `__wisp_int "p:1:1" "`+in+`"; printf '%s\n' "$__ret"`)
		if code != 1 {
			t.Fatalf("int(%q): exit %d, want 1 (bad input)", in, code)
		}
		if !strings.Contains(errb, "int(") {
			t.Fatalf("int(%q): stderr %q lacks context label", in, errb)
		}
		if !strings.HasPrefix(errb, "wisp: p:1:1: ") {
			t.Fatalf("int(%q): stderr %q missing located position prefix", in, errb)
		}
	}
	over := []string{"9223372036854775808", "-9223372036854775809", "99999999999999999999"}
	for _, in := range over {
		_, errb, code := runSnippet(t, []string{"int"}, `__wisp_int "p:1:1" "`+in+`"; printf '%s\n' "$__ret"`)
		if code != 1 {
			t.Fatalf("int(%q): exit %d, want 1 (over range)", in, code)
		}
		if !strings.Contains(errb, "int(") {
			t.Fatalf("int(%q): stderr %q lacks context label", in, errb)
		}
		if !strings.HasPrefix(errb, "wisp: p:1:1: ") {
			t.Fatalf("int(%q): stderr %q missing located position prefix", in, errb)
		}
	}
}

func TestLowerUpper(t *testing.T) {
	out, _, _ := runSnippet(t, []string{"lower", "upper"}, `__wisp_lower "HeLLo-World!"; printf '[%s]\n' "$__ret"; __wisp_upper "HeLLo-World!"; printf '[%s]\n' "$__ret"`)
	if out != "[hello-world!]\n[HELLO-WORLD!]\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestLowerUpperLeadingDash(t *testing.T) {
	// printf '%s' (not echo) so a leading dash is not consumed as a flag.
	out, _, _ := runSnippet(t, []string{"lower"}, `__wisp_lower "-N"; printf '[%s]\n' "$__ret"`)
	if out != "[-n]\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestLowerUpperTrailingNewline(t *testing.T) {
	// Trailing newlines must be preserved ("other bytes unchanged").
	out, _, _ := runSnippet(t, []string{"lower"}, "s=\"ABC\n\"; __wisp_lower \"$s\"; printf '[%s]END' \"$__ret\"")
	if out != "[abc\n]END" {
		t.Fatalf("lower stdout = %q, want %q", out, "[abc\n]END")
	}
	out, _, _ = runSnippet(t, []string{"upper"}, "s=\"abc\n\"; __wisp_upper \"$s\"; printf '[%s]END' \"$__ret\"")
	if out != "[ABC\n]END" {
		t.Fatalf("upper stdout = %q, want %q", out, "[ABC\n]END")
	}
}

func TestTrimLiteral(t *testing.T) {
	cases := []struct{ in, want string }{
		{"   hello   ", "hello"},
		{"\t mid \t", "mid"},
		{"  a  b  ", "a  b"}, // interior untouched
		{"***", "***"},       // metachars, no trim chars
		{"", ""},
		{"\r\n x \r\n", "x"},
	}
	for _, tc := range cases {
		// Build the input via printf so escapes become real bytes.
		drv := "s=$(printf '%s' " + shellSingleQuote(tc.in) + "); __wisp_trim \"$s\"; printf '[%s]' \"$__ret\""
		out, _, _ := runSnippet(t, []string{"trim"}, drv)
		if out != "["+tc.want+"]" {
			t.Fatalf("trim(%q) = %q, want %q", tc.in, out, "["+tc.want+"]")
		}
	}
}

func TestTrimInteriorNewline(t *testing.T) {
	out, _, _ := runSnippet(t, []string{"trim"}, "s=\"  a\nb  \"; __wisp_trim \"$s\"; printf '[%s]END' \"$__ret\"")
	if out != "[a\nb]END" {
		t.Fatalf("trim stdout = %q, want %q", out, "[a\nb]END")
	}
}

func TestReplaceLiteral(t *testing.T) {
	cases := []struct{ in, search, repl, want string }{
		{"hello world", "o", "0", "hell0 w0rld"},
		{"aaa", "a", "bb", "bbbbbb"},
		{"a*b*c", "*", "X", "aXbXc"},
		{"abc", "x", "y", "abc"},
		{"a[b]c", "[b]", "_", "a_c"},
		{"path/to/file", "/", "-", "path-to-file"},
		{"remove", "remove", "", ""},
		{"abab", "ab", "X", "XX"},
		{"axxbxxc", "xx", "-", "a-b-c"},
		{"a?b?c", "?", "Q", "aQbQc"},
		{`a\b`, `\`, "/", "a/b"},
	}
	for _, tc := range cases {
		drv := "__wisp_replace 'p:1:1' " + shellSingleQuote(tc.in) + " " + shellSingleQuote(tc.search) + " " + shellSingleQuote(tc.repl) + `; printf '[%s]' "$__ret"`
		out, errb, code := runSnippet(t, []string{"replace"}, drv)
		if code != 0 {
			t.Fatalf("replace(%q,%q,%q): exit %d stderr %q", tc.in, tc.search, tc.repl, code, errb)
		}
		if out != "["+tc.want+"]" {
			t.Fatalf("replace(%q,%q,%q) = %q, want %q", tc.in, tc.search, tc.repl, out, "["+tc.want+"]")
		}
	}
}

func TestReplaceTrailingNewline(t *testing.T) {
	out, _, _ := runSnippet(t, []string{"replace"}, "s=\"x\n\"; __wisp_replace \"p:1:1\" \"$s\" \"z\" \"q\"; printf '[%s]END' \"$__ret\"")
	if out != "[x\n]END" {
		t.Fatalf("replace stdout = %q, want %q", out, "[x\n]END")
	}
}

func TestReplaceEmptySearchAborts(t *testing.T) {
	_, errb, code := runSnippet(t, []string{"replace"}, `__wisp_replace "p:1:1" "abc" "" "x"`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errb, "replace(") {
		t.Fatalf("stderr %q lacks context label", errb)
	}
	if !strings.HasPrefix(errb, "wisp: p:1:1: ") {
		t.Fatalf("stderr %q missing located position prefix", errb)
	}
}

func TestTreeShakeResolvesDeps(t *testing.T) {
	// int depends on __wisp_fail; requesting only "int" must pull in the fail
	// helper, and must NOT emit unrelated helpers.
	src := Emit([]string{"int"})
	if !strings.Contains(src, "__wisp_fail()") {
		t.Fatalf("Emit([int]) missing dependency __wisp_fail:\n%s", src)
	}
	if strings.Contains(src, "__wisp_replace()") {
		t.Fatalf("Emit([int]) leaked unrelated helper __wisp_replace:\n%s", src)
	}
	if strings.Contains(src, "__wisp_lower()") {
		t.Fatalf("Emit([int]) leaked unrelated helper __wisp_lower:\n%s", src)
	}
}

func TestEmitEmpty(t *testing.T) {
	if got := Emit(nil); got != "" {
		t.Fatalf("Emit(nil) = %q, want empty", got)
	}
}

func TestEmitDeterministicOrderAndDeps(t *testing.T) {
	// A helper must appear after the helpers it depends on, and the output must
	// be stable regardless of request order.
	a := Emit([]string{"int", "replace", "bool"})
	b := Emit([]string{"bool", "replace", "int"})
	if a != b {
		t.Fatalf("Emit order not deterministic:\n--a--\n%s\n--b--\n%s", a, b)
	}
	// fail must precede the helpers that depend on it.
	if i, j := strings.Index(a, "__wisp_fail()"), strings.Index(a, "__wisp_int()"); i < 0 || j < 0 || i > j {
		t.Fatalf("dependency ordering wrong: fail@%d int@%d", i, j)
	}
}

// shellSingleQuote wraps s as a safe single-quoted shell literal for use in
// test drivers (mirrors the codegen encoding so tests feed exact bytes).
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// TestDictKeyEncodeDecodeRoundTrip checks the dict key encode/decode helpers
// round-trip every byte-string class: ASCII, the _len/element-namespace
// collision keys (`len`, `0`), shell-active bytes, an embedded quote, the empty
// string, and multibyte UTF-8 (treated as raw bytes under LC_ALL=C).
func TestDictKeyEncodeDecodeRoundTrip(t *testing.T) {
	keys := []string{
		"hello", "len", "0", "a b", "$(touch x)", "`id`", "a;b", "a'b", "a\"b",
		"", "ünï", "k", // a key literally "k" must still round-trip
	}
	for _, k := range keys {
		driver := "__wisp_dkey_enc " + shellSingleQuote(k) + "\n" +
			"enc=\"$__ret\"\n" +
			"__wisp_dkey_dec \"$enc\"\n" +
			"if [ \"$__ret\" = " + shellSingleQuote(k) + " ]; then printf 'OK %s\\n' \"$enc\"; else printf 'FAIL enc=%s dec=[%s]\\n' \"$enc\" \"$__ret\"; fi"
		out, errb, code := runSnippet(t, []string{DictEnc, DictDec}, driver)
		if code != 0 {
			t.Fatalf("key %q: exit=%d stderr=%q", k, code, errb)
		}
		if !strings.HasPrefix(out, "OK ") {
			t.Errorf("key %q did not round-trip: %q", k, out)
		}
	}
}

// TestDictKeyEncodeDistinct verifies two distinct keys never produce the same
// token, including the namespace-collision pair (`len` vs `0`) and a key that
// equals a token of another key.
func TestDictKeyEncodeDistinct(t *testing.T) {
	pairs := [][2]string{{"len", "0"}, {"a", "b"}, {"len", "len "}, {"0", "00"}}
	for _, p := range pairs {
		driver := "__wisp_dkey_enc " + shellSingleQuote(p[0]) + "; a=\"$__ret\"\n" +
			"__wisp_dkey_enc " + shellSingleQuote(p[1]) + "; b=\"$__ret\"\n" +
			"if [ \"$a\" = \"$b\" ]; then echo COLLIDE; else echo DISTINCT; fi"
		out, _, code := runSnippet(t, []string{DictEnc}, driver)
		if code != 0 || strings.TrimSpace(out) != "DISTINCT" {
			t.Errorf("keys %q/%q: out=%q code=%d, want DISTINCT", p[0], p[1], out, code)
		}
	}
}
