package pkgtool

import (
	"os"
	"path/filepath"
)

func writeFileAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".wisp-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	// os.CreateTemp makes the file 0600; these are project files (wisp.json,
	// wisp.lock, .wisp/.gitignore) that must be world-readable like a normal commit.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName) // do not leave the temp behind on a failed rename
		return err
	}
	return nil
}
