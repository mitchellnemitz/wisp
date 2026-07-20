package testrunner

import (
	"reflect"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

// run builds a shellRun with the given label and TAP results.
func run(label string, results []TAPResult) shellRun {
	return shellRun{
		shell: Shell{Label: label},
		suite: TAPSuite{Results: results},
	}
}

// TestComputeFileStatsMix exercises the single-pass aggregator on a mix of
// pass/fail/skip plus a cross-shell divergence, and checks per-run counts,
// failure capture, divergence, per-file/overall failure flags, and grand totals.
func TestComputeFileStatsMix(t *testing.T) {
	results := []fileResult{{
		path: "/tmp/mix_test.wisp",
		runs: []shellRun{
			run("sh-a", []TAPResult{
				{Name: "alpha_pass", OK: true},
				{Name: "beta_fail", OK: false, Diag: "# boom"},
				{Name: "gamma_skip", OK: true, Skip: true},
				{Name: "delta_flaky", OK: true},
			}),
			run("sh-b", []TAPResult{
				{Name: "alpha_pass", OK: true},
				{Name: "beta_fail", OK: false, Diag: "# boom"},
				{Name: "gamma_skip", OK: true, Skip: true},
				{Name: "delta_flaky", OK: false},
			}),
		},
	}}

	got := computeFileStats(results)

	if got.pass != 3 || got.fail != 3 || got.skip != 2 {
		t.Errorf("totals = %d/%d/%d, want 3/3/2", got.pass, got.fail, got.skip)
	}
	if got.overallOK {
		t.Error("overallOK = true, want false (there are failures)")
	}
	if len(got.files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(got.files))
	}
	fs := got.files[0]
	if fs.shortPath != "mix_test.wisp" {
		t.Errorf("shortPath = %q, want mix_test.wisp", fs.shortPath)
	}
	if !fs.failed || fs.compileErr {
		t.Errorf("fs.failed=%v compileErr=%v, want true/false", fs.failed, fs.compileErr)
	}

	wantRuns := []runStats{
		{label: "sh-a", pass: 2, fail: 1, skip: 1, failures: []runFailure{{name: "beta_fail", diag: "# boom"}}},
		{label: "sh-b", pass: 1, fail: 2, skip: 1, failures: []runFailure{{name: "beta_fail", diag: "# boom"}, {name: "delta_flaky", diag: ""}}},
	}
	if !reflect.DeepEqual(fs.runs, wantRuns) {
		t.Errorf("runs mismatch\n got: %+v\nwant: %+v", fs.runs, wantRuns)
	}

	wantDiv := []divergence{{name: "delta_flaky", failShells: []string{"sh-b"}}}
	if !reflect.DeepEqual(fs.divergences, wantDiv) {
		t.Errorf("divergences mismatch\n got: %+v\nwant: %+v", fs.divergences, wantDiv)
	}
}

// TestComputeFileStatsCompileError verifies a compile-error file is counted as a
// single failure in the totals, marked compileErr, and forces overallOK=false,
// contributing no per-run stats.
func TestComputeFileStatsCompileError(t *testing.T) {
	results := []fileResult{{
		path:  "/tmp/bad_test.wisp",
		diags: []driver.Diagnostic{{Msg: "boom", Severity: driver.Error}},
	}}

	got := computeFileStats(results)

	if got.pass != 0 || got.fail != 1 || got.skip != 0 {
		t.Errorf("totals = %d/%d/%d, want 0/1/0", got.pass, got.fail, got.skip)
	}
	if got.overallOK {
		t.Error("overallOK = true, want false")
	}
	if len(got.files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(got.files))
	}
	fs := got.files[0]
	if !fs.compileErr || !fs.failed {
		t.Errorf("compileErr=%v failed=%v, want true/true", fs.compileErr, fs.failed)
	}
	if len(fs.runs) != 0 {
		t.Errorf("compile-error file should have no runs, got %d", len(fs.runs))
	}
}

// TestComputeFileStatsExitMismatchAndTAPError verifies a run with no failing
// tests but an exit-code mismatch or TAP error is still marked failed, with the
// error state carried onto the runStats for printing.
func TestComputeFileStatsExitMismatchAndTAPError(t *testing.T) {
	results := []fileResult{{
		path: "/tmp/err_test.wisp",
		runs: []shellRun{
			{shell: Shell{Label: "sh-x"}, suite: TAPSuite{Results: []TAPResult{{Name: "t", OK: true}}}, exitMismatch: true},
			{shell: Shell{Label: "sh-y"}, suite: TAPSuite{Results: []TAPResult{{Name: "t", OK: true}}}, tapError: "incomplete TAP: expected 2 results, got 1"},
		},
	}}

	got := computeFileStats(results)

	if got.pass != 2 || got.fail != 0 || got.skip != 0 {
		t.Errorf("totals = %d/%d/%d, want 2/0/0", got.pass, got.fail, got.skip)
	}
	if got.overallOK {
		t.Error("overallOK = true, want false (exit mismatch + tap error)")
	}
	fs := got.files[0]
	if !fs.failed {
		t.Error("fs.failed = false, want true")
	}
	if !fs.runs[0].failed() || !fs.runs[1].failed() {
		t.Errorf("both runs should report failed(): %v %v", fs.runs[0].failed(), fs.runs[1].failed())
	}
}

// TestComputeFileStatsAllPass verifies a clean all-pass run reports overallOK.
func TestComputeFileStatsAllPass(t *testing.T) {
	results := []fileResult{{
		path: "/tmp/ok_test.wisp",
		runs: []shellRun{run("sh", []TAPResult{{Name: "a", OK: true}, {Name: "b", OK: true}})},
	}}

	got := computeFileStats(results)

	if !got.overallOK {
		t.Error("overallOK = false, want true")
	}
	if got.pass != 2 || got.fail != 0 || got.skip != 0 {
		t.Errorf("totals = %d/%d/%d, want 2/0/0", got.pass, got.fail, got.skip)
	}
	if got.files[0].failed {
		t.Error("fs.failed = true, want false")
	}
}
