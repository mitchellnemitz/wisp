package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// checkRegexProg checks a root program that imports regex as namespace "regex"
// (bound to a synthetic core module at id 1). Mirrors checkJSONProg.
func checkRegexProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"regex": 1})
	rx := coreMod(1, "regex")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, rx}})
}

func TestCoreRegexMembersResolve(t *testing.T) {
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let b: bool = regex.matches("a", "b"); return 0 }`, "matches", Bool},
		{`fn main() -> int { let o: Optional[string] = regex.find("a", "b"); return 0 }`, "regex_find", optionalType(String)},
		{`fn main() -> int { let a: string[] = regex.find_all("a", "b"); return 0 }`, "regex_find_all", arrayType(String)},
		{`fn main() -> int { let s: string = regex.replace("a", "b", "c"); return 0 }`, "regex_replace", String},
	} {
		info := checkRegexProg(t, c.src)
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

func TestCoreRegexAliasImport(t *testing.T) {
	// import "regex" as re -> re.find resolves identically to regex.find.
	root := mod(t, 0, `fn main() -> int { let o: Optional[string] = re.find("a", "b"); return 0 }`, map[string]int{"re": 1})
	rx := coreMod(1, "regex")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, rx}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased re.find should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "regex_find"); ci == nil || ci.Result != optionalType(String) {
		t.Errorf("re.find result = %v, want Optional[string]", ci)
	}
}

func TestCoreRegexUnknownMemberNoSuggestion(t *testing.T) {
	// A far typo yields the base no-member message with no did-you-mean (nope is
	// Levenshtein distance >=3 from every member).
	info := checkRegexProg(t, `fn main() -> int { regex.nope(); return 0 }`)
	if !hasErr(info, `module "regex" has no member "nope"`) {
		t.Fatalf("want unknown-member error, got %v", errMsgs(info))
	}
	if hasErr(info, "did you mean") {
		t.Errorf("did not expect a suggestion for far typo; got %v", errMsgs(info))
	}
}

func TestCoreRegexUnknownMemberSuggestion(t *testing.T) {
	// A near typo (distance 1 from "find") yields a did-you-mean suggestion.
	info := checkRegexProg(t, `fn main() -> int { regex.fnd(); return 0 }`)
	if !hasErr(info, `did you mean "find"?`) {
		t.Fatalf("want find suggestion, got %v", errMsgs(info))
	}
}

func TestCoreRegexTypeArgsRejected(t *testing.T) {
	info := checkRegexProg(t, `fn main() -> int { let o: Optional[string] = regex.find[int]("a", "b"); return 0 }`)
	if !hasErr(info, "regex.find does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreRegexArgTypeErrorNamesMember(t *testing.T) {
	info := checkRegexProg(t, `fn main() -> int { let o: Optional[string] = regex.find(1, "x"); return 0 }`)
	if !hasErr(info, "argument 1 of regex.find has type int, want string") {
		t.Fatalf("want member-named arg-type error, got %v", errMsgs(info))
	}
}

func TestCoreRegexMissingImport(t *testing.T) {
	// Without import "regex", the namespace is unbound: the call falls through to
	// expression checking and "regex" resolves as an undeclared identifier.
	root := mod(t, 0, `fn main() -> int { let o: Optional[string] = regex.find("a", "b"); return 0 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `module "regex" is not imported; add import "regex"`) {
		t.Fatalf("want undeclared-name error, got %v", errMsgs(info))
	}
}
