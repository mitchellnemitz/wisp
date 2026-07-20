package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fs.symlink_force is a removable builtin (bare symlink_force no longer
// resolves in the single-module check), so the three runtime tests below
// compile through compileNS with the fs namespace bound.

// TestSymlinkForce_ReplacesDirLink (AC3): a symlink that points at a directory
// is repointed to the new target; the directory and its contents are untouched.
// This guards against the temp+mv / ln -sf botch where BSD mv follows a
// symlink-to-dir destination and creates the link inside the directory instead
// of replacing the link itself.
func TestSymlinkForce_ReplacesDirLink(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "rpl.sh")
	src := `fn main() -> int {
  fs.symlink_force("newtarget", "link")
  return 0
}`
	if err := os.WriteFile(script, compileNS(t, src, "fs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			run := t.TempDir()
			if err := os.Mkdir(filepath.Join(run, "d"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(run, "d", "marker"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("d", filepath.Join(run, "link")); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = run
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: script run: %v", sh.label, err)
			}
			got, err := os.Readlink(filepath.Join(run, "link"))
			if err != nil {
				t.Fatalf("%s: link is not a symlink: %v", sh.label, err)
			}
			if got != "newtarget" {
				t.Errorf("%s: link -> %q, want newtarget (not repointed -- botch?)", sh.label, got)
			}
			ents, err := os.ReadDir(filepath.Join(run, "d"))
			if err != nil {
				t.Fatalf("%s: ReadDir d: %v", sh.label, err)
			}
			var entNames []string
			for _, e := range ents {
				entNames = append(entNames, e.Name())
			}
			if len(ents) != 1 || ents[0].Name() != "marker" {
				t.Errorf("%s: d/ contents = %v, want [marker] (stray link created inside -- botch)", sh.label, entNames)
			}
		})
	}
}

// TestSymlinkForce_RefusesRealDir (AC5): a real directory at link_path causes a
// located abort; the directory and its contents are untouched.
func TestSymlinkForce_RefusesRealDir(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "rrd.sh")
	src := `fn main() -> int {
  fs.symlink_force("t", "d")
  return 0
}`
	if err := os.WriteFile(script, compileNS(t, src, "fs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			run := t.TempDir()
			if err := os.Mkdir(filepath.Join(run, "d"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(run, "d", "marker"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = run
			var stderr strings.Builder
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err == nil {
				t.Errorf("%s: should abort on a real dir", sh.label)
			}
			if !strings.Contains(stderr.String(), "symlink_force") {
				t.Errorf("%s: stderr = %q, want located symlink_force", sh.label, stderr.String())
			}
			ents, readErr := os.ReadDir(filepath.Join(run, "d"))
			if readErr != nil {
				t.Fatalf("%s: ReadDir d: %v", sh.label, readErr)
			}
			var entNames []string
			for _, e := range ents {
				entNames = append(entNames, e.Name())
			}
			if len(ents) != 1 || ents[0].Name() != "marker" {
				t.Errorf("%s: d/ contents changed = %v", sh.label, entNames)
			}
			fi, statErr := os.Lstat(filepath.Join(run, "d"))
			if statErr != nil || !fi.IsDir() {
				t.Errorf("%s: d is no longer a real directory", sh.label)
			}
		})
	}
}

// TestSymlinkForce_InjectionInert (AC6): hostile target and hostile link_path
// strings are inert -- no PWNED/PWNED2 file is created, and the literal hostile
// link_path "$(touch PWNED2)x" is created as a real symlink pointing to "ok".
func TestSymlinkForce_InjectionInert(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "inj.sh")
	src := "fn main() -> int {\n" +
		"  fs.symlink_force(\"$(touch PWNED); `id`; *\", \"link\")\n" +
		"  fs.symlink_force(\"ok\", \"$(touch PWNED2)x\")\n" +
		"  return 0\n}"
	if err := os.WriteFile(script, compileNS(t, src, "fs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			run := t.TempDir()
			if err := os.WriteFile(filepath.Join(run, "decoy"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = run
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: script run: %v", sh.label, err)
			}
			for _, bad := range []string{"PWNED", "PWNED2"} {
				if _, err := os.Stat(filepath.Join(run, bad)); !os.IsNotExist(err) {
					t.Errorf("%s: %s present (or stat err) -- substitution executed", sh.label, bad)
				}
			}
			// AC6 positive assertion: a link was created at the LITERAL hostile
			// link_path "$(touch PWNED2)x" and points to "ok".
			tgt, err := os.Readlink(filepath.Join(run, "$(touch PWNED2)x"))
			if err != nil {
				t.Errorf("%s: no link at the literal hostile link_path: %v", sh.label, err)
			} else if tgt != "ok" {
				t.Errorf("%s: hostile-link_path link -> %q, want ok", sh.label, tgt)
			}
		})
	}
}
