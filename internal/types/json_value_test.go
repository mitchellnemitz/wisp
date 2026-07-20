package types

import "testing"

// jsonValueOpaqueCase wraps a body that binds `let v: json.Value = json.null()`
// and then does something illegal with v.
func jsonValueOpaqueProg(body string) string {
	return `fn main() -> int {` + "\n" +
		`  let v: json.Value = json.null()` + "\n" +
		body + "\n" +
		`  return 0` + "\n" +
		`}`
}

func TestJSONValueRejectsArithmetic(t *testing.T) {
	info := checkJSONProg(t, jsonValueOpaqueProg(`  let n: int = v + 1`))
	if len(info.Errors) == 0 {
		t.Fatal("expected an error adding to a json.Value")
	}
}

func TestJSONValueRejectsComparison(t *testing.T) {
	info := checkJSONProg(t, jsonValueOpaqueProg(`  let b: bool = v == v`))
	if !hasErr(info, "opaque and cannot be compared") {
		t.Fatalf("want opaque-compare error, got %v", errMsgs(info))
	}
}

func TestJSONValueRejectsInterpolation(t *testing.T) {
	info := checkJSONProg(t, jsonValueOpaqueProg(`  let s: string = "${v}"`))
	if !hasErr(info, "cannot interpolate a json.Value") {
		t.Fatalf("want interpolation error, got %v", errMsgs(info))
	}
}

func TestJSONValueRejectsStringConversion(t *testing.T) {
	info := checkJSONProg(t, jsonValueOpaqueProg(`  let s: string = to_string(v)`))
	if len(info.Errors) == 0 {
		t.Fatal("expected an error from to_string(json.Value)")
	}
}

func TestJSONValueRejectsFieldAccess(t *testing.T) {
	info := checkJSONProg(t, jsonValueOpaqueProg(`  let x: int = v.field`))
	if len(info.Errors) == 0 {
		t.Fatal("expected an error accessing a field on a json.Value")
	}
}

func TestJSONValueStructLiteralRejected(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int { let v: json.Value = json.Value { }; return 0 }`)
	if !hasErr(info, "json.Value is not a struct type") {
		t.Fatalf("want not-a-struct-type error, got %v", errMsgs(info))
	}
}

func TestJSONValueCompositeElementTypes(t *testing.T) {
	// Optional[json.Value] (get), json.Value[] (array), dict[string,json.Value]
	// (object) are all valid element/container types.
	src := `fn main() -> int {` + "\n" +
		`  let a: json.Value[] = [json.from_int(1), json.from_int(2)]` + "\n" +
		`  let arr: json.Value = json.array(a)` + "\n" +
		`  let o: Optional[json.Value] = json.at(arr, 0)` + "\n" +
		`  return 0` + "\n" +
		`}`
	info := checkJSONProg(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("composite json.Value types should check clean; errors: %v", errMsgs(info))
	}
}
