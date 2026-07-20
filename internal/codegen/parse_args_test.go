package codegen

import (
	"strings"
	"testing"
)

// parse_args stays flat; these tests only INCIDENTALLY used the removable dict
// builtins get / has and the removable array builtin join to inspect the parsed
// result. Since compile() type-checks a single unlinked module (where namespaced
// dict.get / array.join do not resolve), the bodies are rewritten to inspect the
// result with language constructs only -- the dget / dhas / sjoin wisp helpers
// below iterate via for-in and index directly, preserving each assertion exactly.
// The parse_args classification property under test is unchanged.
const paHelpers = `
fn dget(d: {string: string}, k: string) -> string {
  for (kk in d) { if (kk == k) { return d[kk] } }
  return "?"
}
fn dhas(d: {string: string}, k: string) -> bool {
  for (kk in d) { if (kk == k) { return true } }
  return false
}
fn sjoin(xs: string[], sep: string) -> string {
  let out: string = ""
  let first: bool = true
  for (x in xs) {
    if (first) { out = x
      first = false } else { out = out + sep + x }
  }
  return out
}
`

// TestParseArgs_AC1 covers the basic classification plus the `=` form.
func TestParseArgs_AC1(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let (v: {string: string}, s: string[], p: string[]) = parse_args(["--name", "ada", "--verbose", "f1", "f2"], ["--name"])
  print(dget(v, "--name"))
  print(sjoin(s, ","))
  print(sjoin(p, ","))
  let (v2: {string: string}, _, _) = parse_args(["--name=ada"], ["--name"])
  print(dget(v2, "--name"))
  return 0
}`+paHelpers)
	if out != "ada\n--verbose\nf1,f2\nada\n" {
		t.Errorf("out=%q", out)
	}
}

// TestParseArgs_AC2 covers the `--` terminator: dropped, the rest positional.
func TestParseArgs_AC2(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let (v: {string: string}, s: string[], p: string[]) = parse_args(["--name", "ada", "--", "--not-a-flag", "x"], ["--name"])
  print(dget(v, "--name"))
  print("${length(s)}")
  print(sjoin(p, ","))
  return 0
}`+paHelpers)
	if out != "ada\n0\n--not-a-flag,x\n" {
		t.Errorf("out=%q", out)
	}
}

// TestParseArgs_AC3 covers a space-form value-flag consuming a flag-shaped next
// token, and the end-of-args omit.
func TestParseArgs_AC3(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let (v: {string: string}, _, _) = parse_args(["-o", "--weird"], ["-o"])
  print(dget(v, "-o"))
  let (v2: {string: string}, _, _) = parse_args(["-o"], ["-o"])
  print("${dhas(v2, "-o")}")
  return 0
}`+paHelpers)
	if out != "--weird\nfalse\n" {
		t.Errorf("out=%q", out)
	}
}

// TestParseArgs_AC3a covers the `-o --` precedence, the empty `--name=` value,
// and the distinct (non-deduped) `=`-form switches.
func TestParseArgs_AC3a(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let novf: string[] = []
  let (v: {string: string}, _, p: string[]) = parse_args(["-o", "--", "x"], ["-o"])
  print(dget(v, "-o") + "|" + sjoin(p, ","))
  let (v2: {string: string}, _, _) = parse_args(["--name="], ["--name"])
  print("${dhas(v2, "--name")}|[" + dget(v2, "--name") + "]")
  let (_, s: string[], _) = parse_args(["--verbose", "--verbose=1"], novf)
  print(sjoin(s, ","))
  return 0
}`+paHelpers)
	if out != "--|x\ntrue|[]\n--verbose,--verbose=1\n" {
		t.Errorf("out=%q", out)
	}
}

// TestParseArgs_AC4 covers last-occurrence-wins, the lone `-` positional, and
// exact-string switch dedup.
func TestParseArgs_AC4(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let novf: string[] = []
  let (v: {string: string}, _, _) = parse_args(["--n", "a", "--n", "b"], ["--n"])
  print(dget(v, "--n"))
  let (_, _, p: string[]) = parse_args(["-"], novf)
  print(sjoin(p, ",") + "|${length(p)}")
  let (_, s: string[], _) = parse_args(["--v", "--v"], novf)
  print(sjoin(s, ",") + "|${length(s)}")
  return 0
}`+paHelpers)
	if out != "b\n-|1\n--v|1\n" {
		t.Errorf("out=%q", out)
	}
}

// TestParseArgs_InjectionSafe (AC5): a value/positional with shell
// metacharacters renders literally; no command substitution runs.
func TestParseArgs_InjectionSafe(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let d: string = "$(echo PWNED); `+"`echo NO`"+`; *"
  let (v: {string: string}, _, p: string[]) = parse_args(["--cmd", d, d], ["--cmd"])
  print(dget(v, "--cmd"))
  print(p[0])
  return 0
}`+paHelpers)
	want := "$(echo PWNED); `echo NO`; *\n$(echo PWNED); `echo NO`; *\n"
	if out != want {
		t.Errorf("out=%q want=%q", out, want)
	}
}

// TestParseArgs_TreeShaken: a program that does not use parse_args must not
// carry the helper.
func TestParseArgs_TreeShaken(t *testing.T) {
	noUse := string(compile(t, "fn main() -> int {\n  return 0\n}"))
	if strings.Contains(noUse, "__wisp_parse_args") {
		t.Error("__wisp_parse_args emitted in a program that does not use parse_args")
	}
	use := string(compile(t, `fn main() -> int {
  let (_, _, _) = parse_args(["a"], ["--x"])
  return 0
}`))
	if !strings.Contains(use, "__wisp_parse_args()") {
		t.Error("__wisp_parse_args not emitted when parse_args is used")
	}
}
