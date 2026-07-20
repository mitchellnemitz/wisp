package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
)

// coreMod builds a synthetic core module (empty Prog, Core set) with the given id.
func coreMod(id int, name string) *module.Module {
	return &module.Module{ID: id, Prog: &ast.Program{}, Namespaces: map[string]int{}, Core: name}
}

// checkJSONProg checks a root program that imports json as namespace "json"
// (bound to a synthetic core module at id 1).
func checkJSONProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"json": 1})
	js := coreMod(1, "json")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, js}})
}

// callWithBuiltin returns the first CallInfo whose Builtin == name, or nil.
func callWithBuiltin(info *Info, name string) *CallInfo {
	for _, ci := range info.Calls {
		if ci.Kind == CallBuiltin && ci.Builtin == name {
			return ci
		}
	}
	return nil
}

func TestCoreJSONDecodeDefaultsToValue(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { let v: json.Value = json.decode("{}"); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
	ci := callWithBuiltin(info, "json_decode")
	if ci == nil {
		t.Fatal("no json_decode CallBuiltin recorded")
	}
	if ci.Result != jsonValueType {
		t.Errorf("json.decode result = %q, want %q", ci.Result, jsonValueType)
	}
}

func TestCoreJSONDecodeTypeArgDirected(t *testing.T) {
	for _, c := range []struct {
		targ string
		want Type
	}{
		{"int", Int}, {"string", String}, {"float", Float}, {"bool", Bool},
		{"json.Value", jsonValueType},
	} {
		info := checkJSONProg(t, `fn main() -> int { let v: `+c.targ+` = json.decode[`+c.targ+`]("1"); return 0 }`)
		if len(info.Errors) != 0 {
			t.Fatalf("decode[%s]: unexpected errors: %v", c.targ, errMsgs(info))
		}
		ci := callWithBuiltin(info, "json_decode")
		if ci == nil || ci.Result != c.want {
			t.Errorf("decode[%s] result = %v, want %v", c.targ, ci, c.want)
		}
	}
}

func TestCoreJSONDecodeAliasTarget(t *testing.T) {
	// A transparent alias of a supported type resolves through resolveType, so
	// decode[MyInt] behaves as decode[int].
	info := checkJSONProg(t, `type MyInt = int`+"\n"+`fn main() -> int { let v: MyInt = json.decode[MyInt]("1"); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("decode[MyInt]: unexpected errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "json_decode"); ci == nil || ci.Result != Int {
		t.Errorf("decode[MyInt] result = %v, want int", ci)
	}
}

func TestCoreJSONDecodeUnsupportedTarget(t *testing.T) {
	info := checkJSONProg(t, `struct S { x: int }`+"\n"+`fn main() -> int { let v: S = json.decode[S]("1"); return 0 }`)
	if !hasErr(info, "does not support target type S") {
		t.Fatalf("want unsupported-target error, got %v", errMsgs(info))
	}
}

func TestCoreJSONDecodeTooManyTypeArgs(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { let v: int = json.decode[int, bool]("1"); return 0 }`)
	if !hasErr(info, "at most one type argument") {
		t.Fatalf("want too-many-type-args error, got %v", errMsgs(info))
	}
}

// --- A5: qualified type + value-position resolution ---

func TestCoreJSONValueInTypePosition(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { let v: json.Value = json.null(); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("json.Value type annotation should resolve; errors: %v", errMsgs(info))
	}
}

func TestCoreJSONUnknownTypeMember(t *testing.T) {
	info := checkJSONProg(t, `fn f(x: json.Nope) -> int { return 0 }`+"\n"+`fn main() -> int { return 0 }`)
	if !hasErr(info, `module "json" has no type "Nope"`) {
		t.Fatalf("want unknown-type error, got %v", errMsgs(info))
	}
}

func TestCoreJSONFuncAsValue(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { print(json.encode); return 0 }`)
	if !hasErr(info, `"encode" of module "json" cannot be referenced as a function value (it has no single funcref-shaped scalar lowering); wrap it in a fn`) {
		t.Fatalf("want func-as-value error, got %v", errMsgs(info))
	}
}

func TestCoreJSONTypeAsValue(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { print(json.Value); return 0 }`)
	if !hasErr(info, `"Value" is a type of module "json", not a value`) {
		t.Fatalf("want type-as-value error, got %v", errMsgs(info))
	}
}

