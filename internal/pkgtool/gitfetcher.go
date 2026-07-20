package pkgtool

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// fetchKind labels the steps of the Fetch fallback ladder for the fetchTrace seam.
type fetchKind int

const (
	fetchSHA     fetchKind = iota // initial bare-SHA attempt
	fetchRefTip                   // symbolic ref-tip fetch (shallow or unshallowed)
	fetchAllRefs                  // SHA-valued-ref +refs/*:refs/* escalation
)

// fetchStep records one ladder step and its outcome.
type fetchStep struct {
	kind fetchKind
	err  error
}

// fetchTrace is a package-level hook for testing. When non-nil, Fetch calls it
// after each ladder step. Nil in production.
var fetchTrace func(fetchStep)

// gitFetcher is the production Fetcher that shells out to git.
type gitFetcher struct{}

// GitFetcher returns the production git-backed Fetcher.
func GitFetcher() Fetcher {
	return gitFetcher{}
}

// gitArgs is a prefix applied to every git invocation to neutralize config
// that could alter blob content or encoding (autocrlf, line-ending conversion,
// symlinks) and to suppress fsmonitor overhead.
var gitArgs = []string{
	"-c", "core.autocrlf=false",
	"-c", "core.eol=lf",
	"-c", "core.symlinks=false",
	"-c", "core.fsmonitor=false",
}

