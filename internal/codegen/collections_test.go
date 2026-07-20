package codegen

import (
	"testing"
)

// Most collections runtime behavioral coverage moved to internal/golden
// (coll_arrays, coll_dicts, coll_first_empty, coll_slice_caught, coll_tail_*,
// dict_*), with byte-shape coverage for the funcref-bearing arms in
// core_byteidentity_test.go (TestCoreArraysByteIdentical, TestCoreDictByteIdentical).
// The array.any / array.all short-circuit behavior and array.sort_by's
// stable/bounded runtime property (which the golden harness cannot express) are
// reconstructed below with the namespaced array.* spelling; delegation lowers
// each byte-identically to the pre-removal flat call, so the runtime behavior is
// unchanged.

// Shared top-level helpers for the collections programs (kept out of main).
const collHelpers = `
fn str(x: int) -> string { return to_string(x) }
fn fstr(x: float) -> string { return to_string(x) }
fn ids(s: string) -> string { return s }
fn lt(a: int, b: int) -> bool { return a < b }
fn always(a: int, b: int) -> bool { return true }
fn pos(x: int) -> bool { return x > 0 }
fn isBig(x: int) -> bool { return x > 2 }
fn matchOne(x: int) -> bool { return 10 / x == 10 }
fn nine(x: int) -> bool { return x == 9 }
`

// collOut compiles the program (with the array and string namespaces bound),
// runs it, and returns stdout.
func collOut(t *testing.T, body string) string {
	t.Helper()
	out, errb, code := runNS(t, "fn main() -> int {\n"+body+"\nreturn 0\n}\n"+collHelpers, "array", "string")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	return out
}

// TestCollSortByStableAndBounded: sort_by is stable, and a pathological
// (always-true) comparator must still terminate with a safe permutation.
func TestCollSortByStableAndBounded(t *testing.T) {
	out := collOut(t, `
let xs: int[] = [3, 1, 2, 1]
let s: int[] = array.sort_by(xs, lt)
print(string.join(array.map(s, str), ","))
let p: int[] = array.sort_by(xs, always)
print(to_string(length(p)))
print(string.join(array.map(array.sort(p), str), ","))`)
	want := "1,1,2,3\n4\n1,1,2,3\n"
	if out != want {
		t.Errorf("sort_by = %q, want %q (pathological comparator must terminate with a safe permutation)", out, want)
	}
}

// TestCollFindAnyAllShortCircuit: find/any/all short-circuit -- the decisive
// element comes first, so a correct short-circuit never evaluates the divide-by-
// zero comparator on 0 and the program does not abort.
func TestCollFindAnyAllShortCircuit(t *testing.T) {
	out := collOut(t, `
let a: int[] = [1, 0]
print(to_string(unwrap_or(array.find(a, matchOne), -1)))
print(to_string(array.any(a, matchOne)))
let z: int[] = [0]
print(to_string(array.all(z, nine)))`)
	want := "0\ntrue\nfalse\n"
	if out != want {
		t.Errorf("find/any/all short-circuit = %q, want %q", out, want)
	}
}

// TestCollFindAnyAllEmpty: empty-base-case identities (find->None, any->false,
// all->true).
func TestCollFindAnyAllEmpty(t *testing.T) {
	out := collOut(t, `
let e: int[] = []
print(to_string(unwrap_or(array.find(e, pos), -1)))
print(to_string(array.any(e, pos)))
print(to_string(array.all(e, pos)))`)
	if out != "-1\nfalse\ntrue\n" {
		t.Errorf("empty base cases = %q", out)
	}
}
