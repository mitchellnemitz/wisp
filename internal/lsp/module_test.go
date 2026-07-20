package lsp

import "testing"

// TestDiagnosticsModuleFreeNoRegression: a module-free buffer produces the same
// diagnostics as before M8 (a real type error is still reported).
func TestDiagnosticsModuleFreeNoRegression(t *testing.T) {
	// A bool condition error: regression check that ordinary diagnostics still fire.
	diags := computeDiagnostics("fn main() -> int {\n  while (1) {\n  }\n  return 0\n}\n")
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic for a non-bool while condition")
	}
}

// TestDiagnosticsCleanModuleFree: a valid module-free program yields no
// diagnostics (no spurious suppression side effects).
func TestDiagnosticsCleanModuleFree(t *testing.T) {
	diags := computeDiagnostics("fn main() -> int {\n  return 0\n}\n")
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

// TestDiagnosticsSuppressQualifiedAlias: a buffer that uses an import/include
// alias must not surface a spurious unresolved-namespace error on the qualified
// reference (the LSP cannot resolve the project; spec section 6 defers that).
func TestDiagnosticsSuppressQualifiedAlias(t *testing.T) {
	src := "import \"a/b\" as b\n" +
		"include \"./util.wisp\" as u\n" +
		"fn main() -> int {\n" +
		"  print(b.greet())\n" +
		"  u.log()\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	for _, d := range diags {
		// No "undeclared name" / namespace error about b or u.
		if (containsAll(d.Message, "undeclared name", "\"b\"")) ||
			(containsAll(d.Message, "undeclared name", "\"u\"")) ||
			containsSub(d.Message, "module namespace") {
			t.Errorf("spurious qualified-alias diagnostic not suppressed: %q", d.Message)
		}
	}
}

// TestDiagnosticsBareAliasMisuseNotSuppressed: using an alias as a bare value
// (not as a qualifier) is a genuine error and must NOT be suppressed.
func TestDiagnosticsBareAliasMisuseNotSuppressed(t *testing.T) {
	src := "import \"a/b\" as lib\n" +
		"fn main() -> int {\n" +
		"  let x: int = lib\n" +
		"  return x\n" +
		"}\n"
	diags := computeDiagnostics(src)
	var found bool
	for _, d := range diags {
		if containsSub(d.Message, "lib") &&
			(containsSub(d.Message, "undeclared") || containsSub(d.Message, "module namespace")) {
			found = true
		}
	}
	if !found {
		t.Errorf("bare-alias misuse should still be reported, got %v", diags)
	}
}

func containsSub(s, sub string) bool {
	return len(sub) == 0 || indexSub(s, sub) >= 0
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsSub(s, sub) {
			return false
		}
	}
	return true
}

func indexSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
