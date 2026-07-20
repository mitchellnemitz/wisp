// Package testrunner discovers *_test.wisp files, compiles each to its TAP-13
// runner via driver.Compile, runs the runner under every available target shell,
// parses the TAP, aggregates results, and returns a summary.
package testrunner

import (
	"os/exec"
)

// Shell describes one target shell the runner can use.
type Shell struct {
	Label string   // human label (e.g. "dash", "busybox-sh")
	Bin   string   // path to the binary
	Args  []string // args prepended before the script path
}

// AvailableShells returns the shells available on this machine. Shell order:
// dash, busybox-sh, bash, zsh. An absent shell is silently omitted (never an
// error). This mirrors the discovery in internal/golden/golden_test.go.
func AvailableShells() []Shell {
	var out []Shell

	if bin, err := exec.LookPath("dash"); err == nil {
		out = append(out, Shell{Label: "dash", Bin: bin})
	}
	if bin, err := exec.LookPath("busybox"); err == nil {
		out = append(out, Shell{Label: "busybox-sh", Bin: bin, Args: []string{"sh"}})
	}
	if bin, err := exec.LookPath("bash"); err == nil {
		out = append(out, Shell{Label: "bash", Bin: bin})
	}
	if bin, err := exec.LookPath("zsh"); err == nil {
		// -f (NO_RCS) disables all startup files so the test run is hermetic on a
		// developer machine whose ~/.zshenv might alter PATH or options.
		out = append(out, Shell{Label: "zsh", Bin: bin, Args: []string{"-f"}})
	}

	return out
}
