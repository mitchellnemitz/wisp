package module

import (
	"os"
	"path/filepath"
)

// OSFS is the production FS backed by the real filesystem.
type OSFS struct{}

func (OSFS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (OSFS) Stat(path string) (exists, isDir bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, false
	}
	return true, fi.IsDir()
}

func (OSFS) EvalSymlinks(path string) (string, error) { return filepath.EvalSymlinks(path) }

func (OSFS) Abs(path string) (string, error) { return filepath.Abs(path) }
