package types_test

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/types"
)

func TestOverloadedFuncrefNames(t *testing.T) {
	names := types.OverloadedFuncrefNames()
	if len(names) != 7 {
		t.Fatalf("expected 7 overloaded funcref names, got %d: %v", len(names), names)
	}
	want := map[string]bool{
		"abs": true, "min": true, "max": true, "clamp": true,
		"sign": true, "contains": true, "index_of": true,
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected overloaded funcref name %q", n)
		}
	}
}

func TestGenericFuncrefNames(t *testing.T) {
	names := types.GenericFuncrefNames()
	if len(names) != 12 {
		t.Fatalf("expected 12 generic funcref names, got %d: %v", len(names), names)
	}
	want := map[string]bool{
		"map": true, "filter": true, "each": true, "reduce": true,
		"sort_by": true, "find": true, "any": true, "all": true,
		"count_where": true, "and_then": true, "or_else": true, "map_err": true,
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected generic funcref name %q", n)
		}
	}
}
