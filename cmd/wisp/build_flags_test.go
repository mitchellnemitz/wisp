package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// L3: `wisp build --bogus` must report an unknown flag (exit 2), not treat the
// flag as a filename. This matches check/run/fmt.
func TestBuildUnknownFlagExit2(t *testing.T) {
	var so, se bytes.Buffer
	code := run([]string{"build", "--bogus"}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if !strings.Contains(se.String(), "unknown flag") {
		t.Errorf("expected 'unknown flag' on stderr, got %q", se.String())
	}
}

// L5: `wisp build -o --source-map` must not create a file literally named
// "--source-map"; -o must reject a following flag-shaped token (exit 2).
func TestBuildDashOFlagShapedArgExit2(t *testing.T) {
	src := writeTmp(t, "prog.wisp", hello)
	var so, se bytes.Buffer
	code := run([]string{"build", src, "-o", "--source-map"}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2, stderr=%q", code, se.String())
	}
	if _, err := os.Stat("--source-map"); err == nil {
		os.Remove("--source-map")
		t.Fatalf("a file named --source-map was created")
	}
}

// H7 at the CLI: `wisp build foo.sh` (default out == source) must exit 2 and
// leave the source untouched.
func TestBuildOverSourceCLIExit2(t *testing.T) {
	src := writeTmp(t, "foo.sh", hello)
	var so, se bytes.Buffer
	code := run([]string{"build", src}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2, stderr=%q", code, se.String())
	}
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != hello {
		t.Fatalf("source overwritten: %q", string(b))
	}
}

// H7 at the CLI via explicit -o equal to the source must also refuse.
func TestBuildExplicitOutEqualsSourceExit2(t *testing.T) {
	src := writeTmp(t, "prog.wisp", hello)
	var so, se bytes.Buffer
	code := run([]string{"build", src, "-o", src}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2, stderr=%q", code, se.String())
	}
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != hello {
		t.Fatalf("source overwritten: %q", string(b))
	}
}
