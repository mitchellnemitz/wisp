package types

import (
	"regexp"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// TestCoreFuncMembersAreLowercase enforces the naming convention the editor
// grammars' namespaced-member matching (F1=A) relies on: every callable
// (coreFunc) member key in coreCatalog is lowercase snake_case, matching
// TestBuiltinsAreSnakeCase's pattern for flat builtins. coreType/coreConst
// members are exempt -- json.Value is a type, not a callable.
func TestCoreFuncMembersAreLowercase(t *testing.T) {
	snake := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for ns, members := range coreCatalog {
		for name, m := range members {
			if m.kind != coreFunc {
				continue
			}
			if !snake.MatchString(name) {
				t.Errorf("coreCatalog[%q][%q] is a coreFunc member but not lowercase snake_case", ns, name)
			}
		}
	}
}

func TestIsCoreModule(t *testing.T) {
	for _, name := range []string{"array", "dict", "env", "fs", "json", "math", "process", "regex", "string"} {
		if !isCoreModule(name) {
			t.Errorf("isCoreModule(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"mathh", "not_a_module", ""} {
		if isCoreModule(name) {
			t.Errorf("isCoreModule(%q) = true, want false", name)
		}
	}
}

// TestModuleNotImportedHint drives all 8 non-env core modules (env is excluded:
// it is also a legacy builtinSigs entry, so it always takes the moved-hint path
// in checkIdent's isBuiltin branch, never reaching this fallback -- see
// docs/specs/2026-07-03-module-not-imported-hint-design.md's "The env
// exception") through a qualified-call use, with no import present, asserting
// the new "not imported" message for each. All modules except "string" are
// also driven through a bare value-position use. "string" has no
// value-position case (valueSrc == ""): it is a reserved type-keyword token
// (token.TypeString), so a bare, unqualified `string` value reference is a
// parse error ("type name \"string\" is not a value") under any
// circumstance, never reaching the type checker at all -- see the design
// doc's "The string bare-value-position exception" section. This is
// unrelated to import status and pre-dates this task.
//
// Argument values in the call fixtures are placeholders: checkIndirectCall
// short-circuits on the base identifier's Invalid result before argument
// type-checking ever runs, so exact argument types/counts do not affect the
// outcome. The value-position fixtures use `string` as a placeholder
// annotation type for the same reason -- the assignment's type compatibility
// is never checked because the RHS resolves to Invalid first.
func TestModuleNotImportedHint(t *testing.T) {
	cases := []struct {
		module   string
		callSrc  string
		valueSrc string // empty means: no value-position case for this module
	}{
		{
			module:   "array",
			callSrc:  `fn main() -> int { let xs: int[] = [1]; let n: int = array.sum(xs); return 0 }`,
			valueSrc: `fn main() -> int { let x: string = array; return 0 }`,
		},
		{
			module:   "dict",
			callSrc:  `fn main() -> int { let d: {string: int} = { "a": 1 }; let n: int = dict.size(d); return 0 }`,
			valueSrc: `fn main() -> int { let x: string = dict; return 0 }`,
		},
		{
			module:   "fs",
			callSrc:  `fn main() -> int { let b: bool = fs.file_exists("x"); return 0 }`,
			valueSrc: `fn main() -> int { let x: string = fs; return 0 }`,
		},
		{
			module:   "json",
			callSrc:  `fn main() -> int { let v: string = json.encode("x"); return 0 }`,
			valueSrc: `fn main() -> int { let x: string = json; return 0 }`,
		},
		{
			module:   "math",
			callSrc:  `fn main() -> int { let n: int = math.abs(-3); return 0 }`,
			valueSrc: `fn main() -> int { let x: string = math; return 0 }`,
		},
		{
			module:   "process",
			callSrc:  `fn main() -> int { let a: bool = process.pid_alive(1); return 0 }`,
			valueSrc: `fn main() -> int { let x: string = process; return 0 }`,
		},
		{
			module:   "regex",
			callSrc:  `fn main() -> int { let o: Optional[string] = regex.find("a", "b"); return 0 }`,
			valueSrc: `fn main() -> int { let x: string = regex; return 0 }`,
		},
		{
			module:  "string",
			callSrc: `fn main() -> int { let s: string = string.lower("A"); return 0 }`,
			// no valueSrc: see the doc comment above.
		},
	}
	for _, c := range cases {
		want := `module "` + c.module + `" is not imported; add import "` + c.module + `"`
		t.Run(c.module+"/call", func(t *testing.T) {
			root := mod(t, 0, c.callSrc, nil)
			info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
			if !hasErr(info, want) {
				t.Errorf("want %q, got %v", want, errMsgs(info))
			}
		})
		if c.valueSrc == "" {
			continue
		}
		t.Run(c.module+"/value", func(t *testing.T) {
			root := mod(t, 0, c.valueSrc, nil)
			info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
			if !hasErr(info, want) {
				t.Errorf("want %q, got %v", want, errMsgs(info))
			}
		})
	}
}

// TestModuleNotImportedHint_NonCoreTypoUnaffected confirms a genuine typo that
// is NOT a coreCatalog key still gets the generic undeclared-name message with
// no "did you mean" suffix (mathh is not within edit-distance-2 of any name in
// scope here, since scope is empty).
func TestModuleNotImportedHint_NonCoreTypoUnaffected(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let n: int = mathh.abs(-3); return 0 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `undeclared name "mathh"`) {
		t.Fatalf("want undeclared-name error, got %v", errMsgs(info))
	}
	if hasErr(info, "did you mean") {
		t.Errorf("did not expect a \"did you mean\" suggestion, got %v", errMsgs(info))
	}
}

// TestModuleNotImportedHint_LocalVarTypoUnaffected confirms isCoreModule's
// early-return doesn't disturb the existing fuzzy "did you mean" suggestion
// path for a typo near a real LOCAL VARIABLE name (not a function name --
// suggestSuffix's pool for this diagnostic is varNamesInScope(), which only
// returns variable names, never function names).
func TestModuleNotImportedHint_LocalVarTypoUnaffected(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let total: int = 1; return totl; }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `did you mean "total"?`) {
		t.Fatalf("want did-you-mean suggestion, got %v", errMsgs(info))
	}
}

