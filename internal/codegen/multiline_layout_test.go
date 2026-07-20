package codegen

import "testing"

// TestMultilineLayoutCodegenIdentical confirms that a collection/struct literal
// compiles to byte-identical shell whether written single-line or multi-line:
// the multi-line layout is surface-only (it lives in fmt), not semantic. (R7/N2.)
//
// The single-line variants are padded with blank lines so the trailing
// index/field-access expression lands on the SAME source line in both forms;
// otherwise the runtime bounds-check error string (which legitimately embeds the
// access's `file:line:col`) would differ purely because the literal occupies a
// different number of physical lines. The padding isolates the test to the
// codegen of the literal itself, which is what N2 is about.
func TestMultilineLayoutCodegenIdentical(t *testing.T) {
	cases := []struct {
		name, single, multi string
	}{
		{
			name:   "array",
			single: "fn main() -> int {\n    let xs: int[] = [1, 2, 3]\n\n\n\n\n    return xs[0]\n}\n",
			multi:  "fn main() -> int {\n    let xs: int[] = [\n        1,\n        2,\n        3,\n    ]\n    return xs[0]\n}\n",
		},
		{
			name:   "dict",
			single: "fn main() -> int {\n    let d: {string: int} = { \"a\": 1, \"b\": 2 }\n\n\n\n    return d[\"a\"]\n}\n",
			multi:  "fn main() -> int {\n    let d: {string: int} = {\n        \"a\": 1,\n        \"b\": 2,\n    }\n    return d[\"a\"]\n}\n",
		},
		{
			name:   "struct",
			single: "struct Point { x: int, y: int }\nfn main() -> int {\n    let p: Point = Point { x: 1, y: 2 }\n\n\n\n    return p.x\n}\n",
			multi:  "struct Point { x: int, y: int }\nfn main() -> int {\n    let p: Point = Point {\n        x: 1,\n        y: 2,\n    }\n    return p.x\n}\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			single := string(compile(t, c.single))
			multi := string(compile(t, c.multi))
			if single != multi {
				t.Errorf("%s: codegen differs between single- and multi-line layout\n--single--\n%s\n--multi--\n%s", c.name, single, multi)
			}
		})
	}
}
