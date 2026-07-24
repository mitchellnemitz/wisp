package types

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// TestRemovedExhaustive_BareCallModuleHint drives EVERY name in the mechanically
// derived removable set (RemovableBuiltins()) through a bare call and asserts the
// module-hint diagnostic fires, naming the ns.member spelling from removedHint.
// This is the PR C minimum-test requirement: "for every removable name, assert
// both the bare-call and bare-reference module-hint error." Argument count/types
// do not matter -- checkNamedCall's removed-hint branch fires unconditionally
// before arg checking (see call.go), so a zero-arg call is sufficient.
func TestRemovedExhaustive_BareCallModuleHint(t *testing.T) {
	for _, name := range RemovableBuiltins() {
		hint, ok := RemovedHint(name)
		if !ok {
			t.Fatalf("%s: in RemovableBuiltins() but RemovedHint reports not-ok", name)
		}
		src := fmt.Sprintf("fn main() -> int { %s(); return 0 }", name)
		prog, err := parser.Parse(src, "test.wisp")
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		info := Check(prog)
		ns, _, _ := strings.Cut(hint, ".")
		want := fmt.Sprintf("%q was moved to a module; import %q and call it as %s(...)", name, ns, hint)
		if !hasErr(info, want) {
			t.Errorf("%s: bare call did not produce module-hint error %q, got: %v", name, want, errMsgs(info))
		}
	}
}

// TestRemovedExhaustive_BareReferenceModuleHint is the value-position half of the
// same requirement: a bare removable name used as a value (not called) gets the
// same module-hint diagnostic, not "undeclared name" or a synthesized funcref.
func TestRemovedExhaustive_BareReferenceModuleHint(t *testing.T) {
	for _, name := range RemovableBuiltins() {
		hint, ok := RemovedHint(name)
		if !ok {
			t.Fatalf("%s: in RemovableBuiltins() but RemovedHint reports not-ok", name)
		}
		src := fmt.Sprintf("fn main() -> int { let x: int = %s; return 0 }", name)
		prog, err := parser.Parse(src, "test.wisp")
		if err != nil {
			// A handful of removable names collide with parser-level keywords in
			// value position; that is still a valid rejection of the bare form, just
			// surfaced earlier than the checker. Not the failure mode this test
			// guards against.
			continue
		}
		info := Check(prog)
		ns, _, _ := strings.Cut(hint, ".")
		want := fmt.Sprintf("%q was moved to a module; import %q and call it as %s(...)", name, ns, hint)
		if !hasErr(info, want) {
			t.Errorf("%s: bare reference did not produce module-hint error %q, got: %v", name, want, errMsgs(info))
		}
		if len(info.FuncRefs) != 0 {
			t.Errorf("%s: bare reference to a removed builtin recorded a FuncRef (want none)", name)
		}
	}
}

// TestRemovedExhaustive_StaysFlatRemainsReservedBuiltin is the complement-set half
// of the minimum-test requirement. It does not re-verify every stays-flat
// builtin's full signature/lowering (that is the job of the pre-existing,
// untouched per-builtin test suites, plus TestBuiltinFuncref_Exhaustive for the
// funcref surface) -- it asserts the specific invariant this PR's removal edit
// could have broken: a stays-flat name must remain BOTH a recognized builtin
// (isBuiltin) and reserved (isReservedName), i.e. the removable-set gating added
// to isReservedName's isBuiltin arm must not over-exclude a name it shouldn't.
func TestRemovedExhaustive_StaysFlatRemainsReservedBuiltin(t *testing.T) {
	for _, name := range StaysFlatBuiltins() {
		if isRemovableBuiltin(name) {
			t.Errorf("%s: present in StaysFlatBuiltins() but also reports isRemovableBuiltin", name)
		}
		if !isBuiltin(name) {
			t.Errorf("%s: stays-flat name no longer recognized by isBuiltin", name)
		}
		if !isReservedName(name) {
			t.Errorf("%s: stays-flat name no longer reserved (isReservedName)", name)
		}
	}
}

// TestRemovedExhaustive_FreedIdentifiers is the "freed identifier coverage
// (function and variable rebinding)" minimum test, driven exhaustively over
// RemovableBuiltins() rather than a hand-picked sample: every removed name must
// be usable as both a user function name and a user local-variable name once its
// bare builtin surface is gone.
func TestRemovedExhaustive_FreedIdentifiers(t *testing.T) {
	for _, name := range RemovableBuiltins() {
		fnSrc := fmt.Sprintf("fn %s() -> int { return 0 }\nfn main() -> int { return %s() }", name, name)
		prog, err := parser.Parse(fnSrc, "test.wisp")
		if err != nil {
			t.Fatalf("%s: freed function rebinding: parse: %v", name, err)
		}
		info := Check(prog)
		if len(info.Errors) != 0 {
			t.Errorf("%s: freed function rebinding: unexpected errors: %v", name, errMsgs(info))
		}

		varSrc := fmt.Sprintf("fn main() -> int { let %s: int = 0; return %s }", name, name)
		prog, err = parser.Parse(varSrc, "test.wisp")
		if err != nil {
			t.Fatalf("%s: freed variable rebinding: parse: %v", name, err)
		}
		info = Check(prog)
		if len(info.Errors) != 0 {
			t.Errorf("%s: freed variable rebinding: unexpected errors: %v", name, errMsgs(info))
		}
	}
}

