package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// compileNS compiles rootSrc as "test.wisp" in a Linked module set: the root at
// id 0 with each named core namespace bound to a synthetic core module (id 1..N).
// It is the modules-only analogue of compile() for programs that call namespaced
// members (string.contains, process.pipe, math.pow, ...) whose bare flat spelling
// was removed in PR C and no longer resolves in the single-module check.
//
// The delegate lowering of a namespaced member call is byte-identical to the
// pre-removal flat call (proven by core_byteidentity_test.go and empirically
// re-verified at reconstruction time). A program compiled through this path
// therefore emits exactly the shell a pre-removal flat program emitted, which is
// why the frozen pre-removal *.sh byte-identity snapshots still match a
// namespaced source that is position-aligned to the flat original.
func compileNS(t *testing.T, rootSrc string, namespaces ...string) []byte {
	t.Helper()
	root, err := parser.Parse(rootSrc, "test.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	root0 := &module.Module{ID: 0, Prog: root, Namespaces: map[string]int{}}
	mods := []*module.Module{root0}
	for i, ns := range namespaces {
		id := i + 1
		root0.Namespaces[ns] = id
		mods = append(mods, &module.Module{ID: id, Prog: &ast.Program{}, Namespaces: map[string]int{}, Core: ns})
	}
	linked := &module.Linked{Modules: mods}
	info := types.CheckLinked(linked)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	out, _, err := GenerateLinked(linked, info)
	if err != nil {
		t.Fatalf("GenerateLinked: %v", err)
	}
	return out
}

// runNS compiles (multi-module), shellchecks, and runs rootSrc. Modules-only
// analogue of runWisp for namespaced-member programs.
func runNS(t *testing.T, rootSrc string, namespaces ...string) (string, string, int) {
	t.Helper()
	script := compileNS(t, rootSrc, namespaces...)
	shellcheck(t, script)
	return run(t, script)
}

// runArgsNS compiles (multi-module), shellchecks, and runs rootSrc under dash
// with argv. Modules-only analogue of runArgs for namespaced-member programs
// whose behavior is driven by main's argv.
func runArgsNS(t *testing.T, rootSrc string, argv []string, namespaces ...string) (string, string, int) {
	t.Helper()
	script := compileNS(t, rootSrc, namespaces...)
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

// runNSDir compiles (multi-module), shellchecks, and runs rootSrc under dash with
// the given extra environment variables and a fresh working directory.
// Modules-only analogue of runWispDir for namespaced-member programs whose
// behavior depends on a per-run environment variable (which the golden harness
// cannot provide per fixture).
func runNSDir(t *testing.T, rootSrc string, env []string, namespaces ...string) (string, string, int) {
	t.Helper()
	script := compileNS(t, rootSrc, namespaces...)
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
	cmd := exec.Command(dash, path)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
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
