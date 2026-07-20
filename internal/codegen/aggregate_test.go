package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runArgs compiles+shellchecks src then runs it under dash with argv, returning
// stdout/stderr/exit. Skips when dash is unavailable.
func runArgs(t *testing.T, src string, argv ...string) (string, string, int) {
	t.Helper()
	script := compile(t, src)
	shellcheck(t, script)
	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(dash, append([]string{path}, argv...)...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v", err)
		}
	}
	return out.String(), errb.String(), code
}

func TestStructAliasMutation(t *testing.T) {
	out, errb, code := runWisp(t, `struct Counter { n: int }
fn bump(c: Counter) -> void { c.n = c.n + 1 }
fn main() -> int {
  let a: Counter = Counter { n: 10 }
  let b: Counter = a
  b.n = 99
  bump(a)
  print("a=${a.n}")
  print("b=${b.n}")
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "a=100\nb=100\n" {
		t.Errorf("out=%q, want a=100\\nb=100\\n", out)
	}
}

func TestStructParamReturn(t *testing.T) {
	out, _, code := runWisp(t, `struct Box { v: int }
fn make(n: int) -> Box { return Box { v: n } }
fn main() -> int {
  let b: Box = make(7)
  print(to_string(b.v))
  return 0
}`)
	if code != 0 || out != "7\n" {
		t.Errorf("out=%q code=%d, want 7", out, code)
	}
}

// TestArrayForInSum exercises for-in summation and length, growing the array
// with array.push (the namespaced spelling; bare push no longer resolves in the
// single-module check).
func TestArrayForInSum(t *testing.T) {
	out, _, code := runNS(t, `fn main() -> int {
  let xs: int[] = [3, 4, 5]
  array.push(xs, 6)
  let total: int = 0
  for (x in xs) { total = total + x }
  print("sum=${total}")
  print("len=${length(xs)}")
  return 0
}`, "array")
	if code != 0 || out != "sum=18\nlen=4\n" {
		t.Errorf("out=%q code=%d, want sum=18 len=4", out, code)
	}
}

func TestArrayIndexMutate(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  let xs: int[] = [1, 2, 3]
  xs[1] = 20
  print(to_string(xs[0]))
  print(to_string(xs[1]))
  print(to_string(xs[2]))
  return 0
}`)
	if code != 0 || out != "1\n20\n3\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestArrayOutOfBoundsAbort(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  let xs: int[] = [1, 2]
  print(to_string(xs[5]))
  return 0
}`)
	if code != 1 {
		t.Errorf("exit=%d, want 1", code)
	}
	if !strings.Contains(errb, "out of bounds") || !strings.Contains(errb, ":3:") {
		t.Errorf("stderr=%q, want located out-of-bounds", errb)
	}
}

func TestArrayNegativeIndexAbort(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  let xs: int[] = [1, 2]
  let i: int = 0 - 1
  print(to_string(xs[i]))
  return 0
}`)
	if code != 1 || !strings.Contains(errb, "out of bounds") {
		t.Errorf("exit=%d stderr=%q, want located abort", code, errb)
	}
}

func TestNestedArrayOfStruct(t *testing.T) {
	out, _, code := runWisp(t, `struct Point { x: int, y: int }
fn main() -> int {
  let ps: Point[] = [Point { x: 1, y: 2 }, Point { x: 3, y: 4 }]
  ps[0].x = 100
  print(to_string(ps[0].x))
  print(to_string(ps[1].y))
  return 0
}`)
	if code != 0 || out != "100\n4\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestNestedArrayOfArray(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  let g: int[][] = [[1, 2], [3, 4, 5]]
  print(to_string(length(g[1])))
  print(to_string(g[1][2]))
  return 0
}`)
	if code != 0 || out != "3\n5\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestForInBlockScopeSiblingReuse(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  let xs: int[] = [1, 2]
  let ys: int[] = [10, 20]
  for (x in xs) { print(to_string(x)) }
  for (x in ys) { print(to_string(x)) }
  return 0
}`)
	if code != 0 || out != "1\n2\n10\n20\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestMainArgsEcho(t *testing.T) {
	src := `fn main(args: string[]) -> int {
  print("argc=${length(args)}")
  for (a in args) { print(a) }
  return 0
}`
	out, _, code := runArgs(t, src, "apple", "two words", "cherry")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out != "argc=3\napple\ntwo words\ncherry\n" {
		t.Errorf("out=%q", out)
	}
}

