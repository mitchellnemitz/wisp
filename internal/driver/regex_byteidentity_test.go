package driver

import (
	"bytes"
	"testing"
)

// A program using `import "regex"` + `regex.<member>(...)` must compile to shell
// byte-identical to the pre-removal flat lowering, because the bridge records the
// same CallBuiltin key and codegen never sees the namespace. This is the
// non-breaking / equivalence proof for the regex core module (Unit 12).
//
// PR C removed the flat regex spellings (matches/regex_find/...), so this can no
// longer compile a live flat variant. Instead it compares the current namespaced
// lowering against a frozen baseline captured from the pre-removal commit
// de4ebeff (see regex_byteidentity_baseline.go), where the flat and namespaced
// lowerings were asserted byte-identical at capture time.
//
// Generated shell embeds source-map positions (file:line:col); the baseline was
// captured from the identical namespaced source template below, so positions
// already align.
func TestRegexNamespaceByteIdenticalToFlat(t *testing.T) {
	for _, c := range []struct {
		name string
		// bind: the `let ... = ` prefix; callee follows.
		bind   string
		nsCall string
		use    string // a line consuming the binding
	}{
		{"matches", "let b: bool = ", `regex.matches("a", "b")`, "print(to_string(b))"},
		{"find", "let o: Optional[string] = ", `regex.find("a", "b")`, "print(to_string(is_some(o)))"},
		{"find_all", "let a: string[] = ", `regex.find_all("a", "b")`, "print(to_string(length(a)))"},
		{"replace", "let s: string = ", `regex.replace("a", "b", "c")`, "print(s)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			nsSrc := "import \"regex\"\nfn main() -> int {\n  " + c.bind + c.nsCall + "\n  " + c.use + "\n  return 0\n}"

			nsScript := compileNoErr(t, "p.wisp", nsSrc)

			want, ok := regexByteIdentityBaseline[c.name]
			if !ok {
				t.Fatalf("no frozen baseline for regex.%s (regenerate regex_byteidentity_baseline.go)", c.name)
			}
			if !bytes.Equal(nsScript, []byte(want)) {
				t.Errorf("regex.%s no longer matches the frozen pre-removal baseline\n--- namespaced ---\n%s\n--- baseline ---\n%s",
					c.name, nsScript, want)
			}
		})
	}
}

// compileNoErr compiles src and fails on any error-severity diagnostic (unused
// locals and other warnings are non-gating and ignored). Returns the script.
func compileNoErr(t *testing.T, filename, src string) []byte {
	t.Helper()
	script, _, diags := Compile(filename, src)
	for _, d := range diags {
		if d.Severity == Error {
			t.Fatalf("%s: compile error: %s: %s", filename, d.Pos, d.Msg)
		}
	}
	return script
}