// TestModuleNotImportedHint_ImportedPositiveControl confirms the new code path
// is never reached when the module IS imported -- c.cur.namespaces handles it
// upstream, unchanged.
func TestModuleNotImportedHint_ImportedPositiveControl(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let n: int = math.abs(-3); return 0 }`, map[string]int{"math": 1})
	m := coreMod(1, "math")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, m}})
	if len(info.Errors) != 0 {
		t.Fatalf("want 0 errors, got %v", errMsgs(info))
	}
}

// TestModuleNotImportedHint_AliasedImportStillHintsBareName confirms the fix is
// keyed on the coreCatalog name, not on "is this name imported under any
// alias": when math is imported under alias m (not bare "math"), a bare
// math.abs(-3) reference still gets the new "not imported" message naming the
// bare unaliased form, which is the correct fix regardless of the unrelated
// alias already in scope.
func TestModuleNotImportedHint_AliasedImportStillHintsBareName(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let n: int = math.abs(-3); return 0 }`, map[string]int{"m": 1})
	m := coreMod(1, "math")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, m}})
	if !hasErr(info, `module "math" is not imported; add import "math"`) {
		t.Fatalf("want not-imported error, got %v", errMsgs(info))
	}
}

// TestModuleNotImportedHint_RemovedBareCallGetsImportReminder confirms the
// removed bare-call spelling of a modularized builtin (e.g. abs(-3), the
// pre-migration flat form) now includes the import reminder alongside the
// existing module-hint, via call.go's checkNamedCall moved-hint branch.
func TestModuleNotImportedHint_RemovedBareCallGetsImportReminder(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let n: int = abs(-3); return 0 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `"abs" was moved to a module; import "math" and call it as math.abs(...)`) {
		t.Fatalf("want import-reminder moved-hint error, got %v", errMsgs(info))
	}
}

// TestModuleNotImportedHint_RemovedBareValueGetsImportReminder is the
// value-position half of the same fix, via expr.go's checkIdent moved-hint
// branch.
func TestModuleNotImportedHint_RemovedBareValueGetsImportReminder(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let f: int = abs; return 0 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `"abs" was moved to a module; import "math" and call it as math.abs(...)`) {
		t.Fatalf("want import-reminder moved-hint error, got %v", errMsgs(info))
	}
}
