package pkgtool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractEntriesGuards(t *testing.T) {
	// escapeTarget is an ABSOLUTE path constructed under a process-private temp dir (NOT
	// a real host path like /etc/passwd, which would flake if it happened to exist). It is
	// the absolute / symlink-escape target the malicious entries try to write to; after
	// every rejected case we assert NOTHING was written there.
	escapeRoot := t.TempDir()
	absTarget := filepath.Join(escapeRoot, "absolute_target")     // for the "absolute" case
	symlinkDir := filepath.Join(escapeRoot, "symlink_target_dir") // for the symlink cases
	if err := os.MkdirAll(symlinkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		entries []entry
		wantErr bool
	}{
		{"ok-nested", []entry{{rel: "a/b.wisp", mode: modeRegular, content: []byte("x")}}, false},
		{"absolute", []entry{{rel: absTarget, mode: modeRegular, content: []byte("x")}}, true},
		{"dotdot-escape", []entry{{rel: "../evil", mode: modeRegular, content: []byte("x")}}, true},
		{"deep-dotdot", []entry{{rel: "a/../../evil", mode: modeRegular, content: []byte("x")}}, true},
		{"symlink", []entry{{rel: "a", mode: modeSymlink, content: []byte(symlinkDir)}}, true},
		{"write-through-symlink", []entry{
			{rel: "a", mode: modeSymlink, content: []byte(symlinkDir)},
			{rel: "a/escaped", mode: modeRegular, content: []byte("x")},
		}, true},
		{"special-fifo", []entry{{rel: "p", mode: modeSpecial, content: nil}}, true},
		{"dot-git", []entry{{rel: ".git/config", mode: modeRegular, content: []byte("x")}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dest := t.TempDir()
			err := extractEntries(dest, c.entries)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			// Nothing may ever be written outside dest: assert the parent of dest is
			// untouched by checking no sibling appeared, and that the absolute/symlink
			// escape target under the process-private escapeRoot received no write.
			if c.wantErr {
				if _, statErr := os.Stat(filepath.Join(filepath.Dir(dest), "evil")); statErr == nil {
					t.Fatal("an entry escaped the destination")
				}
				if _, statErr := os.Stat(absTarget); statErr == nil {
					t.Fatal("wrote to the absolute escape target outside dest")
				}
				if _, statErr := os.Stat(filepath.Join(symlinkDir, "escaped")); statErr == nil {
					t.Fatal("wrote through a symlink outside dest")
				}
			}
		})
	}
}