// git runs a git command, returning combined output. dir is the working directory
// (empty = inherit). args are the git sub-command and its positional/flag arguments;
// the global gitArgs prefix is prepended.
func git(dir string, args ...string) ([]byte, error) {
	full := append(append([]string{}, gitArgs...), args...)
	cmd := exec.Command("git", full...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

// DefaultRef returns the remote's symbolic HEAD branch name (e.g. "main").
// It runs `git ls-remote --symref -- <url> HEAD` and parses the
// `ref: refs/heads/<name>\tHEAD` line.
func (gitFetcher) DefaultRef(url string) (string, error) {
	if err := validateGitURL(url); err != nil {
		return "", err
	}
	out, err := git("", "ls-remote", "--symref", "--", url, "HEAD")
	if err != nil {
		return "", fmt.Errorf("ls-remote --symref %s: %v: %s", url, err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Format: "ref: refs/heads/<name>\tHEAD"
		if !strings.HasPrefix(line, "ref: refs/heads/") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) != "HEAD" {
			continue
		}
		name := strings.TrimPrefix(parts[0], "ref: refs/heads/")
		name = strings.TrimSpace(name)
		if name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("cannot determine default branch for %s: no symref line in ls-remote output", url)
}

// Resolve maps a ref name to its commit SHA. It fetches all refs from the
// remote (git ls-remote -- <url>) and matches client-side, then collapses
// annotated-tag peel entries. Errors on ambiguity (>1 distinct base refname
// matching) or no match.
//
// Client-side matching is necessary because `git ls-remote -- url <pattern>`
// with a short name like "v1" does NOT return the `^{}` peel line, so asking
// the server to filter causes annotated tags to resolve to the tag-object SHA
// instead of the peeled commit SHA.
func (gitFetcher) Resolve(url, ref string) (string, error) {
	if err := validateGitURL(url); err != nil {
		return "", err
	}
	if err := validateRef(ref); err != nil {
		return "", err
	}
	out, err := git("", "ls-remote", "--", url)
	if err != nil {
		return "", fmt.Errorf("ls-remote %s: %v: %s", url, err, out)
	}
	// Parse all output and match refs whose last path component equals ref,
	// or whose full refname equals ref, or whose base (^{}-stripped) equals ref.
	type entry struct {
		sha    string
		peeled bool // true for the ^{} line
	}
	groups := map[string]entry{} // base refname -> best entry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		sha, refname := parts[0], parts[1]
		// Skip HEAD itself (we only want named refs).
		if refname == "HEAD" {
			continue
		}
		peeled := strings.HasSuffix(refname, "^{}")
		base := strings.TrimSuffix(refname, "^{}")
		// A refname matches if the last component of base equals ref, or base == ref.
		last := base
		if i := strings.LastIndex(base, "/"); i >= 0 {
			last = base[i+1:]
		}
		if last != ref && base != ref {
			continue
		}
		existing, had := groups[base]
		if !had {
			groups[base] = entry{sha, peeled}
		} else if peeled && !existing.peeled {
			// Prefer the peeled commit SHA over the tag-object SHA.
			groups[base] = entry{sha, true}
		}
	}

	if len(groups) == 0 {
		return "", fmt.Errorf("ref %q not found at %s", ref, url)
	}
	if len(groups) > 1 {
		// More than one distinct base refname matched: true ambiguity.
		var names []string
		for n := range groups {
			names = append(names, n)
		}
		return "", fmt.Errorf("ambiguous ref %q at %s: matches %v", ref, url, names)
	}
	for _, e := range groups {
		return e.sha, nil
	}
	return "", fmt.Errorf("internal: empty groups map for %s@%s", url, ref)
}

// Fetch materialises the tree at the pinned commit into destDir. It uses a
// temporary git repo for the fetch, then extracts the tree using ls-tree + cat-file
// (bypassing git's working-tree machinery so blob bytes are delivered verbatim,
// independent of core.autocrlf, eol conversion, or smudge filters).
//
// Fallback ladder:
//  1. Attempt a bare-SHA fetch (`git fetch --depth 1 -- <url> <sha>`).
//  2. On failure (or when ref is itself a full SHA -- SHA-valued-ref case), escalate:
//     - Non-SHA ref: fetch the ref tip (`git fetch --depth 1 -- <url> <ref>`);
//     if the pinned sha is still absent, unshallow (`git fetch --unshallow`).
//     - SHA-valued ref: mirror all refs (`git fetch -- <url> +refs/*:refs/*`).
//  3. After a successful fetch, run `git ls-tree -r -z <sha>` and extract each entry
//     via `git cat-file blob <oid>`.
func (gitFetcher) Fetch(url, ref, sha, destDir string) error {
	if err := validateGitURL(url); err != nil {
		return err
	}
	if err := validateRef(ref); err != nil {
		return err
	}
	if err := validateCommit(sha); err != nil {
		return err
	}

	// Initialise a temporary git repo. Infer the object format from the SHA length:
	// a 64-hex commit requires a SHA-256 repo to avoid object-format mismatch on fetch.
	tmpRepo, err := os.MkdirTemp("", "wisp-fetch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRepo)

	initArgs := []string{"init", "-q"}
	if len(sha) == 64 {
		initArgs = append(initArgs, "--object-format=sha256")
	}
	if out, err := git(tmpRepo, initArgs...); err != nil {
		return fmt.Errorf("git init: %v: %s", err, out)
	}

	// hasCommit reports whether the pinned sha is present in tmpRepo.
	hasCommit := func() bool {
		out, err := exec.Command("git", append(append([]string{}, gitArgs...),
			"-C", tmpRepo, "cat-file", "-e", sha+"^{commit}")...).CombinedOutput()
		_ = out
		return err == nil
	}

	// SHA-valued-ref special case: skip the bare-SHA + ref-tip ladder and go
	// straight to all-refs mirror. There is no symbolic tip to fall back to.
	if isFullSHA(ref) {
		// Initial bare-SHA attempt.
		_, shaErr := git(tmpRepo, "fetch", "--depth", "1", "--", url, sha)
		if fetchTrace != nil {
			fetchTrace(fetchStep{fetchSHA, shaErr})
		}
		if shaErr != nil || !hasCommit() {
			// Escalate to all-refs mirror.
			out, allErr := git(tmpRepo, "fetch", "--", url, "+refs/*:refs/*")
			if fetchTrace != nil {
				fetchTrace(fetchStep{fetchAllRefs, allErr})
			}
			if allErr != nil {
				return fmt.Errorf("%s: all-refs fetch: %v: %s", sha, allErr, out)
			}
			if !hasCommit() {
				return fmt.Errorf("%s: commit not found after all-refs fetch from %s", sha, url)
			}
		}
	} else {
		// Non-SHA ref: try bare-SHA first, then ref-tip fallback.
		_, shaErr := git(tmpRepo, "fetch", "--depth", "1", "--", url, sha)
		if fetchTrace != nil {
			fetchTrace(fetchStep{fetchSHA, shaErr})
		}
		if shaErr != nil || !hasCommit() {
			// Ref-tip fetch (shallow).
			out, refErr := git(tmpRepo, "fetch", "--depth", "1", "--", url, ref)
			if fetchTrace != nil {
				fetchTrace(fetchStep{fetchRefTip, refErr})
			}
			if refErr != nil {
				return fmt.Errorf("%s: fetch ref %s: %v: %s", sha, ref, refErr, out)
			}
			if !hasCommit() {
				// Unshallow to get the full history including the ancestor commit.
				out2, unshallowErr := git(tmpRepo, "fetch", "--unshallow", "--", url, ref)
				if fetchTrace != nil {
					fetchTrace(fetchStep{fetchRefTip, unshallowErr})
				}
				if unshallowErr != nil {
					return fmt.Errorf("%s: unshallow fetch ref %s: %v: %s", sha, ref, unshallowErr, out2)
				}
				if !hasCommit() {
					return fmt.Errorf("%s: commit not found after full fetch of %s from %s", sha, ref, url)
				}
			}
		}
	}

	// Extract the tree using ls-tree + cat-file: no smudge filters, no encoding
	// conversion -- blob bytes delivered verbatim from the object store.
	return extractFromGit(tmpRepo, sha, destDir)
}

// extractFromGit runs ls-tree -r -z on the pinned commit and extracts each blob
// into destDir via cat-file, then calls extractEntries.
func extractFromGit(tmpRepo, sha, destDir string) error {
	// ls-tree -r -z: recursive, NUL-terminated entries.
	// Format per entry: "<mode> <type> <oid>\t<path>\0"
	out, err := exec.Command("git", append(append([]string{}, gitArgs...),
		"-C", tmpRepo, "ls-tree", "-r", "-z", sha)...).Output()
	if err != nil {
		// Fallback to CombinedOutput for the error message.
		out2, _ := exec.Command("git", append(append([]string{}, gitArgs...),
			"-C", tmpRepo, "ls-tree", "-r", "-z", sha)...).CombinedOutput()
		return fmt.Errorf("ls-tree %s: %v: %s", sha, err, out2)
	}

	var entries []entry
	// Split on NUL; each element (before final empty) is "<mode> <type> <oid>\t<path>".
	parts := strings.Split(string(out), "\x00")
	for _, part := range parts {
		// Entries are NUL-terminated (ls-tree -z), so the final element is empty.
		// Do NOT trim newlines: with -z a path can legally contain '\n', and
		// trimming would corrupt such filenames and change the tree/digest.
		if part == "" {
			continue
		}
		// Split at the tab: left is "<mode> <type> <oid>", right is "<path>".
		tab := strings.IndexByte(part, '\t')
		if tab < 0 {
			return fmt.Errorf("ls-tree: unexpected line format: %q", part)
		}
		meta := part[:tab]
		path := part[tab+1:]
		metaParts := strings.Fields(meta)
		if len(metaParts) != 3 {
			return fmt.Errorf("ls-tree: unexpected meta format: %q", meta)
		}
		mode, typ, oid := metaParts[0], metaParts[1], metaParts[2]
		_ = typ

		var em entryMode
		switch mode {
		case "100644", "100755":
			em = modeRegular
		case "120000":
			em = modeSymlink
		case "160000":
			// Gitlink (submodule): hard reject.
			return fmt.Errorf("fetch %s: gitlink (submodule) entry %q is not allowed", sha, path)
		default:
			em = modeSpecial
		}

		// Read blob content via cat-file blob <oid>.
		blobOut, err := exec.Command("git", append(append([]string{}, gitArgs...),
			"-C", tmpRepo, "cat-file", "blob", oid)...).Output()
		if err != nil {
			return fmt.Errorf("cat-file blob %s (%s): %v", oid, path, err)
		}
		entries = append(entries, entry{rel: path, mode: em, content: blobOut})
	}

	return extractEntries(destDir, entries)
}
