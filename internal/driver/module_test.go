package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProj(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runProj(t *testing.T, dir, rootRel string) (string, string, int) {
	t.Helper()
	root := filepath.Join(dir, rootRel)
	b, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(root, string(b), nil, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func compileProj(t *testing.T, dir, rootRel string) ([]byte, []Diagnostic) {
	t.Helper()
	root := filepath.Join(dir, rootRel)
	b, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	script, _, diags := Compile(root, string(b))
	return script, diags
}

func TestModuleIncludeE2E(t *testing.T) {
	dir := writeProj(t, map[string]string{
		"main.wisp": `include "./lib/math.wisp" as m` + "\n" +
			`fn main() -> int {` + "\n" +
			`  print("sum=${to_string(m.add(2, 3))}")` + "\n" +
			`  return 0` + "\n" +
			`}`,
		"lib/math.wisp": `export fn add(a: int, b: int) -> int { return a + b }`,
	})
	out, errb, code := runProj(t, dir, "main.wisp")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	if out != "sum=5\n" {
		t.Errorf("stdout = %q, want %q", out, "sum=5\n")
	}
}

func TestModuleImportE2E(t *testing.T) {
	dir := writeProj(t, map[string]string{
		"wisp.json": `{"name":"app","main":"main.wisp"}`,
		"main.wisp": `import "acme/strutil"` + "\n" +
			`fn main() -> int {` + "\n" +
			`  print(strutil.shout("hi"))` + "\n" +
			`  return 0` + "\n" +
			`}`,
		".wisp/modules/acme/strutil/wisp.json": `{"name":"strutil","main":"s.wisp"}`,
		".wisp/modules/acme/strutil/s.wisp":    "import \"string\"\n" + `export fn shout(s: string) -> string { return string.upper(s) }`,
	})
	out, errb, code := runProj(t, dir, "main.wisp")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	if out != "HI\n" {
		t.Errorf("stdout = %q, want %q", out, "HI\n")
	}
}

func TestModuleCrossStructE2E(t *testing.T) {
	// A struct defined in an included module, constructed and read through the
	// namespace, round-trips at runtime.
	dir := writeProj(t, map[string]string{
		"main.wisp": `include "./geo.wisp" as g` + "\n" +
			`fn main() -> int {` + "\n" +
			`  let p: g.Point = g.Point { x: 3, y: 4 }` + "\n" +
			`  print(to_string(g.sumxy(p)))` + "\n" +
			`  return 0` + "\n" +
			`}`,
		"geo.wisp": `export struct Point { x: int, y: int }` + "\n" +
			`export fn sumxy(p: Point) -> int { return p.x + p.y }`,
	})
	out, errb, code := runProj(t, dir, "main.wisp")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	if out != "7\n" {
		t.Errorf("stdout = %q, want %q", out, "7\n")
	}
}

func TestModuleCrossTypeError(t *testing.T) {
	dir := writeProj(t, map[string]string{
		"main.wisp": `include "./lib.wisp" as l` + "\n" +
			`fn main() -> int {` + "\n" +
			`  return l.add("x", 3)` + "\n" +
			`}`,
		"lib.wisp": `export fn add(a: int, b: int) -> int { return a + b }`,
	})
	_, diags := compileProj(t, dir, "main.wisp")
	var found bool
	for _, d := range diags {
		if d.Severity == Error && strings.Contains(d.Msg, "want int") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a cross-module argument type error, got %v", diags)
	}
}

func TestWispJsonMainNotBuildTarget(t *testing.T) {
	// wisp.json main names other.wisp, but we build app.wisp; app.wisp's main is
	// the entry, and other.wisp's main field never overrides the build target.
	dir := writeProj(t, map[string]string{
		"wisp.json":  `{"name":"app","main":"other.wisp"}`,
		"app.wisp":   `fn main() -> int { print("from app"); return 0 }`,
		"other.wisp": `fn main() -> int { print("from other"); return 0 }`,
	})
	out, errb, code := runProj(t, dir, "app.wisp")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	if out != "from app\n" {
		t.Errorf("stdout = %q, want %q (the command-line root is the build target)", out, "from app\n")
	}
}

func TestTreeShakeUnusedImport(t *testing.T) {
	// An imported package exports a used and an unused function. Only the reached
	// one is emitted.
	dir := writeProj(t, map[string]string{
		"wisp.json": `{"name":"app","main":"main.wisp"}`,
		"main.wisp": `import "o/r"` + "\n" +
			`fn main() -> int { print(to_string(r.used())); return 0 }`,
		".wisp/modules/o/r/wisp.json": `{"name":"r","main":"r.wisp"}`,
		".wisp/modules/o/r/r.wisp": `export fn used() -> int { return 1 }` + "\n" +
			`export fn unused() -> int { return 2 }`,
	})
	script, diags := compileProj(t, dir, "main.wisp")
	for _, d := range diags {
		if d.Severity == Error {
			t.Fatalf("unexpected error: %v", diags)
		}
	}
	s := string(script)
	if !strings.Contains(s, "__wisp_f_m1_used") {
		t.Errorf("expected the used import function to be emitted")
	}
	if strings.Contains(s, "__wisp_f_m1_unused") {
		t.Errorf("unused import function should have been tree-shaken out")
	}
}

func TestTreeShakeFuncrefValueVsUnused(t *testing.T) {
	// An imported package exports two functions: one taken as a funcref VALUE
	// (never called directly) and one genuinely unreferenced. The funcref-taken
	// one must survive (Fix #2's new MemberFuncRefs reachability edge); the
	// unreferenced one must still be tree-shaken out (Fix #2 must not make it
	// blanket-reachable).
	dir := writeProj(t, map[string]string{
		"wisp.json": `{"name":"app","main":"main.wisp"}`,
		"main.wisp": `import "o/r"` + "\n" +
			`fn main() -> int { let h: fn(int) -> int = r.kept; print(to_string(h(1))); return 0 }`,
		".wisp/modules/o/r/wisp.json": `{"name":"r","main":"r.wisp"}`,
		".wisp/modules/o/r/r.wisp": `export fn kept(x: int) -> int { return x }` + "\n" +
			`export fn unused() -> int { return 2 }`,
	})
	script, diags := compileProj(t, dir, "main.wisp")
	for _, d := range diags {
		if d.Severity == Error {
			t.Fatalf("unexpected error: %v", diags)
		}
	}
	s := string(script)
	if !strings.Contains(s, "__wisp_f_m1_kept() {") {
		t.Errorf("expected the funcref-referenced import function's definition to be emitted")
	}
	if strings.Contains(s, "__wisp_f_m1_unused") {
		t.Errorf("unused import function should have been tree-shaken out")
	}
}

func TestSourceMapSpansModules(t *testing.T) {
	// A --source-map build over a multi-module program emits per-line entries; a
	// line originating in the included module records that module's file (lineEntry
	// "f"), while the root's lines use the top-level "source".
	dir := writeProj(t, map[string]string{
		"main.wisp": `include "./lib.wisp" as l` + "\n" +
			`fn main() -> int { return l.one() }`,
		"lib.wisp": `export fn one() -> int { return 1 }`,
	})
	root := filepath.Join(dir, "main.wisp")
	src, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "main.sh")
	var stderr bytes.Buffer
	if code := Build(root, string(src), out, true, &stderr); code != 0 {
		t.Fatalf("build exit %d: %s", code, stderr.String())
	}
	mapBytes, err := os.ReadFile(out + ".map")
	if err != nil {
		t.Fatalf("no source map: %v", err)
	}
	m := string(mapBytes)
	libCanon, _ := filepath.EvalSymlinks(filepath.Join(dir, "lib.wisp"))
	if !strings.Contains(m, "\"f\"") {
		t.Errorf("source map has no per-module file entry (lineEntry.f):\n%s", m)
	}
	if !strings.Contains(m, libCanon) {
		t.Errorf("source map does not reference the included module %q:\n%s", libCanon, m)
	}
}

func TestSingleFileBackwardCompat(t *testing.T) {
	// A plain single-file program with no wisp.json and no directives still builds
	// and runs (behavior unchanged).
	dir := writeProj(t, map[string]string{
		"prog.wisp": `fn main() -> int { print("hello"); return 0 }`,
	})
	out, errb, code := runProj(t, dir, "prog.wisp")
	if code != 0 || out != "hello\n" {
		t.Fatalf("single-file regression: out=%q code=%d stderr=%q", out, code, errb)
	}
}
