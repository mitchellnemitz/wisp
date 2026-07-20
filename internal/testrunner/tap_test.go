package testrunner

import "testing"

// TestParseTAPEscapedHashIsNotDirective: a description carrying an ESCAPED `#`
// (`\#`, as the codegen emits for a `#` inside a test name) is not a SKIP
// directive. The parser must split the directive only at the first UNESCAPED
// `#`, and unescape the description (`\#` -> `#`, `\\` -> `\`) before storing.
func TestParseTAPEscapedHashIsNotDirective(t *testing.T) {
	// codegen emitted `has # SKIP in name` as `has \# SKIP in name`.
	out := "TAP version 13\n1..1\nok 1 - has \\# SKIP in name\n"
	suite := ParseTAP(out)
	if len(suite.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(suite.Results))
	}
	r := suite.Results[0]
	if r.Skip {
		t.Errorf("escaped `#` was wrongly treated as a SKIP directive")
	}
	if r.Name != "has # SKIP in name" {
		t.Errorf("name did not round-trip: got %q want %q", r.Name, "has # SKIP in name")
	}
}

// TestParseTAPEscapedHashFailNotSkip: the same name on a `not ok` line is a
// FAILURE, not a skip, and its diagnostic is preserved.
func TestParseTAPEscapedHashFailNotSkip(t *testing.T) {
	out := "TAP version 13\n1..1\nnot ok 1 - has \\# SKIP in name\n# assertion failed\n"
	suite := ParseTAP(out)
	if len(suite.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(suite.Results))
	}
	r := suite.Results[0]
	if r.Skip {
		t.Errorf("escaped `#` on a not-ok line was wrongly treated as a skip")
	}
	if r.OK {
		t.Errorf("expected a failure result")
	}
	if r.Name != "has # SKIP in name" {
		t.Errorf("name did not round-trip: got %q", r.Name)
	}
	if r.Diag == "" {
		t.Errorf("diagnostic lost")
	}
}

// TestParseTAPRealSkipStillRecognized: an UNESCAPED ` # SKIP ` directive (as the
// runner emits for a genuine skip) is still recognized.
func TestParseTAPRealSkipStillRecognized(t *testing.T) {
	out := "TAP version 13\n1..1\nok 1 - c skips # SKIP not ready\n"
	suite := ParseTAP(out)
	if len(suite.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(suite.Results))
	}
	r := suite.Results[0]
	if !r.Skip {
		t.Errorf("genuine SKIP directive not recognized")
	}
	if r.Name != "c skips" {
		t.Errorf("name = %q, want %q", r.Name, "c skips")
	}
	if r.SkipReason != "not ready" {
		t.Errorf("skip reason = %q, want %q", r.SkipReason, "not ready")
	}
}

// TestParseTAPEscapedBackslashRoundTrips: a literal backslash in a name is
// emitted as `\\` and must round-trip to a single backslash.
func TestParseTAPEscapedBackslashRoundTrips(t *testing.T) {
	// name `a\b` -> escaped `a\\b`.
	out := "TAP version 13\n1..1\nok 1 - a\\\\b\n"
	suite := ParseTAP(out)
	if len(suite.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(suite.Results))
	}
	if got := suite.Results[0].Name; got != "a\\b" {
		t.Errorf("name = %q, want %q", got, "a\\b")
	}
}
