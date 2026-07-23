package codegen

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenericComparableStringNoAwkVChannel(t *testing.T) {
	src := `fn eq2[T: comparable](a: T, b: T) -> bool { return a == b }
fn main() -> int { print("${eq2("x", "y")}"); return 0 }`
	sh := string(compile(t, src))
	// The erased (string) body must compare via a quoted text test, never feed a
	// non-float operand to __wisp_fcmp's awk -v channel.
	if strings.Contains(sh, "__wisp_fcmp") {
		t.Errorf("string comparable body must not use __wisp_fcmp (awk -v channel); got:\n%s", sh)
	}
}

func TestGenericComparableBoolNoAwkVChannel(t *testing.T) {
	src := `fn eq2[T: comparable](a: T, b: T) -> bool { return a == b }
fn main() -> int { print("${eq2(true, false)}"); return 0 }`
	sh := string(compile(t, src))
	// SC-028(ii): the bool instantiation shares the erased text-compare body; no
	// __wisp_fcmp on the bool call path.
	if strings.Contains(sh, "__wisp_fcmp") {
		t.Errorf("bool comparable body must not use __wisp_fcmp; got:\n%s", sh)
	}
}

func TestComparableGenericFloatOnlyNoErasedBody(t *testing.T) {
	// eq2 is called ONLY at float, so every call specializes to the __T_float
	// instance and needsErasedBody[eq2] stays false: the base-mangled erased body
	// must NOT be emitted.
	src := `fn eq2[T: comparable](a: T, b: T) -> bool { return a == b }
fn main() -> int { print("${eq2(1.0, 1.00)}"); return 0 }`
	sh := string(compile(t, src))
	if !strings.Contains(sh, "__wisp_fcmp") {
		t.Fatalf("float instance must use __wisp_fcmp; got:\n%s", sh)
	}
	re := regexp.MustCompile(`(?m)^(__wisp_f_[A-Za-z0-9_]*eq2[A-Za-z0-9_]*)\(\) \{`)
	var defs []string
	for _, m := range re.FindAllStringSubmatch(sh, -1) {
		defs = append(defs, m[1])
	}
	if len(defs) != 1 {
		t.Fatalf("expected exactly one eq2 definition (the float instance), got %v in:\n%s", defs, sh)
	}
	if !strings.HasSuffix(defs[0], "_float") {
		t.Fatalf("the sole eq2 definition must be the float instance (suffix _float), got %q", defs[0])
	}
}

func TestComparableGenericMultiParamDistinctInstances(t *testing.T) {
	// The two partial-float specializations MUST get distinct param-name-qualified
	// names (__A_float vs __B_float), and because an all-non-float call exists the
	// bare erased base body MUST also be emitted -- three distinct pair_eq bodies.
	src := `fn pair_eq[A: comparable, B: comparable](a1: A, a2: A, b1: B, b2: B) -> bool { return a1 == a2 && b1 == b2 }
fn main() -> int {
  print("${pair_eq(1.0, 1.00, "x", "x")}")
  print("${pair_eq("p", "p", 2.5, 2.50)}")
  print("${pair_eq("m", "m", "n", "n")}")
  return 0
}`
	sh := string(compile(t, src))
	re := regexp.MustCompile(`(?m)^(__wisp_f_[A-Za-z0-9_]*pair_eq[A-Za-z0-9_]*)\(\) \{`)
	defs := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(sh, -1) {
		defs[m[1]] = true
	}
	var aFloat, bFloat, base int
	for name := range defs {
		switch {
		case strings.HasSuffix(name, "__A_float"):
			aFloat++
		case strings.HasSuffix(name, "__B_float"):
			bFloat++
		default:
			base++ // the bare erased body carries no comparable-float suffix
		}
	}
	if aFloat != 1 || bFloat != 1 || base != 1 {
		t.Fatalf("expected exactly one each of __A_float, __B_float, and the erased base pair_eq body; got %v", defs)
	}
}

func TestNestedComparableFloatOnlyNoErasedBody(t *testing.T) {
	src := `fn inner[T: comparable](a: T, b: T) -> bool { return a == b }
fn outer[U: comparable](x: U, y: U) -> bool { return inner(x, y) }
fn main() -> int { print("${outer(1.0, 1.00)}"); return 0 }`
	sh := string(compile(t, src))
	if !strings.Contains(sh, "__wisp_fcmp") {
		t.Fatalf("nested float instance must use __wisp_fcmp; got:\n%s", sh)
	}
	headers := func(name string) []string {
		re := regexp.MustCompile(`(?m)^(__wisp_f_[A-Za-z0-9_]*` + name + `[A-Za-z0-9_]*)\(\) \{`)
		var out []string
		for _, m := range re.FindAllStringSubmatch(sh, -1) {
			out = append(out, m[1])
		}
		return out
	}
	for _, name := range []string{"inner", "outer"} {
		defs := headers(name)
		if len(defs) != 1 {
			t.Fatalf("%s: expected exactly one (float) definition, got %v in:\n%s", name, defs, sh)
		}
		if !strings.HasSuffix(defs[0], "_float") {
			t.Fatalf("%s: sole definition must be the float instance (suffix _float), got %q", name, defs[0])
		}
	}
}