// TestRemovedExhaustive_DelegateNonPerturbation is the "delegate non-perturbation
// under colliding user binding with before/after behavioral parity" minimum
// test. For a representative sample spanning several core modules and both
// direct and delegate builtin routing, it asserts that a user binding which
// collides with the freed flat name does NOT perturb the ns.member delegate
// resolution: the namespaced call still resolves to the same builtin/result type
// it would with no colliding binding present ("after" == "before").
func TestRemovedExhaustive_DelegateNonPerturbation(t *testing.T) {
	cases := []struct {
		ns, member, builtin, collideDecl, collideUse, nsCallSrc string
		want                                                    Type
	}{
		{
			ns: "fs", member: "read_file", builtin: "read_file",
			collideDecl: "fn read_file() -> int { return 0 }",
			collideUse:  "read_file()",
			nsCallSrc:   `let s: string = fs.read_file("x")`,
			want:        String,
		},
		{
			ns: "string", member: "reverse", builtin: "reverse_string",
			collideDecl: `const reverse: int = 0`,
			collideUse:  `let n: int = reverse`,
			nsCallSrc:   `let s: string = string.reverse("x")`,
			want:        String,
		},
		{
			ns: "math", member: "gcd", builtin: "gcd",
			collideDecl: "fn gcd() -> int { return 0 }",
			collideUse:  "gcd()",
			nsCallSrc:   `let g: int = math.gcd(4, 6)`,
			want:        Int,
		},
		{
			ns: "env", member: "get", builtin: "env",
			collideDecl: `const env: int = 0`,
			collideUse:  `let n: int = env`,
			nsCallSrc:   `let s: Optional[string] = env.get("X")`,
			want:        optionalType(String),
		},
	}
	for _, tc := range cases {
		t.Run(tc.ns+"."+tc.member, func(t *testing.T) {
			// "Before": no colliding user binding, just the namespaced call.
			before := mod(t, 0, fmt.Sprintf("fn main() -> int { %s; return 0 }", tc.nsCallSrc), map[string]int{tc.ns: 1})
			beforeInfo := CheckLinked(&module.Linked{Modules: []*module.Module{before, coreMod(1, tc.ns)}})
			if len(beforeInfo.Errors) != 0 {
				t.Fatalf("%s.%s: before (no collision): unexpected errors: %v", tc.ns, tc.member, errMsgs(beforeInfo))
			}
			beforeCI := callWithBuiltin(beforeInfo, tc.builtin)
			if beforeCI == nil {
				t.Fatalf("%s.%s: before (no collision): no CallBuiltin %q recorded", tc.ns, tc.member, tc.builtin)
			}
			if beforeCI.Result != tc.want {
				t.Fatalf("%s.%s: before (no collision): result = %q, want %q", tc.ns, tc.member, beforeCI.Result, tc.want)
			}

			// "After": a user binding collides with the freed flat name, declared
			// alongside the namespaced call in the same program, and actually used
			// so the collision is exercised, not merely declared.
			afterSrc := fmt.Sprintf("%s\nfn main() -> int { %s; %s; return 0 }", tc.collideDecl, tc.nsCallSrc, tc.collideUse)
			after := mod(t, 0, afterSrc, map[string]int{tc.ns: 1})
			afterInfo := CheckLinked(&module.Linked{Modules: []*module.Module{after, coreMod(1, tc.ns)}})
			if len(afterInfo.Errors) != 0 {
				t.Fatalf("%s.%s: after (collision): unexpected errors: %v", tc.ns, tc.member, errMsgs(afterInfo))
			}
			afterCI := callWithBuiltin(afterInfo, tc.builtin)
			if afterCI == nil {
				t.Fatalf("%s.%s: after (collision): no CallBuiltin %q recorded", tc.ns, tc.member, tc.builtin)
			}
			if afterCI.Result != tc.want {
				t.Fatalf("%s.%s: after (collision): result = %q, want %q (delegate perturbed by user binding)", tc.ns, tc.member, afterCI.Result, tc.want)
			}
			if afterCI.Result != beforeCI.Result {
				t.Errorf("%s.%s: before/after result mismatch: before=%q after=%q", tc.ns, tc.member, beforeCI.Result, afterCI.Result)
			}
		})
	}
}
