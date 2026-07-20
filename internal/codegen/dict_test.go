package codegen

import (
	"strings"
	"testing"
)

// --- M3 PR-C: dict codegen (executed under dash) ---
//
// dict.has / dict.keys are removable builtins (bare has/keys no longer resolve
// in the single-module check), so every test below that uses them compiles
// through runNS with the dict namespace bound (array.push similarly needs the
// array namespace in TestDictNestedValueArray).

func TestDictLiteralLookupHasSetKeys(t *testing.T) {
	out, errb, code := runNS(t, `fn main() -> int {
  let m: {string: int} = { "a": 1, "b": 2 }
  m["c"] = 3
  print("a=${m["a"]}")
  print("hasb=${to_string(dict.has(m, "b"))}")
  print("hasz=${to_string(dict.has(m, "z"))}")
  print("nkeys=${length(dict.keys(m))}")
  return 0
}`, "dict")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "a=1\nhasb=true\nhasz=false\nnkeys=3\n" {
		t.Errorf("out=%q", out)
	}
}

func TestDictForInInsertionOrder(t *testing.T) {
	// Insert out of sorted order; for-in must yield insertion order, not sorted.
	out, _, code := runWisp(t, `fn main() -> int {
  let m: {string: int} = {}
  m["zebra"] = 1
  m["apple"] = 2
  m["mango"] = 3
  for (k in m) { print(k) }
  return 0
}`)
	if code != 0 || out != "zebra\napple\nmango\n" {
		t.Errorf("out=%q code=%d, want insertion order zebra/apple/mango", out, code)
	}
}

func TestDictKeysInsertionOrder(t *testing.T) {
	out, _, code := runNS(t, `fn main() -> int {
  let m: {string: int} = {}
  m["c"] = 1
  m["a"] = 2
  m["b"] = 3
  let ks: string[] = dict.keys(m)
  for (k in ks) { print(k) }
  return 0
}`, "dict")
	if code != 0 || out != "c\na\nb\n" {
		t.Errorf("out=%q code=%d, want c/a/b", out, code)
	}
}

func TestDictOverwriteKeepsPosition(t *testing.T) {
	// Overwriting an existing key changes its value but NOT its insertion position.
	out, _, code := runWisp(t, `fn main() -> int {
  let m: {string: int} = {}
  m["x"] = 1
  m["y"] = 2
  m["x"] = 99
  for (k in m) { print("${k}=${m[k]}") }
  return 0
}`)
	if code != 0 || out != "x=99\ny=2\n" {
		t.Errorf("out=%q code=%d, want x=99 then y=2 (x keeps first position)", out, code)
	}
}

func TestDictMissingKeyLocatedAbort(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  let m: {string: int} = { "a": 1 }
  print(to_string(m["nope"]))
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit=%d, want 1", code)
	}
	if !strings.Contains(errb, "dict key not found") || !strings.Contains(errb, ":3:") {
		t.Errorf("stderr=%q, want located missing-key abort at line 3", errb)
	}
}

