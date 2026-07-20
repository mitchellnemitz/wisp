package types

import "testing"

// Regular-expression stdlib (regex milestone): type-checker coverage for the
// four whole-match builtins. All four are removable builtins now spelled
// regex.matches / regex.find / regex.find_all / regex.replace, so every test
// below checks through the linked module set with the regex namespace bound.
// Fixed-signature (every arg string); results are Bool / Optional[string] /
// string[] / string.

// --- well-typed results ---

func TestRegex_Builtins_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let a: bool = regex.matches("x", "y")
let b: Optional[string] = regex.find("x", "y")
let c: string[] = regex.find_all("x", "y")
let d: string = regex.replace("x", "y", "z")`), "regex")
}

// --- argument typing ---

func TestRegex_Matches_ArgsMustBeStrings(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = regex.matches(1, "y")`), "regex.matches", "regex")
	expectErrNS(t, wrapMain(`let a: bool = regex.matches("x", 1)`), "regex.matches", "regex")
}

func TestRegex_Find_ArgsMustBeStrings(t *testing.T) {
	expectErrNS(t, wrapMain(`let b: Optional[string] = regex.find(1, "y")`), "regex.find", "regex")
	expectErrNS(t, wrapMain(`let b: Optional[string] = regex.find("x", 1)`), "regex.find", "regex")
}

func TestRegex_FindAll_ArgsMustBeStrings(t *testing.T) {
	expectErrNS(t, wrapMain(`let c: string[] = regex.find_all(1, "y")`), "regex.find_all", "regex")
	expectErrNS(t, wrapMain(`let c: string[] = regex.find_all("x", 1)`), "regex.find_all", "regex")
}

func TestRegex_Replace_ArgsMustBeStrings(t *testing.T) {
	expectErrNS(t, wrapMain(`let d: string = regex.replace(1, "y", "z")`), "regex.replace", "regex")
	expectErrNS(t, wrapMain(`let d: string = regex.replace("x", 1, "z")`), "regex.replace", "regex")
	expectErrNS(t, wrapMain(`let d: string = regex.replace("x", "y", 1)`), "regex.replace", "regex")
}

// --- argument counts ---

func TestRegex_ArgCounts(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = regex.matches("x")`), "regex.matches", "regex")
	expectErrNS(t, wrapMain(`let a: bool = regex.matches("x", "y", "z")`), "regex.matches", "regex")
	expectErrNS(t, wrapMain(`let b: Optional[string] = regex.find("x")`), "regex.find", "regex")
	expectErrNS(t, wrapMain(`let c: string[] = regex.find_all("x")`), "regex.find_all", "regex")
	expectErrNS(t, wrapMain(`let d: string = regex.replace("x", "y")`), "regex.replace", "regex")
	expectErrNS(t, wrapMain(`let d: string = regex.replace("x", "y", "z", "w")`), "regex.replace", "regex")
}

// --- result types ---

func TestRegex_ResultTypes(t *testing.T) {
	expectErrNS(t, wrapMain(`let x: string = regex.matches("x", "y")`), "bool", "regex")
	expectErrNS(t, wrapMain(`let x: string = regex.find("x", "y")`), "Optional", "regex")
	expectErrNS(t, wrapMain(`let x: string = regex.find_all("x", "y")`), "string", "regex")
	expectErrNS(t, wrapMain(`let x: bool = regex.replace("x", "y", "z")`), "string", "regex")
}

// --- names freed for user binding (all four) ---

// matches/regex_find/regex_find_all/regex_replace are removable builtins:
// their flat names were freed by the modules-only migration (isReservedName
// excludes the removable set), so they are now ordinary identifiers a user
// may bind -- unlike the pre-removal original, which reserved them as bare
// builtin names.
var regexNames = []string{"matches", "regex_find", "regex_find_all", "regex_replace"}

func TestRegex_NamesFreed_Fn(t *testing.T) {
	for _, name := range regexNames {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "regex")
	}
}

func TestRegex_NamesFreed_Let(t *testing.T) {
	for _, name := range regexNames {
		expectOKNS(t, wrapMain("let "+name+": int = 0"), "regex")
	}
}

func TestRegex_NamesFreed_Param(t *testing.T) {
	for _, name := range regexNames {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "regex")
	}
}
