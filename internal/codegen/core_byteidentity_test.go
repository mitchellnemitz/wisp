package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// genCore compiles rootSrc as "test.wisp" in a two-module Linked set: the root at
// id 0 (with the given namespace bindings) and a synthetic core module at id 1.
// Both the namespaced and the flat variants of a byte-identity pair go through
// THIS path (identical module structure), so the only difference between them is
// the call spelling in the source. The generated shell embeds source positions
// (per-function `# file:line`; runtime `file:line:col` literals), so callers must
// keep the two source variants position-aligned (same filename, same line for
// every function, and identical columns for every embedded position).
func genCore(t *testing.T, rootSrc string, ns map[string]int, coreName string) string {
	t.Helper()
	root, err := parser.Parse(rootSrc, "test.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	linked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: root, Namespaces: ns},
		{ID: 1, Prog: &ast.Program{}, Namespaces: map[string]int{}, Core: coreName},
	}}
	info := types.CheckLinked(linked)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	out, _, err := GenerateLinked(linked, info)
	if err != nil {
		t.Fatalf("GenerateLinked: %v", err)
	}
	return string(out)
}

// coreAnnotationRe matches per-function `# test.wisp:<N>` location comments. The
// leading `# ` distinguishes these header comments from runtime position literals
// (`test.wisp:<line>:<col>` embedded inside emitted shell strings), which are
// load-bearing and are NOT stripped.
var coreAnnotationRe = regexp.MustCompile(`# test\.wisp:\d+`)

// stripCoreAnnotations removes per-function location comments defensively before
// comparing a live namespaced lowering against the frozen baseline. PR C removed
// the flat surface, so the old flat-vs-namespaced comparison is gone; the frozen
// baseline (core_byteidentity_baseline.go) is the pre-removal shared lowering, and
// the namespaced lowering must still equal it byte-for-byte. In practice the
// baseline was captured from the identical source templates, so line numbers
// already match without stripping; the strip is a guard against future line drift.
func stripCoreAnnotations(s string) string {
	return coreAnnotationRe.ReplaceAllString(s, "")
}

// assertCoreByteIdentical compiles the namespaced program and asserts its generated
// shell equals the frozen pre-removal baseline for this call site (byte-for-byte
// after stripping location-comment annotations), and is non-trivial.
func assertCoreByteIdentical(t *testing.T, coreName, memberPath, builtin, argsStr, bodyTmpl string) {
	t.Helper()
	nsCall := memberPath + "(" + argsStr + ")"
	nsSrc := strings.Replace(bodyTmpl, "@CALL@", nsCall, 1)
	got := genCore(t, nsSrc, map[string]int{coreName: 1}, coreName)

	key := coreName + "|" + memberPath + "|" + argsStr
	want, ok := coreByteIdentityBaseline[key]
	if !ok {
		t.Fatalf("no frozen baseline entry for %q (regenerate core_byteidentity_baseline.go)", key)
	}
	if stripCoreAnnotations(got) != stripCoreAnnotations(want) {
		t.Fatalf("%s no longer matches the frozen baseline.\n--- namespaced ---\n%s\n--- baseline ---\n%s", memberPath, got, want)
	}
	if !strings.Contains(got, builtin) && !strings.Contains(got, "__wisp") {
		t.Fatalf("%s produced trivial output (no builtin marker):\n%s", memberPath, got)
	}
}

