package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
)

const helloSrc = `fn main() -> int {
  print("hello")
  return 0
}
`

func TestCompileHelloProducesScriptNoDiags(t *testing.T) {
	script, _, diags := Compile("hello.wisp", helloSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(script) == 0 {
		t.Fatal("expected non-empty script")
	}
	if !bytes.HasPrefix(script, []byte("#!/bin/sh\n")) {
		t.Fatalf("script missing shebang, got: %q", script[:min(20, len(script))])
	}
}

func TestCompileTypeErrorNoScriptWithPosition(t *testing.T) {
	// 1 == "1" is a type error (no implicit coercion).
	src := `fn main() -> int {
  if (1 == "1") { return 1 }
  return 0
}
`
	script, _, diags := Compile("bad.wisp", src)
	if script != nil {
		t.Fatalf("expected nil script on error, got %q", script)
	}
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic")
	}
	var sawError bool
	for _, d := range diags {
		if d.Severity == Error {
			sawError = true
			if d.Pos.Line == 0 {
				t.Errorf("diagnostic missing position: %v", d)
			}
		}
	}
	if !sawError {
		t.Fatalf("expected an Error severity diagnostic, got %v", diags)
	}
}

func TestCompileParseErrorNoScript(t *testing.T) {
	src := `fn main() -> int {
  return
` // missing closing brace / value
	script, _, diags := Compile("p.wisp", src)
	if script != nil {
		t.Fatal("expected nil script on parse error")
	}
	if len(diags) == 0 {
		t.Fatal("expected a parse diagnostic")
	}
	if diags[0].Severity != Error {
		t.Fatalf("parse diagnostic should be Error, got %v", diags[0])
	}
}

func TestCompileWarningStillProducesScript(t *testing.T) {
	src := `fn main() -> int {
  let unused: int = 5
  return 0
}
`
	script, _, diags := Compile("w.wisp", src)
	if len(script) == 0 {
		t.Fatal("warning-only program must still compile")
	}
	var sawWarn bool
	for _, d := range diags {
		if d.Severity == Warning {
			sawWarn = true
		}
		if d.Severity == Error {
			t.Fatalf("unexpected error diagnostic: %v", d)
		}
	}
	if !sawWarn {
		t.Fatalf("expected a warning diagnostic, got %v", diags)
	}
}

func TestDiagnosticString(t *testing.T) {
	d := Diagnostic{
		Pos:      token.Position{File: "f.wisp", Line: 3, Col: 7},
		Severity: Error,
		Msg:      "boom",
	}
	got := d.String()
	want := "f.wisp:3:7: boom"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCheckExitCodes(t *testing.T) {
	var out bytes.Buffer
	// warning-only -> 0
	code := Check("w.wisp", "fn main() -> int {\n let u: int = 1\n return 0\n}\n", &out)
	if code != 0 {
		t.Errorf("warning-only check exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "w.wisp") {
		t.Errorf("warning not written to stderr buffer: %q", out.String())
	}
	out.Reset()
	// error -> non-zero
	code = Check("b.wisp", `fn main() -> int { return "x" }`, &out)
	if code == 0 {
		t.Error("error check exit = 0, want non-zero")
	}
}

func TestBuildWritesFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hello.sh")
	var stderr bytes.Buffer
	code := Build("hello.wisp", helloSrc, out, false, &stderr)
	if code != 0 {
		t.Fatalf("build exit = %d, stderr=%q", code, stderr.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("#!/bin/sh")) {
		t.Fatalf("built file missing shebang")
	}
	info, _ := os.Stat(out)
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("built file not executable: %v", info.Mode())
	}
}

func TestBuildErrorWritesNoFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bad.sh")
	var stderr bytes.Buffer
	code := Build("bad.wisp", `fn main() -> int { return "x" }`, out, false, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit on error")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("output file should not exist on error: %v", err)
	}
	if stderr.Len() == 0 {
		t.Error("expected diagnostics on stderr")
	}
}

func TestRunPropagatesExitAndStdout(t *testing.T) {
	src := `fn main() -> int {
  print("out-marker")
  return 7
}
`
	var stdout, stderr bytes.Buffer
	code := Run("e.wisp", src, nil, &stdout, &stderr)
	if code != 7 {
		t.Fatalf("run exit = %d, want 7; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "out-marker") {
		t.Fatalf("missing stdout marker: %q", stdout.String())
	}
}

func TestRunFailFastExit1(t *testing.T) {
	src := `fn main() -> int {
  let z: int = 0
  return 1 / z
}
`
	var stdout, stderr bytes.Buffer
	code := Run("z.wisp", src, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("div-by-zero run exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "division by zero") {
		t.Fatalf("missing abort message: %q", stderr.String())
	}
}

func TestRunCompileErrorNoExec(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run("b.wisp", `fn main() -> int { return "x" }`, nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero on compile error")
	}
	if stdout.Len() != 0 {
		t.Errorf("no stdout expected on compile error, got %q", stdout.String())
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