// TestCoreJSONDecodeAsValue pins that json.decode -- a type-argument member --
// can never be a funcref value. The rejection is gated on the takesTypeArgs
// flag (NOT on an empty builtin string): json.decode HAS a builtin key
// ("json_decode"), so a builtin-string check would wrongly try to funcref it.
func TestCoreJSONDecodeAsValue(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { let f: fn(string)->int = json.decode; return 0 }`)
	if !hasErr(info, "json.decode takes type arguments and cannot be used as a function value") {
		t.Fatalf("want type-args-as-value error, got %v", errMsgs(info))
	}
}

func TestCoreJSONAliasImport(t *testing.T) {
	// import "json" as j -> j.decode resolves identically.
	root := mod(t, 0, `fn main() -> int { let v: int = j.decode[int]("1"); return 0 }`, map[string]int{"j": 1})
	js := coreMod(1, "json")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, js}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased j.decode should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "json_decode"); ci == nil || ci.Result != Int {
		t.Errorf("j.decode[int] result = %v, want int", ci)
	}
}

func TestCoreJSONUnknownMember(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { json.nope(); return 0 }`)
	if !hasErr(info, `module "json" has no member "nope"`) {
		t.Fatalf("want unknown-member error, got %v", errMsgs(info))
	}
}

func TestCoreJSONTypeNotCallable(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { json.Value(); return 0 }`)
	if !hasErr(info, `"Value" is a type of module "json", not callable`) {
		t.Fatalf("want type-not-callable error, got %v", errMsgs(info))
	}
}

func TestCoreJSONEncodeTypeArgsRejected(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { json.encode[int](json.null()); return 0 }`)
	if !hasErr(info, "json.encode does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreJSONArgTypeErrorNamesMember(t *testing.T) {
	// Guards the checkBuiltinSig extraction: a wrong-typed core-member arg must
	// name the member (json.encode), not an empty string.
	info := checkJSONProg(t, `fn main() -> int { let s: string = json.encode(5); return 0 }`)
	if !hasErr(info, "argument 1 of json.encode has type int, want json.Value") {
		t.Fatalf("want member-named arg-type error, got %v", errMsgs(info))
	}
}

// --- Generality proof: a genuinely SEPARATE reserved namespace resolves through
// the SAME loader-synthetic-module + checker-bridge paths, with NON-json data
// (including a coreConst, which json has none of). ---

func withProbeNamespace(t *testing.T) func() {
	t.Helper()
	coreCatalog["__probe"] = map[string]coreMember{
		"T": {kind: coreType, typ: Int}, // any resolvable Type stands in
		"f": {kind: coreFunc, builtin: "__probe_f", sig: coreSig0(Int)},
		"C": {kind: coreConst, constVal: ConstEntry{Value: int64(42), Type: Int}},
	}
	return func() { delete(coreCatalog, "__probe") }
}

func TestCoreBridgeGenericSecondNamespace(t *testing.T) {
	defer withProbeNamespace(t)()

	root := mod(t, 0, `fn main() -> int { return __probe.f() }`, map[string]int{"__probe": 1})
	pm := coreMod(1, "__probe")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, pm}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors resolving __probe.f(): %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "__probe_f"); ci == nil || ci.Result != Int {
		t.Errorf("__probe.f() did not resolve to CallBuiltin __probe_f -> int; got %v", ci)
	}
}

func TestCoreBridgeGenericUnknownMember(t *testing.T) {
	defer withProbeNamespace(t)()
	root := mod(t, 0, `fn main() -> int { __probe.g(); return 0 }`, map[string]int{"__probe": 1})
	pm := coreMod(1, "__probe")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, pm}})
	if !hasErr(info, `module "__probe" has no member "g"`) {
		t.Fatalf("want unknown-member error for the second namespace, got %v", errMsgs(info))
	}
}

func TestCoreBridgeGenericConstAndType(t *testing.T) {
	defer withProbeNamespace(t)()
	// coreConst success path (json exposes no consts, so this is the only coverage)
	// and coreType in type position, both through the generic resolvers.
	root := mod(t, 0, `fn main() -> int { let x: __probe.T = __probe.C; return x }`, map[string]int{"__probe": 1})
	pm := coreMod(1, "__probe")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, pm}})
	if len(info.Errors) != 0 {
		t.Fatalf("__probe.T / __probe.C should resolve; errors: %v", errMsgs(info))
	}
}
