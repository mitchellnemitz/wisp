package docslint_test

import (
	"os"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/docslint"
	"github.com/mitchellnemitz/wisp/internal/types"
)

const (
	stdlibIndexPath = "../../docs/stdlib-index.md"
	languageMdPath  = "../../docs/guide/language.md"
)

func TestStdlibIndexNoStaleBareSpelling(t *testing.T) {
	data, err := os.ReadFile(stdlibIndexPath)
	if err != nil {
		t.Fatalf("reading %s: %v", stdlibIndexPath, err)
	}
	offenses := docslint.StaleBareSpellings(string(data), types.RemovableBuiltins())
	for _, o := range offenses {
		t.Errorf("%s:%d: stale bare spelling %q; use its RemovedHint()-qualified form", stdlibIndexPath, o.Line, o.Name)
	}
}

func TestLanguageMdFuncrefClassesAccurate(t *testing.T) {
	data, err := os.ReadFile(languageMdPath)
	if err != nil {
		t.Fatalf("reading %s: %v", languageMdPath, err)
	}
	mono, overloaded, generic, err := docslint.LanguageMdFuncrefExamples(string(data))
	if err != nil {
		t.Fatalf("extracting funcref examples: %v", err)
	}

	generatable := types.GeneratableBuiltinFuncrefs()
	for _, name := range mono {
		if !generatable[name] {
			t.Errorf("language.md lists %q as monomorphic-generatable, but it is not in GeneratableBuiltinFuncrefs()", name)
		}
	}

	overloadedSet := map[string]bool{}
	for _, n := range types.OverloadedFuncrefNames() {
		overloadedSet[n] = true
	}
	for _, name := range overloaded {
		if !overloadedSet[name] {
			t.Errorf("language.md lists %q as overloaded, but it is not in OverloadedFuncrefNames()", name)
		}
	}

	genericSet := map[string]bool{}
	for _, n := range types.GenericFuncrefNames() {
		genericSet[n] = true
	}
	for _, name := range generic {
		if !genericSet[name] {
			t.Errorf("language.md lists %q as generic, but it is not in GenericFuncrefNames()", name)
		}
	}
}

func TestDocslintGuard_ExecutableNegative(t *testing.T) {
	syntheticDoc := "## Arrays\n\n- `[x] push(a: T[], v: T) -> T[]`\n"
	offenses := docslint.StaleBareSpellings(syntheticDoc, types.RemovableBuiltins())
	found := false
	for _, o := range offenses {
		if o.Name == "push" && o.Line == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("StaleBareSpellings did not detect synthetic drift for %q; got %v", "push", offenses)
	}
}
