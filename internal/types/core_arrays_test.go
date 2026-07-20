package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// checkArraysProg checks a root program that imports array as namespace "array"
// (bound to a synthetic core module at id 1). Mirrors checkMathProg.
func checkArraysProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"array": 1})
	am := coreMod(1, "array")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, am}})
}

// TestCoreArraysNonFuncrefMembers asserts each non-funcref member delegates to
// its flat builtin with the VERIFIED result type. pop/first/last return the bare
// element type (NOT Optional). Void members (push/insert_at/remove_at) are tested
// in statement position.
func TestCoreArraysNonFuncrefMembers(t *testing.T) {
	const xs = `let xs: int[] = [3, 1, 2];`
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { ` + xs + ` let v: int = array.pop(xs); return 0 }`, "pop", Int},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.reverse(xs); return 0 }`, "reverse", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.sort(xs); return 0 }`, "sort", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.slice(xs, 0, 2); return 0 }`, "slice", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.concat(xs, [9]); return 0 }`, "concat", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let n: int = array.sum(xs); return 0 }`, "sum", Int},
		{`fn main() -> int { let ys: int[] = array.range(5); return 0 }`, "range", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let v: int = array.first(xs); return 0 }`, "first", Int},
		{`fn main() -> int { ` + xs + ` let v: int = array.last(xs); return 0 }`, "last", Int},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.unique(xs); return 0 }`, "unique", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.take(xs, 2); return 0 }`, "take", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.drop(xs, 1); return 0 }`, "drop", arrayType(Int)},
		{`fn main() -> int { let nn: int[][] = [[1, 2], [3]]; let ys: int[] = array.flatten(nn); return 0 }`, "flatten", arrayType(Int)},
		// void members, statement position.
		{`fn main() -> int { ` + xs + ` array.push(xs, 9); return 0 }`, "push", Void},
		{`fn main() -> int { ` + xs + ` array.insert_at(xs, 0, 9); return 0 }`, "insert_at", Void},
		{`fn main() -> int { ` + xs + ` array.remove_at(xs, 0); return 0 }`, "remove_at", Void},
	} {
		info := checkArraysProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

// TestCoreArraysContainsIndexOf asserts the new array.contains/array.index_of
// catalog entries type-check to the correct result types (Bool / Optional[int]).
func TestCoreArraysContainsIndexOf(t *testing.T) {
	const xs = `let xs: int[] = [3, 1, 2];`
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { ` + xs + ` let b: bool = array.contains(xs, 1); return 0 }`, "contains", Bool},
		{`fn main() -> int { ` + xs + ` let o: Optional[int] = array.index_of(xs, 1); return 0 }`, "index_of", optionalType(Int)},
	} {
		info := checkArraysProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

// TestCoreArraysZip: zip yields a tuple-array; assert the Builtin key is recorded
// and the result is an array (the exact tuple token is left unpinned).
func TestCoreArraysZip(t *testing.T) {
	info := checkArraysProg(t, `fn main() -> int { let a: int[] = [1]; let b: string[] = ["x"]; let z: (int, string)[] = array.zip(a, b); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("array.zip: unexpected errors: %v", errMsgs(info))
	}
	ci := callWithBuiltin(info, "zip")
	if ci == nil {
		t.Fatal("array.zip did not record Builtin zip")
	}
	if !isArray(ci.Result) {
		t.Errorf("array.zip result = %q, want an array", ci.Result)
	}
}

// TestCoreArraysFuncrefMembers proves funcref-arg inference runs through
// delegation: passing a user fn into array.<member> infers exactly as the flat
// builtin. Void each is in statement position.
func TestCoreArraysFuncrefMembers(t *testing.T) {
	const helpers = "fn dbl(x: int) -> int { return x * 2 }\n" +
		"fn is_even(x: int) -> bool { return x % 2 == 0 }\n" +
		"fn sink(x: int) -> void { print(to_string(x)) }\n" +
		"fn add(a: int, b: int) -> int { return a + b }\n" +
		"fn lt(a: int, b: int) -> bool { return a < b }\n"
	const xs = `let xs: int[] = [1, 2, 3];`
	for _, c := range []struct {
		body    string
		builtin string
		want    Type
	}{
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.map(xs, dbl); return 0 }`, "map", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.filter(xs, is_even); return 0 }`, "filter", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` array.each(xs, sink); return 0 }`, "each", Void},
		{`fn main() -> int { ` + xs + ` let s: int = array.reduce(xs, 0, add); return 0 }`, "reduce", Int},
		{`fn main() -> int { ` + xs + ` let ys: int[] = array.sort_by(xs, lt); return 0 }`, "sort_by", arrayType(Int)},
		{`fn main() -> int { ` + xs + ` let o: Optional[int] = array.find(xs, is_even); return 0 }`, "find", optionalType(Int)},
		{`fn main() -> int { ` + xs + ` let b: bool = array.any(xs, is_even); return 0 }`, "any", Bool},
		{`fn main() -> int { ` + xs + ` let b: bool = array.all(xs, is_even); return 0 }`, "all", Bool},
		{`fn main() -> int { ` + xs + ` let n: int = array.count_where(xs, is_even); return 0 }`, "count_where", Int},
	} {
		info := checkArraysProg(t, helpers+c.body)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

// TestCoreArraysFuncrefArgInferenceMatchesFlat proves funcref-arg inference runs
// through delegation: passing a user fn into array.map infers the map result as
// int[]. The bare flat spelling no longer exists, so only the namespaced form is
// exercised.
func TestCoreArraysFuncrefArgInferenceMatchesFlat(t *testing.T) {
	const helper = "fn dbl(x: int) -> int { return x * 2 }\n"
	nsInfo := checkArraysProg(t, helper+`fn main() -> int { let xs: int[] = [1, 2]; let ys: int[] = array.map(xs, dbl); return 0 }`)
	if len(nsInfo.Errors) != 0 {
		t.Fatalf("array.map should type-check; errors: %v", errMsgs(nsInfo))
	}
	nsCI := callWithBuiltin(nsInfo, "map")
	if nsCI == nil {
		t.Fatalf("missing map CallInfo")
	}
	if nsCI.Result != arrayType(Int) {
		t.Errorf("array.map result = %q, want int[]", nsCI.Result)
	}
}

// TestCoreArraysInsertRemoveDomainModuleNamed: delegate preserves the
// compile-time index arg-domain diagnostic, now module-qualified.
func TestCoreArraysInsertRemoveDomainModuleNamed(t *testing.T) {
	for _, c := range []struct {
		src  string
		want string
	}{
		{`fn main() -> int { let xs: int[] = [1]; array.remove_at(xs, -1); return 0 }`, "array.remove_at: index out of range"},
		{`fn main() -> int { let xs: int[] = [1]; array.insert_at(xs, -1, 9); return 0 }`, "array.insert_at: index out of range"},
	} {
		info := checkArraysProg(t, c.src)
		if !hasErr(info, c.want) {
			t.Fatalf("%q: want %q, got %v", c.src, c.want, errMsgs(info))
		}
	}
}

func TestCoreArraysFuncrefValueOutOfScope(t *testing.T) {
	info := checkArraysProg(t, `fn main() -> int { let xs: int[][] = [[1]]; let r: int[][] = map(xs, array.reverse); return 0 }`)
	if !hasErr(info, `"reverse" of module "array" cannot be referenced as a function value (it is overloaded or generic); wrap it in a fn`) {
		t.Fatalf("want func-as-value error, got %v", errMsgs(info))
	}
}

func TestCoreArraysTypeArgsRejected(t *testing.T) {
	info := checkArraysProg(t, `fn main() -> int { let xs: int[] = [1]; let n: int = array.sum[int](xs); return 0 }`)
	if !hasErr(info, "array.sum does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreArraysUnknownMemberSuggestion(t *testing.T) {
	info := checkArraysProg(t, `fn main() -> int { array.ma(); return 0 }`)
	if !hasErr(info, `did you mean "map"?`) {
		t.Fatalf("want map suggestion, got %v", errMsgs(info))
	}
}

func TestCoreArraysAliasImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let xs: int[] = [1]; let n: int = ar.sum(xs); return 0 }`, map[string]int{"ar": 1})
	am := coreMod(1, "array")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, am}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased ar.sum should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "sum"); ci == nil || ci.Result != Int {
		t.Errorf("ar.sum result = %v, want int", ci)
	}
}

func TestCoreArraysMissingImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let xs: int[] = [1]; let n: int = array.sum(xs); return 0 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `module "array" is not imported; add import "array"`) {
		t.Fatalf("want undeclared-name error, got %v", errMsgs(info))
	}
}
