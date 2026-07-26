// internal/fuzz/builtins_test.go
package fuzz

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/types"
)

func fullCatalog() map[string]bool {
	cat := map[string]bool{}
	for _, n := range types.BuiltinNames() {
		cat[n] = true
	}
	for _, ns := range types.CoreNamespaces() {
		for _, m := range types.CoreMembers(ns) {
			cat[ns+"."+m] = true
		}
	}
	return cat
}

func TestAdmittedBuiltinsExistInCatalog(t *testing.T) {
	cat := fullCatalog()
	for _, b := range admittedBuiltins {
		if !cat[b.Name] {
			t.Errorf("admitted builtin %q not in catalog", b.Name)
		}
		if b.Family == "" {
			t.Errorf("admitted builtin %q has no family (SC-009 needs family witnesses)", b.Name)
		}
	}
}

func TestAdmittedSetIsExactlyCatalogMinusExcluded(t *testing.T) {
	// FR-004 + documented non-generable exclusion: admitted == catalog minus
	// (effectful ∪ non-generable). Forces completeness AND freezes the snapshot.
	cat := fullCatalog()
	admitted := map[string]bool{}
	for _, b := range admittedBuiltins {
		admitted[b.Name] = true
	}
	for name := range cat {
		if isExcluded(name) {
			continue
		}
		if !admitted[name] {
			t.Errorf("pure, generable catalog builtin %q is not admitted", name)
		}
	}
	for name := range admitted {
		if isExcluded(name) {
			t.Errorf("admitted builtin %q is on an exclusion list", name)
		}
	}
}
