package pkgtool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/module"
)

type entryMode int

const (
	modeRegular entryMode = iota
	modeSymlink
	modeSpecial
)

type entry struct {
	rel     string
	mode    entryMode
	content []byte
}

// extractEntries writes the given tree entries under dest, treating every entry as
// untrusted (spec section 7). It rejects: absolute paths, any cleaned relative path
// that escapes dest (.. traversal), ANY symlink (so no write-through-symlink is
// possible), any special file (device/fifo/socket), and any .git/ path. On any
// rejection it returns an error and writes nothing further; it never writes outside
// dest. Only regular files and their parent dirs are materialized.
func extractEntries(dest string, entries []entry) error {
	// Ensure the destination root exists even for a degenerate (empty) tree, so the
	// module dir is always present after a successful extract.
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.mode == modeSymlink {
			return fmt.Errorf("refusing symlink entry %q (symlinks are out of contract)", e.rel)
		}
		if e.mode == modeSpecial {
			return fmt.Errorf("refusing special file entry %q (device/fifo/socket)", e.rel)
		}
		if filepath.IsAbs(e.rel) {
			return fmt.Errorf("refusing absolute path entry %q", e.rel)
		}
		clean := filepath.Clean(e.rel)
		// .git is never copied (any segment named .git).
		for _, seg := range strings.Split(filepath.ToSlash(clean), "/") {
			if seg == ".git" {
				return fmt.Errorf("refusing .git path %q", e.rel)
			}
		}
		full := filepath.Join(dest, clean)
		if !module.Within(full, dest) {
			return fmt.Errorf("entry %q escapes the destination", e.rel)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, e.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