// TestMainArgsFullAPI proves main's args bind as a real array usable with the
// stays-flat array API (length + indexing), and that it is mutable via
// array.push (the namespaced spelling; bare push no longer resolves in the
// single-module check).
func TestMainArgsFullAPI(t *testing.T) {
	src := `fn main(args: string[]) -> int {
  array.push(args, "extra")
  print(to_string(length(args)))
  print(args[0])
  return 0
}`
	out, _, code := runArgsNS(t, src, []string{"first"}, "array")
	if code != 0 || out != "2\nfirst\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestMainArgsZeroArgsStillWorks(t *testing.T) {
	src := `fn main(args: string[]) -> int {
  print(to_string(length(args)))
  return 0
}`
	out, _, code := runArgs(t, src)
	if code != 0 || out != "0\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestZeroArgMainStillWorks(t *testing.T) {
	out, _, code := runArgs(t, "fn main() -> int { print(\"ok\")\n return 0 }", "ignored")
	if code != 0 || out != "ok\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestArgInjectionInert(t *testing.T) {
	// An arg containing shell-active bytes must be inert data, never executed.
	src := `fn main(args: string[]) -> int {
  for (a in args) { print(a) }
  return 0
}`
	out, _, code := runArgs(t, src, "$(touch /tmp/wisp_pwn)", "`id`", "a;b")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	want := "$(touch /tmp/wisp_pwn)\n`id`\na;b\n"
	if out != want {
		t.Errorf("out=%q, want %q", out, want)
	}
}

// TestAggregateSourceMap asserts the generated lines lowering an array literal
// and an index read carry a non-nil source position (M3 source-map coverage).
func TestAggregateSourceMap(t *testing.T) {
	src := "fn main() -> int {\n  let xs: int[] = [1, 2]\n  print(to_string(xs[0]))\n  return 0\n}"
	script, lm := compileWithMap(t, src)
	lines := strings.Split(strings.TrimSuffix(string(script), "\n"), "\n")
	if len(lines) != len(lm) {
		t.Fatalf("lines=%d lm=%d", len(lines), len(lm))
	}
	// The array-literal element set (`eval "__wisp_a_${...}_0=\$..."`) must map to
	// the let on source line 2; the bounds check for xs[0] must map to source
	// line 3.
	sawLit := false
	sawIdx := false
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "eval \"__wisp_a_") && strings.Contains(trimmed, "_0=") {
			sawLit = true
			if lm[i] == nil || lm[i].Line != 2 {
				t.Errorf("array-literal line %q maps to %v, want src line 2", trimmed, lm[i])
			}
		}
		if strings.Contains(trimmed, "__wisp_bounds_fail") {
			if lm[i] != nil && lm[i].Line == 3 {
				sawIdx = true
			}
		}
	}
	if !sawLit {
		t.Errorf("did not find array-literal element line in:\n%s", script)
	}
	if !sawIdx {
		t.Errorf("bounds-check line did not map to source line 3")
	}
}

// TestAggregateStringFieldInjectionInert asserts a string field/element holding
// shell-active bytes is stored and read back inertly (regression: the eval-set
// must carry the value through a deferred \$temp, never inline a single-quoted
// token inside the eval's double quotes).
func TestAggregateStringFieldInjectionInert(t *testing.T) {
	out, _, code := runWisp(t, "struct Msg { text: string }\nfn main() -> int {\n  let m: Msg = Msg { text: \"a'b\\\"c$d`e;f|g\" }\n  print(m.text)\n  let xs: string[] = [\"$(echo x)\"]\n  print(xs[0])\n  return 0\n}")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	want := "a'b\"c$d`e;f|g\n$(echo x)\n"
	if out != want {
		t.Errorf("out=%q, want %q", out, want)
	}
}
