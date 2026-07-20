package main

import (
	"bytes"
	"testing"
)

// TestExamplesCheckClean rides the existing go test ./... CI gate to keep
// examples/ fmt-clean (dogfooding the directory mode added by this change).
func TestExamplesCheckClean(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"fmt", "--check", "../../examples"}, &so, &se); code != 0 {
		t.Fatalf("examples not fmt-clean: exit=%d stdout=%q stderr=%q", code, so.String(), se.String())
	}
}
