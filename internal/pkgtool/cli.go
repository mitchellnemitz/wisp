package pkgtool

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AddArgs holds the parsed arguments for the `wisp add` subcommand.
type AddArgs struct {
	Repo   string // owner/repo key
	Git    string // git URL (empty if not provided)
	Ref    string // ref name (empty if not provided)
	HasGit bool
	HasRef bool
}

// Add implements `wisp add`. It:
//  1. Resolves the effective git URL and ref (querying DefaultRef when needed).
//  2. Writes the updated wisp.json to disk.
//  3. Calls installAt.
//  4. On any error, restores wisp.json (and wisp.lock if absent before) to
//     their pre-command bytes so the project is byte-identical.
func Add(root string, args AddArgs, f Fetcher, stderr io.Writer) int {
	manifestPath := filepath.Join(root, "wisp.json")
	lockPath := filepath.Join(root, "wisp.lock")

	man, err := readManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "wisp: add: %v\n", err)
		return 1
	}

	existing, isRepin := man.Dependencies[args.Repo]

	// Resolve effective git URL.
	gitURL := args.Git
	if !args.HasGit {
		if !isRepin {
			// First add requires --git.
			fmt.Fprintf(stderr, "wisp: add %s: first add requires --git <url>\n%s", args.Repo, addUsage)
			return 2
		}
		gitURL = existing.Git
	}

	// Resolve effective ref.
	ref := args.Ref
	if !args.HasRef {
		if isRepin {
			// Re-pin with no --ref: reuse the existing recorded ref.
			ref = existing.Ref
		} else {
			// First add with no --ref: discover the remote's default branch name.
			defaultBranch, err := f.DefaultRef(gitURL)
			if err != nil {
				fmt.Fprintf(stderr, "wisp: add %s (%s): discover default branch: %v\n", args.Repo, gitURL, err)
				return 1
			}
			ref = defaultBranch
		}
	}

	// Capture pre-command state for rollback.
	manifestBefore, _ := os.ReadFile(manifestPath)
	lockBefore, lockExisted := readOptionalFile(lockPath)

	// Update the in-memory manifest and write it to disk so installAt reads the new state.
	if man.Dependencies == nil {
		man.Dependencies = map[string]dependency{}
	}
	man.Dependencies[args.Repo] = dependency{Git: gitURL, Ref: ref}
	if err := writeManifest(manifestPath, man); err != nil {
		fmt.Fprintf(stderr, "wisp: add %s (%s): write wisp.json: %v\n", args.Repo, gitURL, err)
		return 1
	}

	if err := installAt(root, f, stderr); err != nil {
		fmt.Fprintf(stderr, "wisp: add %s (%s): %v\n", args.Repo, gitURL, err)
		if rerr := restoreFiles(manifestPath, manifestBefore, lockPath, lockBefore, lockExisted); rerr != nil {
			fmt.Fprintf(stderr, "wisp: WARNING rollback incomplete, project may not be restored: %v\n", rerr)
		}
		return 1
	}
	return 0
}

// Install implements `wisp install`.
func Install(root string, f Fetcher, stderr io.Writer) int {
	if err := installAt(root, f, stderr); err != nil {
		fmt.Fprintf(stderr, "wisp: install: %v\n", err)
		return 1
	}
	return 0
}

// Remove implements `wisp remove`. It drops the key from wisp.json, then
// calls installAt (which re-resolves and prunes the removed module's tree).
func Remove(root, key string, f Fetcher, stderr io.Writer) int {
	manifestPath := filepath.Join(root, "wisp.json")
	lockPath := filepath.Join(root, "wisp.lock")

	man, err := readManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "wisp: remove: %v\n", err)
		return 1
	}
	if _, ok := man.Dependencies[key]; !ok {
		fmt.Fprintf(stderr, "wisp: remove %s: not a direct dependency\n", key)
		return 1
	}

	manifestBefore, _ := os.ReadFile(manifestPath)
	lockBefore, lockExisted := readOptionalFile(lockPath)

	delete(man.Dependencies, key)
	if err := writeManifest(manifestPath, man); err != nil {
		fmt.Fprintf(stderr, "wisp: remove %s: write wisp.json: %v\n", key, err)
		return 1
	}

	if err := installAt(root, f, stderr); err != nil {
		fmt.Fprintf(stderr, "wisp: remove %s: %v\n", key, err)
		if rerr := restoreFiles(manifestPath, manifestBefore, lockPath, lockBefore, lockExisted); rerr != nil {
			fmt.Fprintf(stderr, "wisp: WARNING rollback incomplete, project may not be restored: %v\n", rerr)
		}
		return 1
	}
	return 0
}

const addUsage = "usage: wisp add <owner/repo> --git <url> [--ref <ref>]\n"

func readOptionalFile(path string) ([]byte, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return b, true
}

// restoreFiles rolls wisp.json and wisp.lock back to their pre-command bytes.
// It returns a non-nil error if any restore step fails, so the caller can warn
// that the project may NOT be byte-identical despite the atomicity contract.
func restoreFiles(manifestPath string, manifestBefore []byte, lockPath string, lockBefore []byte, lockExisted bool) error {
	var errs []string
	if manifestBefore != nil {
		if err := writeFileAtomic(manifestPath, manifestBefore); err != nil {
			errs = append(errs, fmt.Sprintf("restore wisp.json: %v", err))
		}
	}
	if lockExisted {
		if err := writeFileAtomic(lockPath, lockBefore); err != nil {
			errs = append(errs, fmt.Sprintf("restore wisp.lock: %v", err))
		}
	} else if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Sprintf("remove wisp.lock: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
