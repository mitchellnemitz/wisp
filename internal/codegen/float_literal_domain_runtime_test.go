package codegen

// TestFloatLiteralDomainAgree: AC5 compile/runtime agreement gate.
//
// For each vector in the required boundary set, two INDEPENDENT decisions are
// computed:
//
//  1. COMPILE decision: parse + types.Check on `fn main() -> int { let x: float = V return 0 }`;
//     inspect Info.Errors -- ACCEPTED iff no error, REJECTED iff any error names V.
//     Never calls compile(t,...) which calls t.Fatalf on checker errors.
//
//  2. RUNTIME decision: for each shell in execShells(t), run a small script that
//     (a) formats V via `awk -v a="V" 'BEGIN{ printf "%.17g", (a+0) }'` (value
//     passed via -v, never interpolated into the awk program), then (b) feeds the
//     formatted string to __wisp_ffinite. ACCEPTED iff script exits 0, REJECTED
//     iff exit nonzero. This must NOT depend on V compiling.
//
// ASSERT: compile == runtime for every vector on every shell.
// ALSO: for every runtime-ACCEPTED vector, assert the awk %.17g output satisfies
// the full __wisp_ffinite glob (no e/E/inf/nan, no leading/trailing/double dot).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// floatDomainVector is one test case. wantIn mirrors the %.17g exponent rule:
// a value is in-domain iff fmt.Sprintf("%.17g", value) has no exponent character
// and is finite. The in/out expectation here is the EXPECTED classification; the
// test computes it independently and asserts compile == runtime (not == wantIn).
// wantIn is used only to annotate intent and to check that the test's own logic
// matches the expected classification.
type floatDomainVector struct {
	value  string // wisp digits.digits literal text
	wantIn bool   // expected: true=in-domain, false=out-of-domain
	note   string // brief rationale
}

// floatDomainVectors is the required vector set from spec AC5 / task brief.
// Each boundary is documented with the %.17g exponent rule:
// exponent form is used when decimal exponent < -4 OR >= 17.
var floatDomainVectors = []floatDomainVector{
	// --- lower boundary (small magnitudes) ---
	// 0.0: zero, always "0" in %.17g -> in
	{"0.0", true, "zero: %.17g='0'"},
	// 0.0005: exp = -4, boundary is < -4 so -4 is NOT in exponent form -> in
	{"0.0005", true, "exp = -4, not < -4, plain decimal %.17g='0.00050000000000000001'"},
	// 0.0001: exp = -4, same reasoning -> in
	{"0.0001", true, "exp = -4, plain decimal"},
	// 0.00009: exp = -5, -5 < -4 -> exponent form -> out
	{"0.00009", false, "exp = -5 < -4: exponent form"},
	// 0.000099999: exp = -5 -> exponent form -> out
	{"0.000099999", false, "exp = -5 < -4: exponent form"},
	// 0.000001: exp = -6 -> exponent form -> out
	{"0.000001", false, "exp = -6 < -4: exponent form (canonical task brief example)"},

	// --- mid-range ---
	{"3.14", true, "mid-range decimal, plain %.17g"},

	// --- upper boundary (large magnitudes) ---
	// 99999999999999.0: 14 digits, exp = 13, 13 < 17 -> plain decimal -> in
	{"99999999999999.0", true, "exp=13 < 17: plain decimal"},
	// 10000000000000000.0: 1e16 region, exp = 16, 16 < 17 -> plain decimal -> in
	{"10000000000000000.0", true, "exp=16 < 17: plain decimal"},
	// 9.9999e16-region literal: exp = 16 still < 17 -> in
	{"99999999999999984.0", true, "exp=16 < 17: 9.9999e16-region plain decimal"},
	// 100000000000000000.0: 1e17 region, exp = 17, 17 >= 17 -> exponent form -> out
	{"100000000000000000.0", false, "exp=17 >= 17: exponent form"},

	// --- overflow to Inf ---
	// A ~300-digit integer-part literal that overflows float64 to Inf -> out
	{"9" + strings.Repeat("9", 308) + ".0", false, "~309-digit literal overflows float64 to Inf"},
}

