package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// withDelegateProbe injects a probe namespace whose member `d` DELEGATES to the
// overloaded flat builtin `abs` via the general delegation seam. This proves the
// seam routes a namespaced call through the real special handler (checkAbsCall),
// not the fixed coreSig path.
func withDelegateProbe(t *testing.T) func() {
	t.Helper()
	coreCatalog["__dprobe"] = map[string]coreMember{
		"d": {kind: coreFunc, builtin: "abs", delegate: true},
	}
	return func() { delete(coreCatalog, "__dprobe") }
}

func TestCoreDelegateRunsOverloadHandler(t *testing.T) {
	defer withDelegateProbe(t)()
	// __dprobe.d(-5): abs on an int -> the int overload handler (checkAbsCall)
	// records Builtin=="abs" with Result==Int.
	root := mod(t, 0, `fn main() -> int { return __dprobe.d(-5) }`, map[string]int{"__dprobe": 1})
	pm := coreMod(1, "__dprobe")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, pm}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors resolving __dprobe.d(-5): %v", errMsgs(info))
	}
	ci := callWithBuiltin(info, "abs")
	if ci == nil {
		t.Fatal("no CallBuiltin abs recorded (delegation did not reach checkAbsCall)")
	}
	if ci.Result != Int {
		t.Errorf("__dprobe.d(-5) result = %q, want int", ci.Result)
	}
}

func TestCoreDelegatePreservesArgDomainCheck(t *testing.T) {
	defer withDelegateProbe(t)()
	// abs(INT_MIN) is a compile-time overflow reject in builtinIntArgDomains; the
	// seam must run checkBuiltinArgDomains so the delegated call keeps that
	// diagnostic (a plain coreSig path would drop it).
	root := mod(t, 0, `fn main() -> int { return __dprobe.d(-9223372036854775808) }`, map[string]int{"__dprobe": 1})
	pm := coreMod(1, "__dprobe")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, pm}})
	if !hasErr(info, "__dprobe.d(): integer overflow") {
		t.Fatalf("want abs overflow domain error via delegation, got %v", errMsgs(info))
	}
}
