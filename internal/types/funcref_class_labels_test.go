package types

import "testing"

// TestFuncrefClassLabels is a defensive/exhaustiveness check on
// FuncrefClassLabels(): it must list exactly the 8 declared
// BuiltinFuncrefClass constants (no duplicates), and every classification
// BuiltinFuncrefClassOf ever actually returns over the full builtin set must
// be one of them. If an 9th class constant is ever added to
// funcref_class.go without adding it here, this test catches the omission
// via the exhaustiveness half (a name would classify into a label not in
// the list).
func TestFuncrefClassLabels(t *testing.T) {
	labels := FuncrefClassLabels()
	if len(labels) != 8 {
		t.Fatalf("FuncrefClassLabels() = %d labels, want 8: %v", len(labels), labels)
	}
	seen := make(map[BuiltinFuncrefClass]bool, len(labels))
	for _, l := range labels {
		if seen[l] {
			t.Errorf("FuncrefClassLabels() has duplicate %q", l)
		}
		seen[l] = true
	}
	for name := range builtinSigs {
		class := BuiltinFuncrefClassOf(name)
		if !seen[class] {
			t.Errorf("BuiltinFuncrefClassOf(%q) = %q, not present in FuncrefClassLabels()", name, class)
		}
	}
}
