package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTmp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const hello = "fn main() -> int {\n  print(\"hi\")\n  return 0\n}\n"

func TestNoArgsExit2(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, &out, &errOut); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if !strings.Contains(errOut.String(), "usage") {
		t.Errorf("expected usage on stderr, got %q", errOut.String())
	}
}

func TestUnknownCommandExit2(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"frobnicate", "x.wisp"}, &out, &errOut); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestBuildDefaultOut(t *testing.T) {
	src := writeTmp(t, "prog.wisp", hello)
	var out, errOut bytes.Buffer
	code := run([]string{"build", src}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
	def := strings.TrimSuffix(src, ".wisp") + ".sh"
	if _, err := os.Stat(def); err != nil {
		t.Fatalf("default output not written: %v", err)
	}
}

func TestBuildExplicitOut(t *testing.T) {
	src := writeTmp(t, "prog.wisp", hello)
	out := filepath.Join(t.TempDir(), "custom.sh")
	var so, se bytes.Buffer
	code := run([]string{"build", src, "-o", out}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("explicit output not written: %v", err)
	}
}

func TestBuildMissingFileExit2(t *testing.T) {
	var so, se bytes.Buffer
	code := run([]string{"build"}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestCheckOK(t *testing.T) {
	src := writeTmp(t, "ok.wisp", hello)
	var so, se bytes.Buffer
	if code := run([]string{"check", src}, &so, &se); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
}

func TestCheckError(t *testing.T) {
	src := writeTmp(t, "bad.wisp", "fn main() -> int { return \"x\" }\n")
	var so, se bytes.Buffer
	if code := run([]string{"check", src}, &so, &se); code == 0 {
		t.Fatal("expected non-zero on type error")
	}
}

func TestRunPropagatesExit(t *testing.T) {
	src := writeTmp(t, "exit.wisp", "fn main() -> int {\n  return 5\n}\n")
	var so, se bytes.Buffer
	if code := run([]string{"run", src}, &so, &se); code != 5 {
		t.Fatalf("exit=%d want 5 stderr=%q", code, se.String())
	}
}

func TestNonexistentSourceExit2(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"check", "/no/such/file.wisp"}, &so, &se); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestBuildSourceMapFlagWritesBoth(t *testing.T) {
	src := writeTmp(t, "prog.wisp", hello)
	out := filepath.Join(t.TempDir(), "custom.sh")
	var so, se bytes.Buffer
	code := run([]string{"build", src, "-o", out, "--source-map"}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf(".sh not written: %v", err)
	}
	if _, err := os.Stat(out + ".map"); err != nil {
		t.Fatalf(".sh.map not written: %v", err)
	}
}

func TestBuildNoSourceMapFlagWritesOnlySh(t *testing.T) {
	src := writeTmp(t, "prog.wisp", hello)
	out := filepath.Join(t.TempDir(), "custom.sh")
	var so, se bytes.Buffer
	if code := run([]string{"build", src, "-o", out}, &so, &se); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	if _, err := os.Stat(out + ".map"); !os.IsNotExist(err) {
		t.Fatalf(".sh.map should not exist without --source-map: %v", err)
	}
}

// run and check do not define --source-map; using it there is a usage error
// (exit 2), per the plan (no new check milestone work).
func TestRunSourceMapFlagUsageError(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"run", "--source-map"}, &so, &se); code != 2 {
		t.Fatalf("run --source-map exit=%d, want 2", code)
	}
}

func TestCheckSourceMapFlagUsageError(t *testing.T) {
	src := writeTmp(t, "ok.wisp", hello)
	var so, se bytes.Buffer
	// As a flag in the source position.
	if code := run([]string{"check", "--source-map"}, &so, &se); code != 2 {
		t.Fatalf("check --source-map exit=%d, want 2", code)
	}
	// As an extra arg alongside the source.
	if code := run([]string{"check", src, "--source-map"}, &so, &se); code != 2 {
		t.Fatalf("check src --source-map exit=%d, want 2", code)
	}
}
