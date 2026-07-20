package pkgtool

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// lockEntry is one entry in wisp.lock.
type lockEntry struct {
	Git        string   `json:"git"`
	Ref        string   `json:"ref"`
	Commit     string   `json:"commit"`
	TreeDigest string   `json:"treeDigest"`
	Deps       []string `json:"deps"`
}

// lockVersion is the current wisp.lock schema version.
const lockVersion = 1

// lockfile is the in-memory representation of wisp.lock.
type lockfile struct {
	Version int                  `json:"version"`
	Wisp    string               `json:"wisp,omitempty"`
	Modules map[string]lockEntry `json:"modules"`
}

func marshalLock(lf *lockfile) ([]byte, error) {
	// Sort module keys so the output is deterministic.
	keys := make([]string, 0, len(lf.Modules))
	for k := range lf.Modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Determinism comes from two things, both below: (1) encoding/json sorts a
	// string-keyed map's keys when marshalling (a documented guarantee), and
	// (2) each entry's Deps slice is sorted before encoding. So a plain
	// map[string]lockEntry marshalled with MarshalIndent is byte-stable. The
	// nil-Deps -> [] normalisation (so the JSON renders [] not null) is done per
	// entry in the loop below.
	wm := make(map[string]lockEntry, len(lf.Modules))
	for _, k := range keys {
		e := lf.Modules[k]
		deps := e.Deps
		if deps == nil {
			deps = []string{}
		} else {
			d2 := make([]string, len(deps))
			copy(d2, deps)
			sort.Strings(d2)
			deps = d2
		}
		wm[k] = lockEntry{
			Git:        e.Git,
			Ref:        e.Ref,
			Commit:     e.Commit,
			TreeDigest: e.TreeDigest,
			Deps:       deps,
		}
	}

	// encoding/json sorts string map keys when marshalling (documented in
	// json.Marshal), so the modules object is emitted in a stable key order.
	b, err := json.MarshalIndent(lockfile{Version: lf.Version, Wisp: lf.Wisp, Modules: wm}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func readLock(path string) (*lockfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lf lockfile
	if err := json.Unmarshal(b, &lf); err != nil {
		return nil, fmt.Errorf("malformed wisp.lock: %w", err)
	}
	if lf.Version != lockVersion {
		return nil, fmt.Errorf("wisp.lock version %d is not supported (this wisp expects version %d)", lf.Version, lockVersion)
	}
	for key, e := range lf.Modules {
		if _, _, err := module.SplitRepo(key); err != nil {
			return nil, fmt.Errorf("lock: invalid module key %q: %v", key, err)
		}
		if err := validateGitURL(e.Git); err != nil {
			return nil, fmt.Errorf("lock: module %q: %v", key, err)
		}
		if err := validateRef(e.Ref); err != nil {
			return nil, fmt.Errorf("lock: module %q: %v", key, err)
		}
		if err := validateCommit(e.Commit); err != nil {
			return nil, fmt.Errorf("lock: module %q: %v", key, err)
		}
		if err := validateTreeDigest(e.TreeDigest); err != nil {
			return nil, fmt.Errorf("lock: module %q: %v", key, err)
		}
		// Check that every dep edge points to a known module.
		for _, dep := range e.Deps {
			if _, ok := lf.Modules[dep]; !ok {
				return nil, fmt.Errorf("lock: module %q has dangling dep edge %q", key, dep)
			}
		}
	}
	return &lf, nil
}

// treeDigest hashes the regular files under dir in a canonical, mode-independent
// way. The hash input is a length-framed stream sorted by slash path:
//
//	for each file in sorted slash-path order:
//	    8-byte big-endian uint64 of len(slashPath)
//	    slashPath bytes
//	    8-byte big-endian uint64 of file size in bytes
//	    file content bytes
//
// Returns "sha256:" + 64 lowercase hex.
func treeDigest(dir string) (string, error) {
	type entry struct {
		slashPath string
		absPath   string
	}
	var files []entry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Extraction forbids symlinks and special files, so the only legitimate
		// non-dir entry under a module tree is a regular file. A symlink, device,
		// fifo, or socket here means the tree was tampered with after install:
		// hard-error rather than skip, otherwise a tampered tree could still match
		// the locked digest (the foreign entry would be silently excluded).
		if !d.Type().IsRegular() {
			return fmt.Errorf("non-regular file %q (%s) in module tree: refusing to digest a tampered tree", path, d.Type())
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, entry{
			slashPath: filepath.ToSlash(rel),
			absPath:   path,
		})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].slashPath < files[j].slashPath
	})

	h := sha256.New()
	var lenBuf [8]byte
	for _, f := range files {
		// frame: slash path
		pathBytes := []byte(f.slashPath)
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(pathBytes)))
		h.Write(lenBuf[:])
		h.Write(pathBytes)

		// frame: content
		data, err := os.ReadFile(f.absPath)
		if err != nil {
			return "", err
		}
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(data)))
		h.Write(lenBuf[:])
		h.Write(data)
	}
	return "sha256:" + fmt.Sprintf("%x", h.Sum(nil)), nil
}
