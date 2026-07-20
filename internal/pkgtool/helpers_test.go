package pkgtool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hex40(c string) string { return strings.Repeat(c, 40) }

// writeLock marshals lf and atomically writes it to path. Test-only: real
// install paths write via writeFileAtomic(lockPath, lockBytes) after marshalLock.
func writeLock(path string, lf *lockfile) error {
	b, err := marshalLock(lf)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}
func hex64(c string) string { return strings.Repeat(c, 64) }

func contains(s, substr string) bool { return strings.Contains(s, substr) }
