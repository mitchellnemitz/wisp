package pkgtool

import (
	"fmt"
	"os"
	"path/filepath"
)

type refKey struct{ url, ref string }

// fakeFetcher serves fixtures from in-memory trees keyed by SHA. No git, no network.
type fakeFetcher struct {
	resolve    map[refKey]string            // (url,ref) -> sha
	trees      map[string]map[string]string // sha -> relpath -> content
	failFetch  map[string]bool              // sha -> inject a mid-fetch failure
	defaultRef map[string]string            // url -> default branch name (e.g. "main")
}

func (f *fakeFetcher) DefaultRef(url string) (string, error) {
	if r, ok := f.defaultRef[url]; ok {
		return r, nil
	}
	return "", fmt.Errorf("fake: no default ref for %s", url)
}

func (f *fakeFetcher) Resolve(url, ref string) (string, error) {
	if sha, ok := f.resolve[refKey{url, ref}]; ok {
		return sha, nil
	}
	return "", fmt.Errorf("fake: no resolve for %s@%s", url, ref)
}

func (f *fakeFetcher) Fetch(url, ref, sha, destDir string) error {
	tree, ok := f.trees[sha]
	if !ok {
		return fmt.Errorf("fake: no tree for sha %s", sha)
	}
	for rel, content := range tree {
		full := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
		if f.failFetch[sha] {
			return fmt.Errorf("fake: injected fetch failure for %s", sha)
		}
	}
	if f.failFetch[sha] {
		return fmt.Errorf("fake: injected fetch failure for %s", sha)
	}
	return nil
}

type countingFetcher struct {
	inner       Fetcher
	fetches     int
	resolves    int
	defaultRefs int
}

func (c *countingFetcher) DefaultRef(url string) (string, error) {
	c.defaultRefs++
	return c.inner.DefaultRef(url)
}
func (c *countingFetcher) Resolve(url, ref string) (string, error) {
	c.resolves++
	return c.inner.Resolve(url, ref)
}
func (c *countingFetcher) Fetch(url, ref, sha, dest string) error {
	c.fetches++
	return c.inner.Fetch(url, ref, sha, dest)
}
