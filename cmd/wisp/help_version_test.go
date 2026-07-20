package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/version"
)

func TestVersionSpellings(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"version"}} {
		var out, errOut bytes.Buffer
		code := run(args, &out, &errOut)
		if code != 0 {
			t.Fatalf("%v: exit=%d want 0", args, code)
		}
		want := "wisp " + version.Number + "\n"
		if out.String() != want {
			t.Errorf("%v: out=%q want %q", args, out.String(), want)
		}
		if errOut.String() != "" {
			t.Errorf("%v: errOut=%q want empty", args, errOut.String())
		}
	}
}

func TestHelpTopLevel(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		var out, errOut bytes.Buffer
		code := run(args, &out, &errOut)
		if code != 0 {
			t.Fatalf("%v: exit=%d want 0", args, code)
		}
		if out.String() != usage {
			t.Errorf("%v: out=%q want %q", args, out.String(), usage)
		}
		if errOut.String() != "" {
			t.Errorf("%v: errOut=%q want empty", args, errOut.String())
		}
	}
}

func TestHelpPerCommand(t *testing.T) {
	summary := "usage: wisp <build|run|check|fmt|test|doc|add|install|remove> [args] (per-command syntax below)\n"
	expected := map[string]string{
		"build":   summary + "  build <src> [-o <out>] [--source-map]   compile to a .sh script (default out: src with .sh ext)\n",
		"run":     summary + "  run <src> [args...]                     compile and execute, propagating its exit status\n",
		"check":   summary + "  check <src>                             type-check only; report diagnostics\n",
		"fmt":     summary + "  fmt <path>... [-w | --check]            canonically format files/dirs; -w rewrites in place, --check verifies\n",
		"test":    summary + "  test [path] [--tap] [--filter <re>] [--shell <name>] [--coverage]   run *_test.wisp files cross-shell\n",
		"doc":     summary + "  doc <path>...                           extract /// doc-comments to Markdown\n",
		"add":     summary + "  add <owner/repo> [--git <url>] [--ref <ref>]   add/repin a dependency, then install\n",
		"install": summary + "  install                                 fetch/verify dependencies from wisp.json + wisp.lock\n",
		"remove":  summary + "  remove <owner/repo>                     drop a dependency, prune its module tree, update the lock\n",
	}

	commands := []string{"build", "run", "check", "fmt", "test", "doc", "add", "install", "remove"}
	if len(expected) != len(commands) {
		t.Fatalf("expected table has %d entries, want %d", len(expected), len(commands))
	}

	for _, cmd := range commands {
		want, ok := expected[cmd]
		if !ok {
			t.Fatalf("no expected help literal for command %q", cmd)
		}
		for _, args := range [][]string{{"help", cmd}, {cmd, "--help"}, {cmd, "-h"}} {
			var out, errOut bytes.Buffer
			code := run(args, &out, &errOut)
			if code != 0 {
				t.Fatalf("%v: exit=%d want 0", args, code)
			}
			if errOut.String() != "" {
				t.Errorf("%v: errOut=%q want empty", args, errOut.String())
			}
			if out.String() != want {
				t.Errorf("%v: out=%q want %q", args, out.String(), want)
			}
			for other, otherWant := range expected {
				if other == cmd {
					continue
				}
				otherLine := strings.TrimPrefix(otherWant, summary)
				if strings.Contains(out.String(), otherLine) {
					t.Errorf("%v: out contains other command %q's line", args, other)
				}
			}
		}
	}
}

func TestHelpUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"help", "frobnicate"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if out.String() != "" {
		t.Errorf("out=%q want empty", out.String())
	}
	if !strings.Contains(errOut.String(), `unknown command "frobnicate"`) {
		t.Errorf("errOut=%q want to contain unknown command message", errOut.String())
	}

	out.Reset()
	errOut.Reset()
	code = run([]string{"frobnicate", "--help"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if out.String() != "" {
		t.Errorf("out=%q want empty", out.String())
	}
}

func TestUnknownCommandPlain(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"frobnicate"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if out.String() != "" {
		t.Errorf("out=%q want empty", out.String())
	}
	want := "wisp: unknown command \"frobnicate\"\n" + usage
	if errOut.String() != want {
		t.Errorf("errOut=%q want %q", errOut.String(), want)
	}
}

func TestDashVNotVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"-v"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if out.String() != "" {
		t.Errorf("out=%q want empty", out.String())
	}
}

func TestNoArgsUnchanged(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(nil, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	want := "wisp " + version.Number + "\n" + usage
	if errOut.String() != want {
		t.Errorf("errOut=%q want %q", errOut.String(), want)
	}
	if out.String() != "" {
		t.Errorf("out=%q want empty", out.String())
	}
}

func TestUnknownFlagUnchanged(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"build", "--nope"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if out.String() != "" {
		t.Errorf("out=%q want empty", out.String())
	}
	want := "wisp: unknown flag \"--nope\"\n" + usage
	if errOut.String() != want {
		t.Errorf("errOut=%q want %q", errOut.String(), want)
	}
}

func TestCompileErrorExitUnchanged(t *testing.T) {
	src := writeTmp(t, "bad.wisp", "fn main() -> int { return \"x\" }\n")
	var out, errOut bytes.Buffer
	code := run([]string{"check", src}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	if out.String() != "" {
		t.Errorf("out=%q want empty", out.String())
	}
	if errOut.String() == "" {
		t.Errorf("errOut want non-empty diagnostic")
	}
}