func TestDictShellActiveKeyInert(t *testing.T) {
	// A key containing $(...), backtick, ;, space, and quotes must be stored and
	// retrieved verbatim and INERTLY (no command execution).
	out, errb, code := runWisp(t, `fn main() -> int {
  let m: {string: int} = {}
  m["$(echo PWN); `+"`id`"+` 'q' \"d\""] = 42
  print("v=${m["$(echo PWN); `+"`id`"+` 'q' \"d\""]}")
  for (k in m) { print("k=${k}") }
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	wantKey := "$(echo PWN); `id` 'q' \"d\""
	if out != "v=42\nk="+wantKey+"\n" {
		t.Errorf("out=%q, want v=42 and the key echoed verbatim", out)
	}
	if strings.Contains(out, "PWN\n") || strings.Contains(out, "uid=") {
		t.Errorf("injection: key was evaluated, out=%q", out)
	}
}

func TestDictNamespaceOverlapKeys(t *testing.T) {
	// Keys "len" and "0" must not collide with the array _len / element vars or
	// with each other; values round-trip independently.
	out, _, code := runNS(t, `fn main() -> int {
  let m: {string: int} = {}
  m["len"] = 111
  m["0"] = 222
  m["keys"] = 333
  print("len=${m["len"]}")
  print("z=${m["0"]}")
  print("keys=${m["keys"]}")
  print("n=${length(dict.keys(m))}")
  return 0
}`, "dict")
	if code != 0 || out != "len=111\nz=222\nkeys=333\nn=3\n" {
		t.Errorf("out=%q code=%d, want no namespace collision", out, code)
	}
}

func TestDictDistinctKeysNoCollision(t *testing.T) {
	out, _, code := runNS(t, `fn main() -> int {
  let m: {string: int} = {}
  m["a"] = 1
  m["b"] = 2
  m["ab"] = 3
  m["ba"] = 4
  print("${m["a"]}${m["b"]}${m["ab"]}${m["ba"]}")
  print("n=${length(dict.keys(m))}")
  return 0
}`, "dict")
	if code != 0 || out != "1234\nn=4\n" {
		t.Errorf("out=%q code=%d, want 1234 and 4 distinct keys", out, code)
	}
}

func TestDictIntKeyRoundTripAndArithmetic(t *testing.T) {
	// Build an int-keyed dict, iterate (decode re-validates via __wisp_int), do
	// arithmetic on a decoded key, and look up by an int value -- the key stays
	// int-valid throughout.
	out, _, code := runNS(t, `fn main() -> int {
  let m: {int: string} = { 10: "ten", 20: "twenty" }
  m[30] = "thirty"
  let total: int = 0
  for (k in m) { total = total + k }
  print("sum=${total}")
  print("doubled=${total * 2}")
  let ks: int[] = dict.keys(m)
  print("k0=${ks[0]}")
  print("at20=${m[20]}")
  return 0
}`, "dict")
	if code != 0 || out != "sum=60\ndoubled=120\nk0=10\nat20=twenty\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestDictIntKeyCanonicalization(t *testing.T) {
	// 05 and 5 and +5 are the same int key; setting via different spellings hits
	// one entry (canonicalized through __wisp_int before encoding).
	out, _, code := runNS(t, `fn main() -> int {
  let m: {int: int} = {}
  m[5] = 1
  m[05] = 2
  print("v=${m[5]}")
  print("n=${length(dict.keys(m))}")
  return 0
}`, "dict")
	if code != 0 || out != "v=2\nn=1\n" {
		t.Errorf("out=%q code=%d, want v=2 n=1 (5 and 05 are one key)", out, code)
	}
}

func TestDictNestedValueArray(t *testing.T) {
	// {string: int[]} one level: a dict whose values are arrays.
	out, _, code := runNS(t, `fn main() -> int {
  let m: {string: int[]} = { "a": [1, 2], "b": [3, 4, 5] }
  print("alen=${length(m["a"])}")
  print("b2=${m["b"][2]}")
  array.push(m["a"], 9)
  print("alen2=${length(m["a"])}")
  return 0
}`, "array")
	if code != 0 || out != "alen=2\nb2=5\nalen2=3\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestDictForInBlockScopedSiblingReuse(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  let m: {string: int} = { "a": 1 }
  let n: {string: int} = { "b": 2 }
  for (k in m) { print(k) }
  for (k in n) { print(k) }
  return 0
}`)
	if code != 0 || out != "a\nb\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

// TestDictSourceMapCoversDictLines asserts the source map (M2) attributes the
// generated dict-set line back to its wisp source line (spec 4.8 / cross-cutting).
func TestDictSourceMapCoversDictLines(t *testing.T) {
	src := `fn main() -> int {
  let m: {string: int} = {}
  m["a"] = 7
  return 0
}`
	script, lm := compileWithMap(t, src)
	lines := strings.Split(strings.TrimSuffix(string(script), "\n"), "\n")
	// The dict-set emits an entry-var assignment via eval; find a line that writes
	// a __wisp_d_ backing var and assert it maps to the m["a"] = 7 source line (3).
	var found bool
	for i, ln := range lines {
		if strings.Contains(ln, "__wisp_d_") && strings.Contains(ln, "_k") && strings.Contains(ln, "eval") {
			if lm[i] != nil && lm[i].Line == 3 {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no dict-set line maps to source line 3 in:\n%s", script)
	}
}

func TestDictEmptyLiteral(t *testing.T) {
	out, _, code := runNS(t, `fn main() -> int {
  let m: {string: int} = {}
  print("n=${length(dict.keys(m))}")
  print("has=${to_string(dict.has(m, "x"))}")
  return 0
}`, "dict")
	if code != 0 || out != "n=0\nhas=false\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}
