package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// checkEnvProg checks a root program importing env as namespace "env" at id 1.
func checkEnvProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"env": 1})
	em := coreMod(1, "env")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, em}})
}

func TestCoreEnvMembersResolve(t *testing.T) {
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let s: string = env.get("X"); return 0 }`, "env", String},
		{`fn main() -> int { let b: bool = env.has("X"); return 0 }`, "has_env", Bool},
		{`fn main() -> int { env.set("X", "v"); return 0 }`, "set_env", Void},
		{`fn main() -> int { env.unset("X"); return 0 }`, "unset_env", Void},
		{`fn main() -> int { let s: string = env.get_or("X", "d"); return 0 }`, "env_or", String},
	} {
		info := checkEnvProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		ci := callWithBuiltin(info, c.builtin)
		if ci == nil {
			t.Fatalf("%s: no CallBuiltin recorded", c.builtin)
		}
		if ci.Result != c.want {
			t.Errorf("%s: result = %q, want %q", c.builtin, ci.Result, c.want)
		}
	}
}

func TestCoreEnvAliasImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let s: string = e.get("X"); return 0 }`, map[string]int{"e": 1})
	em := coreMod(1, "env")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, em}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased e.get should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "env"); ci == nil || ci.Result != String {
		t.Errorf("e.get result = %v, want string", ci)
	}
}

// TestCoreEnvNamespaceCoexistsWithBuiltin proves that with the namespace "env"
// bound, env.get resolves via the namespace, while the bare flat spelling env(x)
// has been removed and now yields the moved-to-module diagnostic.
func TestCoreEnvNamespaceCoexistsWithBuiltin(t *testing.T) {
	info := checkEnvProg(t, `fn main() -> int { let a: string = env.get("X"); let b: string = env("Y"); return 0 }`)
	if !hasErr(info, `"env" was moved to a module; import "env" and call it as env.get(...)`) {
		t.Fatalf("want moved-to-module error for bare env(), got %v", errMsgs(info))
	}
	// The namespaced call still records CallBuiltin "env".
	found := false
	for _, ci := range info.Calls {
		if ci.Kind == CallBuiltin && ci.Builtin == "env" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected env.get to record CallBuiltin env")
	}
}

// TestCoreEnvValuePositionMessageShift pins value-position resolution of bare
// `env`. With the namespace bound, the alias takes precedence and the
// namespace-not-a-value message wins. Without it, `env` is a removable builtin
// whose bare spelling is gone, so the value reference yields the moved-to-module
// diagnostic.
func TestCoreEnvValuePositionMessageShift(t *testing.T) {
	// With the namespace bound: the namespace-not-a-value message wins.
	withNS := checkEnvProg(t, `fn main() -> int { let x: string = env; return 0 }`)
	if !hasErr(withNS, `"env" is a module namespace, not a value`) {
		t.Fatalf("with import: want namespace-not-a-value error, got %v", errMsgs(withNS))
	}
	// Without it: bare env is a removed flat builtin, so a value reference yields
	// the moved-to-module diagnostic.
	root := mod(t, 0, `fn main() -> int { let x: string = env; return 0 }`, nil)
	noNS := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(noNS, `"env" was moved to a module; import "env" and call it as env.get(...)`) {
		t.Fatalf("without import: want moved-to-module error, got %v", errMsgs(noNS))
	}
}

func TestCoreEnvUnknownMemberSuggestion(t *testing.T) {
	info := checkEnvProg(t, `fn main() -> int { env.st("X", "v"); return 0 }`)
	if !hasErr(info, `did you mean "set"?`) {
		t.Fatalf("want set suggestion, got %v", errMsgs(info))
	}
}

func TestCoreEnvTypeArgsRejected(t *testing.T) {
	info := checkEnvProg(t, `fn main() -> int { let s: string = env.get[int]("X"); return 0 }`)
	if !hasErr(info, "env.get does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreEnvArgTypeErrorNamesMember(t *testing.T) {
	info := checkEnvProg(t, `fn main() -> int { let s: string = env.get(1); return 0 }`)
	if !hasErr(info, "argument 1 of env.get has type int, want string") {
		t.Fatalf("want member-named arg-type error, got %v", errMsgs(info))
	}
}
