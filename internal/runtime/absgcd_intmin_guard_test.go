package runtime

// TestAbsGcdIntMinRuntimeGuard exercises the abs() and gcd() INT_MIN overflow
// guards directly at the prelude level (the carve-out for spec constructs
// #12/#13). The cross-shell runtime-injection method used for the other
// constructs cannot apply to INT_MIN because zsh cannot parse it as a literal
// nor do 64-bit arithmetic on it (spec 2.4); the prelude guards detect INT_MIN
// via the doubling model, which works on dash/bash/busybox. This test confirms
// the runtime side of the abs/gcd agreement; the compile side is the Task 3
// checker reject test. No prior test exercised these guards (verified absent).

import (
	"strings"
	"testing"
)

const intMinLit = "-9223372036854775808"

func TestAbsGcdIntMinRuntimeGuard(t *testing.T) {
	const intMaxLit = "9223372036854775807" // INT_MIN+1 = -(INT_MAX); abs of it is INT_MAX
	// abs(INT_MIN) aborts; abs(INT_MIN+1) accepts and returns INT_MAX (AC7 accept path).
	t.Run("abs", func(t *testing.T) {
		_, errb, code := runSnippet(t, []string{AbsInt},
			`__wisp_abs_int "p:1:1" "`+intMinLit+`"; printf '%s\n' "$__ret"`)
		if code == 0 {
			t.Fatalf("abs(INT_MIN): exit 0, want non-zero (overflow guard)")
		}
		if !strings.Contains(errb, "abs(): integer overflow") {
			t.Fatalf("abs(INT_MIN): stderr %q lacks overflow message", errb)
		}
		out, errb2, code2 := runSnippet(t, []string{AbsInt},
			`__wisp_abs_int "p:1:1" "-`+intMaxLit+`"; printf '%s\n' "$__ret"`)
		if code2 != 0 {
			t.Fatalf("abs(INT_MIN+1): exit %d stderr %q, want 0 (accept path)", code2, errb2)
		}
		if strings.TrimSpace(out) != intMaxLit {
			t.Fatalf("abs(INT_MIN+1) = %q, want %q", out, intMaxLit)
		}
	})
	// gcd(INT_MIN, n) and gcd(n, INT_MIN) both abort; gcd(INT_MIN+1, 6) accepts (AC7).
	t.Run("gcd", func(t *testing.T) {
		for _, args := range []string{`"` + intMinLit + `" "6"`, `"6" "` + intMinLit + `"`} {
			_, errb, code := runSnippet(t, []string{Gcd},
				`__wisp_gcd "p:1:1" `+args+`; printf '%s\n' "$__ret"`)
			if code == 0 {
				t.Fatalf("gcd(%s): exit 0, want non-zero (overflow guard)", args)
			}
			if !strings.Contains(errb, "gcd(): integer overflow") {
				t.Fatalf("gcd(%s): stderr %q lacks overflow message", args, errb)
			}
		}
		// gcd(INT_MIN+1, 6) = gcd(9223372036854775807, 6) = 1: clean exit AND correct value.
		outA, errbA, codeA := runSnippet(t, []string{Gcd},
			`__wisp_gcd "p:1:1" "-`+intMaxLit+`" "6"; printf '%s\n' "$__ret"`)
		if codeA != 0 {
			t.Fatalf("gcd(INT_MIN+1, 6): exit %d stderr %q, want 0 (accept path)", codeA, errbA)
		}
		if strings.TrimSpace(outA) != "1" {
			t.Fatalf("gcd(INT_MIN+1, 6) = %q, want \"1\"", outA)
		}
	})
}

// TestWaitAnyPollRuntimeGuard is the runtime side of the wait_any (#8) carve-out
// (spec Section 3.1). __wisp_wait_any checks the empty-list precondition before the
// poll<0 guard, so the test supplies a NON-EMPTY fake array (one element in state
// "done") to isolate the poll guard. poll=-1 must abort with the poll message;
// poll=0 must return the done element without aborting.
func TestWaitAnyPollRuntimeGuard(t *testing.T) {
	// Fake a one-element Process[] array (id "t") whose single process p1 is done.
	const fake = `__wisp_a_t_len=1
__wisp_a_t_0=p1
__wisp_s_p1_state=done
__wisp_s_p1_done=/dev/null
`
	// poll = -1 -> aborts (empty-list bypassed because len=1).
	_, errb, code := runSnippet(t, []string{WaitAny}, fake+`__wisp_wait_any "p:1:1" "t" "-1"; printf '%s\n' "$__ret"`)
	if code == 0 {
		t.Fatalf("wait_any poll=-1: exit 0, want non-zero (poll guard)")
	}
	if !strings.Contains(errb, "wait_any: poll_secs must be >= 0") {
		t.Fatalf("wait_any poll=-1: stderr %q lacks poll message", errb)
	}
	// poll = 0 -> returns the done element, no abort.
	out, errb0, code0 := runSnippet(t, []string{WaitAny}, fake+`__wisp_wait_any "p:1:1" "t" "0"; printf '%s\n' "$__ret"`)
	if code0 != 0 {
		t.Fatalf("wait_any poll=0 on a done process: exit %d stderr %q, want 0", code0, errb0)
	}
	if strings.TrimSpace(out) != "p1" {
		t.Fatalf("wait_any poll=0: __ret = %q, want \"p1\"", out)
	}
}
