//go:build fuzzshell

// internal/fuzz/replay_live_test.go
package fuzz

import "testing"

// TestCuratedReplayPasses is the PASS half of SC-008: every committed curated
// regression entry (corpus/regressions/, including intmin-via-var.json) replays
// clean on the current, correct compiler. The FAIL half (a reverted compiler makes
// the same entry's `wisp-fuzz replay` report a regression) is the README done-signal
// -- it cannot be asserted here because a genuine cross-shell divergence cannot exist
// on a correct compiler.
func TestCuratedReplayPasses(t *testing.T) {
	shells, err := RequirePrereqs()
	if err != nil {
		t.Skipf("prereqs unavailable: %v", err)
	}
	entries, err := LoadCorpus("corpus/regressions")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no curated regression entries committed (expected at least intmin-via-var.json)")
	}
	for _, f := range entries {
		res, err := ReplayEntry(shells, f)
		if err != nil {
			t.Fatalf("replay error (%s): %v", f.Provenance.Filename(), err)
		}
		if res.Diverged || res.Unstable {
			t.Fatalf("curated entry %s unexpectedly failed replay on the correct compiler: %s", f.Provenance.Filename(), res.Detail)
		}
	}
}
