// internal/fuzz/value_test.go
package fuzz

import (
	"strings"
	"testing"
)

func TestGenStringValueCoversAllPayloadCategories(t *testing.T) {
	want := []Category{CatCmdSubst, CatNewline, CatFormatPct, CatBackslash, CatLeadDash, CatGlob}
	var seen CategorySet
	r := NewRNG(12345)
	for i := 0; i < 5000; i++ {
		var cats CategorySet
		genStringValue(r, &cats)
		seen |= cats
	}
	for _, c := range want {
		if !seen.Has(c) {
			t.Errorf("payload category %d never generated in 5000 draws", c)
		}
	}
}

func TestGenIntValueEmitsEdgeValues(t *testing.T) {
	r := NewRNG(999)
	sawMin, sawMax := false, false
	for i := 0; i < 5000; i++ {
		var cats CategorySet
		lit := genIntValue(r, &cats).(*IntLit)
		switch lit.V {
		case "-9223372036854775808":
			sawMin = true
		case "9223372036854775807":
			sawMax = true
		}
	}
	if !sawMin {
		t.Error("INT_MIN never emitted (needed for the SC-001 done-signal)")
	}
	if !sawMax {
		t.Error("INT_MAX never emitted")
	}
}

func TestGenStringValuesAreSafeToPrint(t *testing.T) {
	r := NewRNG(7)
	for i := 0; i < 500; i++ {
		var cats CategorySet
		got := escapeWispString(genStringValue(r, &cats).(*StringLit).V)
		if !strings.HasPrefix(got, "\"") || !strings.HasSuffix(got, "\"") {
			t.Fatalf("escaped literal not quoted: %q", got)
		}
	}
}