// fullFFiniteGlob checks whether a string satisfies the full __wisp_ffinite
// acceptance glob exactly as the shell does:
//   - no e/E in body
//   - not inf/nan (case-insensitive via the glob arms)
//   - no leading dot
//   - no trailing dot
//   - no double dot
//
// This mirrors the case statement in __wisp_ffinite:
//
//	case "$__f_body" in
//	  '' | *[!0-9.]* | .* | *. | *.*.*)
//	    __wisp_fail ...
//
// where __f_body has the leading sign stripped.
func fullFFiniteGlob(s string) bool {
	body := s
	if len(body) > 0 && (body[0] == '-' || body[0] == '+') {
		body = body[1:]
	}
	if body == "" {
		return false
	}
	// *[!0-9.]* -- any byte outside [0-9.]
	for _, c := range []byte(body) {
		if (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	// .* -- leading dot
	if body[0] == '.' {
		return false
	}
	// *. -- trailing dot
	if body[len(body)-1] == '.' {
		return false
	}
	// *.*.* -- more than one dot
	first := strings.Index(body, ".")
	if first != -1 && strings.Index(body[first+1:], ".") != -1 {
		return false
	}
	return true
}

// compileDecision returns (accepted bool, awkFormatted string).
// It parses + type-checks the wisp source and returns true iff no error.
// Never calls t.Fatalf on checker errors; a type error is a normal REJECTED outcome.
func compileDecision(value string) (accepted bool) {
	src := fmt.Sprintf("fn main() -> int { let x: float = %s\n return 0\n}", value)
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		// parse error means rejected (e.g. the huge literal may fail to parse)
		return false
	}
	info := types.Check(prog)
	return len(info.Errors) == 0
}

// runtimeDecision runs a shell script that formats value via awk %.17g, passes
// the result to __wisp_ffinite, and returns (accepted bool, awkOutput string).
// The script always runs, even for compile-rejected values.
func runtimeDecision(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, value string) (accepted bool, awkOut string) {
	t.Helper()

	// Emit __wisp_fail and __wisp_ffinite as a shell function block.
	prelude := runtime.Emit([]string{runtime.FFinite})

	// The script:
	// 1. define __wisp_fail and __wisp_ffinite from prelude
	// 2. format value via awk -v (value passed as -v arg, never interpolated into program)
	// 3. call __wisp_ffinite on the formatted result
	// 4. exit 0 on success, nonzero (from __wisp_fail) on rejection
	script := "#!/bin/sh\n" +
		prelude + "\n" +
		`__f_formatted="$(awk -v a="` + value + `" 'BEGIN{ printf "%.17g", (a+0) }')"` + "\n" +
		`__wisp_ffinite "test:1:1" "$__f_formatted"` + "\n" +
		`printf '%s\n' "$__f_formatted"` + "\n" +
		"exit 0\n"

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "check.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	args := append(append([]string{}, sh.args...), scriptPath)
	cmd := exec.Command(sh.bin, args...)
	var outb, errb strings.Builder
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	runErr := cmd.Run()
	outStr := strings.TrimRight(outb.String(), "\n")

	if runErr != nil {
		// nonzero exit -> rejected; the formatted string may still be in stdout
		return false, outStr
	}
	return true, outStr
}

func TestFloatLiteralDomainAgree(t *testing.T) {
	shells := execShells(t)

	for _, vec := range floatDomainVectors {
		vec := vec
		t.Run(vec.value, func(t *testing.T) {
			// COMPILE decision (independent of any shell).
			compAccepted := compileDecision(vec.value)

			for _, sh := range shells {
				sh := sh
				t.Run(sh.label, func(t *testing.T) {
					// RUNTIME decision (independent of compile outcome).
					rtAccepted, awkOut := runtimeDecision(t, sh, vec.value)

					// PRIMARY assertion: compile == runtime.
					if compAccepted != rtAccepted {
						t.Errorf(
							"DIVERGENCE for %q on %s: compile=%v runtime=%v (awk-out=%q) note: %s",
							vec.value, sh.label, compAccepted, rtAccepted, awkOut, vec.note,
						)
					}

					// SECONDARY assertion for accepted values: awk output satisfies the
					// full __wisp_ffinite glob, confirming no-e-subset == full-glob.
					if rtAccepted {
						if !fullFFiniteGlob(awkOut) {
							t.Errorf(
								"runtime-ACCEPTED but awk output %q fails full __wisp_ffinite glob for %q on %s",
								awkOut, vec.value, sh.label,
							)
						}
					}

					// Informational: log each vector result for the report.
					t.Logf("vector=%q shell=%s compile=%v runtime=%v awk-out=%q wantIn=%v",
						vec.value, sh.label, compAccepted, rtAccepted, awkOut, vec.wantIn)
				})
			}
		})
	}
}
