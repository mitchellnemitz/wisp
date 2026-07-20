package runtime

import (
	"strings"
	"testing"
)

// M6 PR-A: isolated runtime tests for the core stdlib helpers. These exercise
// the prelude snippets directly (the codegen package has the compile-and-run
// behavioral coverage); here we pin literal/injection semantics, the located
// aborts, and the error-mode short-circuit (split/repeat set pending at depth>0
// instead of exiting).

func TestM6_Contains_Helper(t *testing.T) {
	cases := []struct {
		s, sub, want string
	}{
		{"hello", "ell", "true"},
		{"hello", "xyz", "false"},
		{"hello", "", "true"},
		{"hello", "hello", "true"},
		{"hello", "lo", "true"},
		{"", "x", "false"},
		{"", "", "true"},
	}
	for _, c := range cases {
		out, _, code := runSnippet(t, []string{Contains},
			`__wisp_contains `+sq(c.s)+` `+sq(c.sub)+`; printf '%s' "$__ret"`)
		if code != 0 {
			t.Fatalf("contains(%q,%q): exit %d", c.s, c.sub, code)
		}
		if out != c.want {
			t.Errorf("contains(%q,%q) = %q, want %q", c.s, c.sub, out, c.want)
		}
	}
}

func TestM6_IndexOf_Helper(t *testing.T) {
	cases := []struct {
		s, sub, want string
	}{
		{"hello", "l", "2"},
		{"hello", "lo", "3"},
		{"hello", "z", "-1"},
		{"hello", "", "0"},
		{"hello", "h", "0"},
		{"hello", "o", "4"},
	}
	for _, c := range cases {
		out, _, code := runSnippet(t, []string{IndexOf},
			`__wisp_index_of `+sq(c.s)+` `+sq(c.sub)+`; printf '%s' "$__ret"`)
		if code != 0 {
			t.Fatalf("index_of(%q,%q): exit %d", c.s, c.sub, code)
		}
		if out != c.want {
			t.Errorf("index_of(%q,%q) = %q, want %q", c.s, c.sub, out, c.want)
		}
	}
}

func TestM6_StartsEndsWith_Helper(t *testing.T) {
	out, _, code := runSnippet(t, []string{StartsWith, EndsWith},
		`__wisp_starts_with 'hello' 'he'; printf '%s,' "$__ret"
__wisp_starts_with 'hello' 'lo'; printf '%s,' "$__ret"
__wisp_starts_with 'hello' ''; printf '%s,' "$__ret"
__wisp_ends_with 'hello' 'lo'; printf '%s,' "$__ret"
__wisp_ends_with 'hello' 'he'; printf '%s,' "$__ret"
__wisp_ends_with 'hello' ''; printf '%s' "$__ret"`)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "true,false,true,true,false,true" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Repeat_Helper(t *testing.T) {
	out, _, code := runSnippet(t, []string{Repeat},
		`__wisp_repeat 'p:1:1' 'ab' 3; printf '%s,' "$__ret"
__wisp_repeat 'p:1:1' 'x' 0; printf '[%s]' "$__ret"`)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "ababab,[]" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Repeat_NegativeAborts(t *testing.T) {
	_, errb, code := runSnippet(t, []string{Repeat},
		`__wisp_repeat 'p:2:3' 'x' -1`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errb, "p:2:3") || !strings.Contains(errb, "repeat") {
		t.Errorf("stderr=%q, want located repeat abort", errb)
	}
}

// Literal/inert: a separator/search of shell-active bytes is matched literally.
func TestM6_Contains_LiteralMetachars(t *testing.T) {
	out, _, code := runSnippet(t, []string{Contains},
		`__wisp_contains 'a*b' '*'; printf '%s,' "$__ret"
__wisp_contains 'axb' '*'; printf '%s' "$__ret"`)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// '*' is matched literally: present in 'a*b', absent in 'axb'.
	if out != "true,false" {
		t.Errorf("out=%q (glob leaked?)", out)
	}
}

func TestM6_IndexOf_InertCommandSub(t *testing.T) {
	// A command-substitution-looking subject is inert data.
	out, _, code := runSnippet(t, []string{IndexOf},
		`__wisp_index_of '$(echo X)abc' 'abc'; printf '%s' "$__ret"`)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "9" {
		t.Errorf("out=%q (re-evaluation?)", out)
	}
}

// Error mode: at depth>0, the empty-separator/negative-count aborts set pending
// and RETURN (no exit), so a catch can observe them.
func TestM6_Split_EmptySep_ShortCircuits(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{Split},
		`__wisp_try_depth=1; __wisp_split 'p:1:1' '7' 'abc' ''; printf 'after\n'; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (should not exit at depth>0)", code)
	}
	if !strings.Contains(out, "after\n") || !strings.Contains(out, "pending=1") {
		t.Errorf("out=%q, expected after+pending set", out)
	}
	if !strings.Contains(out, "split") {
		t.Errorf("out=%q, expected split message", out)
	}
}

func TestM6_Repeat_Negative_ShortCircuits(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{Repeat},
		`__wisp_try_depth=1; __wisp_repeat 'p:1:1' 'x' -1; printf 'after\n'; printf 'pending=%s\n' "$__wisp_err_pending"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "after\n") || !strings.Contains(out, "pending=1") {
		t.Errorf("out=%q", out)
	}
}

// Non-error mode strips the error-mode short-circuit guard lines (zero overhead).
func TestM6_NonErrorMode_StripsShortCircuits(t *testing.T) {
	for _, id := range []string{Split, Repeat} {
		src := Emit([]string{id})
		if strings.Contains(src, `[ -n "$__wisp_err_pending" ] && return`) {
			t.Errorf("%s: short-circuit guard line not stripped in non-error mode", id)
		}
	}
}

// sq single-quotes a Go string into a POSIX-sh literal word for the driver.
func sq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
