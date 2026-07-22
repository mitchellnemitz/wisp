package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// enumValues resolves an enum by source name in the root module and returns its
// ordered (variant -> value) mapping for assertions.
func enumValues(t *testing.T, info *Info, name string) map[string]int64 {
	t.Helper()
	ei, ok := info.Enums[string(internalEnumName(name, 0))]
	if !ok {
		t.Fatalf("enum %q not registered; have %v", name, info.Enums)
	}
	out := map[string]int64{}
	for i, vn := range ei.Variants {
		out[vn] = ei.Consts[i].(int64)
	}
	return out
}

func TestEnum_ValueResolution_Default(t *testing.T) {
	info := expectOK(t, `enum Color: int { Red, Green, Blue }
fn main() -> int { return 0 }`)
	got := enumValues(t, info, "Color")
	want := map[string]int64{"Red": 0, "Green": 1, "Blue": 2}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("variant %s = %d, want %d", k, got[k], v)
		}
	}
}

func TestEnum_ValueResolution_ExplicitReseed(t *testing.T) {
	info := expectOK(t, `enum E: int { A = 5, B, C }
fn main() -> int { return 0 }`)
	got := enumValues(t, info, "E")
	want := map[string]int64{"A": 5, "B": 6, "C": 7}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("variant %s = %d, want %d", k, got[k], v)
		}
	}
}

func TestEnum_ValueResolution_NegativeExplicit(t *testing.T) {
	info := expectOK(t, `enum E2: int { X = -1, Y }
fn main() -> int { return 0 }`)
	got := enumValues(t, info, "E2")
	if got["X"] != -1 || got["Y"] != 0 {
		t.Fatalf("got X=%d Y=%d, want X=-1 Y=0", got["X"], got["Y"])
	}
}

func TestEnum_DuplicateValueAfterAutoIncrement(t *testing.T) {
	expectErr(t, `enum E: int { A = 1, B = 0, C }
fn main() -> int { return 0 }`, "duplicate")
}

func TestEnum_NonLiteralExplicitValue(t *testing.T) {
	expectErr(t, `enum E: int { A = 1 + 1 }
fn main() -> int { return 0 }`, "integer literal")
}

func TestEnum_DuplicateVariantName(t *testing.T) {
	expectErr(t, `enum E: int { A, A }
fn main() -> int { return 0 }`, "more than once")
}

func TestEnum_NameCollidesWithStruct(t *testing.T) {
	expectErr(t, `struct Color { x: int }
enum Color: int { Red }
fn main() -> int { return 0 }`, "Color")
}

func TestEnum_VariantAccessTypesAndFolds(t *testing.T) {
	info := expectOK(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let c: Color = Color.Green
	return 0
}`)
	// Find the Color.Green FieldAccess node and assert its folded value.
	var found bool
	for node, fv := range info.FoldedValues {
		fa, ok := node.(*ast.FieldAccess)
		if !ok {
			continue
		}
		id, ok := fa.X.(*ast.Ident)
		if !ok || id.Name != "Color" || fa.Field != "Green" {
			continue
		}
		found = true
		iv, ok := fv.(int64)
		if !ok || iv != 1 {
			t.Fatalf("Color.Green folded to %#v, want int64(1)", fv)
		}
		if got := info.Types[node]; got != internalEnumName("Color", 0) {
			t.Fatalf("Color.Green typed to %q, want enum Color token", got)
		}
	}
	if !found {
		t.Fatal("Color.Green FieldAccess not found in FoldedValues")
	}
}

func TestEnum_UnknownVariant(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let c: Color = Color.Nope
	return 0
}`, "Nope")
}

func TestEnum_SameEnumComparisonOK(t *testing.T) {
	expectOK(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let c: Color = Color.Green
	let b: bool = c == Color.Red
	let n: bool = c != Color.Blue
	return 0
}`)
}

func TestEnum_VsIntComparisonError(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let c: Color = Color.Green
	let b: bool = c == 1
	return 0
}`, "same type")
}

func TestEnum_CrossEnumComparisonError(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green }
enum Mood: int { Sad, Glad }
fn main() -> int {
	let c: Color = Color.Red
	let b: bool = c == Mood.Sad
	return 0
}`, "same type")
}

func TestEnum_ArithmeticError(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let c: Color = Color.Red + Color.Green
	return 0
}`, "+")
}

func TestEnum_AssignIntError(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let c: Color = 1
	return 0
}`, "Color")
}

func TestEnum_IntConversion(t *testing.T) {
	expectOK(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let n: int = to_int(Color.Blue)
	return n
}`)
}

func TestEnum_StringConversionError(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let s: string = to_string(Color.Red)
	return 0
}`, "enum")
}

func TestEnum_DebugError(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
	let s: string = debug(Color.Red)
	return 0
}`, "enum")
}

func TestEnum_LocalShadowsEnum(t *testing.T) {
	// A local variable named Color shadows the enum-type interpretation, so
	// Color.Green is a field access on the local (a non-struct), not a variant.
	expectErr(t, `enum Color: int { Red, Green, Blue }
struct Pair { Green: int }
fn main() -> int {
	let Color: int = 3
	let x: int = Color.Green
	return 0
}`, "non-struct")
}

func TestEnum_VariantInConstInitializer(t *testing.T) {
	// A bare variant access is itself a constant expression of the enum type, so it
	// is a valid const/final initializer (R3: foldConst folds it unconditionally).
	info := expectOK(t, `enum Color: int { Red, Green, Blue }
const C: Color = Color.Blue
fn main() -> int { return to_int(C) }`)
	if got, ok := info.ConstTable["C"]; !ok {
		t.Fatal("const C not in ConstTable")
	} else if got.Value.(int64) != 2 {
		t.Fatalf("const C folded to %#v, want int64(2)", got.Value)
	}
}
