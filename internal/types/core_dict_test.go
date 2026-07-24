package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// checkDictProg checks a root program that imports dict as namespace "dict"
// (bound to a synthetic core module at id 1). Mirrors checkMathProg.
func checkDictProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"dict": 1})
	dm := coreMod(1, "dict")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, dm}})
}

// TestCoreDictMembersResolve asserts every dict member delegates to its flat
// builtin with the correct generic result. The value-returning members are tested
// in value position; the void members (remove, clear) in statement position.
func TestCoreDictMembersResolve(t *testing.T) {
	const decl = `let d: {string: int} = { "a": 1, "b": 2 };`
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { ` + decl + ` let x: bool = dict.has(d, "a"); return 0 }`, "has", Bool},
		{`fn main() -> int { ` + decl + ` let k: string[] = dict.keys(d); return 0 }`, "keys", arrayType(String)},
		{`fn main() -> int { ` + decl + ` let o: Optional[int] = dict.get(d, "a"); return 0 }`, "get", optionalType(Int)},
		{`fn main() -> int { ` + decl + ` let vs: int[] = dict.values(d); return 0 }`, "values", arrayType(Int)},
		{`fn main() -> int { ` + decl + ` let n: int = dict.size(d); return 0 }`, "size", Int},
		{`fn main() -> int { ` + decl + ` let b: bool = dict.is_empty(d); return 0 }`, "dict_is_empty", Bool},
		{`fn main() -> int { let e: {string: int} = {}; let b: bool = dict.is_empty(e); return 0 }`, "dict_is_empty", Bool},
		{`fn main() -> int { ` + decl + ` let m: {string: int} = dict.merge(d, d); return 0 }`, "merge", dictType(String, Int)},
		{`fn main() -> int { ` + decl + ` dict.remove(d, "a"); return 0 }`, "remove", Void},
		{`fn main() -> int { ` + decl + ` dict.clear(d); return 0 }`, "clear", Void},
	} {
		info := checkDictProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

// TestCoreDictGenericKV proves the generic inference runs through delegation:
// get/keys over {int:string} yield the other K/V pair.
func TestCoreDictGenericKV(t *testing.T) {
	info := checkDictProg(t, `fn main() -> int { let d: {int: string} = { 1: "a" }; let o: Optional[string] = dict.get(d, 1); let k: int[] = dict.keys(d); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "get"); ci == nil || ci.Result != optionalType(String) {
		t.Errorf("dict.get over {int:string} result = %v, want Optional[string]", ci)
	}
	if ci := callWithBuiltin(info, "keys"); ci == nil || ci.Result != arrayType(Int) {
		t.Errorf("dict.keys over {int:string} result = %v, want int[]", ci)
	}
}

// TestCoreDictGetIsDictGetNotJSON pins dict.get -> flat "get" (dict get), NOT
// json_get; the two are distinct builtin keys.
func TestCoreDictGetIsDictGetNotJSON(t *testing.T) {
	info := checkDictProg(t, `fn main() -> int { let d: {string: int} = { "a": 1 }; let o: Optional[int] = dict.get(d, "a"); return 0 }`)
	if ci := callWithBuiltin(info, "get"); ci == nil {
		t.Fatal("dict.get did not record Builtin get")
	}
	if ci := callWithBuiltin(info, "json_get"); ci != nil {
		t.Errorf("dict.get must NOT record Builtin json_get")
	}
}

// TestCoreDictArgErrorModuleNamed: a delegate member surfaces the module-qualified
// dict diagnostic (the reused handler now receives dispName).
func TestCoreDictArgErrorModuleNamed(t *testing.T) {
	info := checkDictProg(t, `fn main() -> int { let d: {string: int} = { "a": 1 }; let o: Optional[int] = dict.get(d, 1); return 0 }`)
	if !hasErr(info, "argument 2 of dict.get has type int, want string (the dict key type)") {
		t.Fatalf("want module-named dict key-type error, got %v", errMsgs(info))
	}
}

func TestCoreDictFuncrefValueOutOfScope(t *testing.T) {
	info := checkDictProg(t, `fn main() -> int { let xs: string[] = ["a"]; let r: bool[] = map(xs, dict.has); return 0 }`)
	if !hasErr(info, `"has" of module "dict" cannot be referenced as a function value (it has no single funcref-shaped scalar lowering); wrap it in a fn`) {
		t.Fatalf("want func-as-value error, got %v", errMsgs(info))
	}
}

func TestCoreDictTypeArgsRejected(t *testing.T) {
	info := checkDictProg(t, `fn main() -> int { let d: {string: int} = { "a": 1 }; let n: int = dict.size[int](d); return 0 }`)
	if !hasErr(info, "dict.size does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreDictUnknownMemberSuggestion(t *testing.T) {
	info := checkDictProg(t, `fn main() -> int { dict.ha(); return 0 }`)
	if !hasErr(info, `did you mean "has"?`) {
		t.Fatalf("want has suggestion, got %v", errMsgs(info))
	}
}

func TestCoreDictAliasImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let d: {string: int} = { "a": 1 }; let n: int = dd.size(d); return 0 }`, map[string]int{"dd": 1})
	dm := coreMod(1, "dict")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, dm}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased dd.size should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "size"); ci == nil || ci.Result != Int {
		t.Errorf("dd.size result = %v, want int", ci)
	}
}

func TestCoreDictMissingImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let d: {string: int} = { "a": 1 }; let n: int = dict.size(d); return 0 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `module "dict" is not imported; add import "dict"`) {
		t.Fatalf("want undeclared-name error, got %v", errMsgs(info))
	}
}