func TestCoreEnvByteIdentical(t *testing.T) {
	// env.get -> env (coreSig, Optional[string] result).
	assertCoreByteIdentical(t, "env", "env.get", "env", `"X"`,
		"fn main() -> int { let s: Optional[string] = @CALL@; return 0 }")
	// env.get_or -> env_or (two args).
	assertCoreByteIdentical(t, "env", "env.get_or", "env_or", `"X", "d"`,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
	// env.set -> set_env (void, statement position).
	assertCoreByteIdentical(t, "env", "env.set", "set_env", `"X", "v"`,
		"fn main() -> int { @CALL@; return 0 }")
}

func TestCoreMathByteIdentical(t *testing.T) {
	// coreSig float member.
	assertCoreByteIdentical(t, "math", "math.sqrt", "sqrt", `4.0`,
		"fn main() -> int { let f: float = @CALL@; print(to_string(f)); return 0 }")
	// nullary member.
	assertCoreByteIdentical(t, "math", "math.pi", "pi", ``,
		"fn main() -> int { let f: float = @CALL@; print(to_string(f)); return 0 }")
	// overloaded delegate, int form.
	assertCoreByteIdentical(t, "math", "math.clamp", "clamp", `5, 1, 10`,
		"fn main() -> int { let i: int = @CALL@; print(to_string(i)); return 0 }")
	// overloaded delegate, FLOAT form -- exercises posLiteral(args[0]) column
	// alignment (genClamp float path embeds the first-arg position).
	assertCoreByteIdentical(t, "math", "math.clamp", "clamp", `5.0, 1.0, 10.0`,
		"fn main() -> int { let f: float = @CALL@; print(to_string(f)); return 0 }")
	// abs int and float forms.
	assertCoreByteIdentical(t, "math", "math.abs", "abs", `-5`,
		"fn main() -> int { let i: int = @CALL@; print(to_string(i)); return 0 }")
	assertCoreByteIdentical(t, "math", "math.abs", "abs", `-5.0`,
		"fn main() -> int { let f: float = @CALL@; print(to_string(f)); return 0 }")
	// domain-checked delegate, valid value.
	assertCoreByteIdentical(t, "math", "math.random", "random", `5`,
		"fn main() -> int { let i: int = @CALL@; print(to_string(i)); return 0 }")
}

func TestCoreFSByteIdentical(t *testing.T) {
	// string result (fallible, embeds a position literal).
	assertCoreByteIdentical(t, "fs", "fs.read_file", "read_file", `"p"`,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
	// nullary member.
	assertCoreByteIdentical(t, "fs", "fs.cwd", "cwd", ``,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
	// Optional[string] result.
	assertCoreByteIdentical(t, "fs", "fs.which", "which", `"ls"`,
		"fn main() -> int { let o: Optional[string] = @CALL@; return 0 }")
	// void statement-position member.
	assertCoreByteIdentical(t, "fs", "fs.write_file", "write_file", `"p", "c"`,
		"fn main() -> int { @CALL@; return 0 }")
}

func TestCoreStringsByteIdentical(t *testing.T) {
	// coreSig single-arg string result.
	assertCoreByteIdentical(t, "string", "string.upper", "upper", `"x"`,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
	// coreSig multi-arg (exercises arg-column alignment).
	assertCoreByteIdentical(t, "string", "string.substring", "substring", `"abc", 0, 2`,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
	// coreSig string[] result. The result is consumed by string.join (same
	// namespace, so it resolves) to keep a non-trivial use of the array.
	assertCoreByteIdentical(t, "string", "string.split", "split", `"a,b", ","`,
		"fn main() -> int { let a: string[] = @CALL@; print(string.join(a, \"|\")); return 0 }")
	// overloaded delegate, string form.
	assertCoreByteIdentical(t, "string", "string.contains", "contains", `"ab", "a"`,
		"fn main() -> int { let b: bool = @CALL@; print(to_string(b)); return 0 }")
	assertCoreByteIdentical(t, "string", "string.index_of", "index_of", `"ab", "a"`,
		"fn main() -> int { let o: Optional[int] = @CALL@; return 0 }")
	// arg-domain delegate, valid value.
	assertCoreByteIdentical(t, "string", "string.repeat", "repeat", `"x", 3`,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
	// reverse -> reverse_string rename: the member path is longer than the builtin
	// key, so flatCall pads to keep positions aligned.
	assertCoreByteIdentical(t, "string", "string.reverse", "reverse_string", `"abc"`,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
}

func TestCoreDictByteIdentical(t *testing.T) {
	// generic delegate, Optional[V] result.
	assertCoreByteIdentical(t, "dict", "dict.get", "get", `d, "a"`,
		"fn main() -> int { let d: {string: int} = { \"a\": 1 }; let o: Optional[int] = @CALL@; return 0 }")
	// generic delegate, K[] result. Consumed by stays-flat length/to_string
	// (the dict namespace has no join member to reach here).
	assertCoreByteIdentical(t, "dict", "dict.keys", "keys", `d`,
		"fn main() -> int { let d: {string: int} = { \"a\": 1 }; let k: string[] = @CALL@; print(to_string(length(k))); return 0 }")
	// void delegate, statement position.
	assertCoreByteIdentical(t, "dict", "dict.clear", "clear", `d`,
		"fn main() -> int { let d: {string: int} = { \"a\": 1 }; @CALL@; return 0 }")
}

func TestCoreArraysByteIdentical(t *testing.T) {
	// The load-bearing funcref byte-identity proofs. bodyTmpl declares the helper
	// fns; both variants share them, only @CALL@ differs.
	mapBody := "fn dbl(x: int) -> int { return x * 2 }\n" +
		"fn main() -> int { let xs: int[] = [1, 2]; let ys: int[] = @CALL@; print(to_string(length(ys))); return 0 }"
	assertCoreByteIdentical(t, "array", "array.map", "map", `xs, dbl`, mapBody)

	filterBody := "fn is_even(x: int) -> bool { return x % 2 == 0 }\n" +
		"fn main() -> int { let xs: int[] = [1, 2]; let ys: int[] = @CALL@; print(to_string(length(ys))); return 0 }"
	assertCoreByteIdentical(t, "array", "array.filter", "filter", `xs, is_even`, filterBody)

	reduceBody := "fn add(a: int, b: int) -> int { return a + b }\n" +
		"fn main() -> int { let xs: int[] = [1, 2]; let s: int = @CALL@; print(to_string(s)); return 0 }"
	assertCoreByteIdentical(t, "array", "array.reduce", "reduce", `xs, 0, add`, reduceBody)

	sortByBody := "fn lt(a: int, b: int) -> bool { return a < b }\n" +
		"fn main() -> int { let xs: int[] = [2, 1]; let ys: int[] = @CALL@; print(to_string(length(ys))); return 0 }"
	assertCoreByteIdentical(t, "array", "array.sort_by", "sort_by", `xs, lt`, sortByBody)

	// composite int[] result.
	assertCoreByteIdentical(t, "array", "array.range", "range", `5`,
		"fn main() -> int { let xs: int[] = @CALL@; print(to_string(length(xs))); return 0 }")
	// void member, statement position.
	assertCoreByteIdentical(t, "array", "array.push", "push", `xs, 9`,
		"fn main() -> int { let xs: int[] = [1]; @CALL@; return 0 }")
	// array.contains delegates to the identical "contains" builtin key
	// string.contains uses -- same argsStr, byte-identical output.
	assertCoreByteIdentical(t, "array", "array.contains", "contains", `"ab", "a"`,
		"fn main() -> int { let b: bool = @CALL@; print(to_string(b)); return 0 }")
	// array.index_of delegates to the identical "index_of" builtin key
	// string.index_of uses -- same argsStr, byte-identical output.
	assertCoreByteIdentical(t, "array", "array.index_of", "index_of", `"ab", "a"`,
		"fn main() -> int { let o: Optional[int] = @CALL@; return 0 }")
}

func TestCoreProcessByteIdentical(t *testing.T) {
	// composite string[] arg, string result.
	assertCoreByteIdentical(t, "process", "process.run", "run", `["echo", "hi"]`,
		"fn main() -> int { let s: string = @CALL@; print(s); return 0 }")
	// special return type RunResult (field access uses flat form in both variants).
	assertCoreByteIdentical(t, "process", "process.run_full", "run_full", `["echo"]`,
		"fn main() -> int { let r: RunResult = @CALL@; print(r.stdout); return 0 }")
	// special return type Process (consumed by process.is_done, same namespace).
	assertCoreByteIdentical(t, "process", "process.spawn", "spawn", `["sleep", "1"]`,
		"fn main() -> int { let p: Process = @CALL@; let b: bool = process.is_done(p); print(to_string(b)); return 0 }")
	// coreSig member.
	assertCoreByteIdentical(t, "process", "process.pid_alive", "pid_alive", `123`,
		"fn main() -> int { let b: bool = @CALL@; print(to_string(b)); return 0 }")
}
