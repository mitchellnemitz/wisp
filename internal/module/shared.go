package module

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DiscoverRoot walks upward from startDir (a cleaned absolute dir) for the nearest
// ancestor containing a wisp.json file; if none, returns startDir. It is the single
// implementation shared by the loader and the package tool (internal/pkgtool).
func DiscoverRoot(fs FS, startDir string) string {
	dir := startDir
	for {
		if exists, isDir := fs.Stat(filepath.Join(dir, "wisp.json")); exists && !isDir {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir
		}
		dir = parent
	}
}

// Within reports whether path is dir or a descendant of dir (string comparison on
// cleaned absolute paths). Shared by the loader's include containment check and the
// package tool's extraction guard.
func Within(path, dir string) bool {
	return within(path, dir)
}

// SplitRepo validates and splits an owner/repo string. The error (when non-nil)
// describes the rejection; the loader wraps it into a positioned diagnostic.
func SplitRepo(s string) (owner, repo string, err error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("path %q must be exactly owner/repo (two segments)", s)
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || !repoSegmentRe.MatchString(p) {
			return "", "", fmt.Errorf("path %q has an invalid segment %q (allowed: letters, digits, . _ -; not . or ..)", s, p)
		}
	}
	return parts[0], parts[1], nil
}
