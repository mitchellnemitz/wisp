package types

// Diagnostics polish (M6 B3): "did you mean" suggestions. When the checker
// reports an unknown name, it appends a suggestion of the closest known name of
// the SAME KIND, but only when there is a UNIQUE candidate at the MINIMUM
// case-sensitive Levenshtein distance and that minimum is in [1, 2]. A tie at
// the minimum, or no candidate within distance 2, yields no suggestion. The
// suggestion is purely additive: it never changes which programs compile.

import (
	"sort"
	"strconv"
)

// maxSuggestDistance bounds how far a candidate may be to be suggested.
const maxSuggestDistance = 2

// suggestSuffix returns ` ; did you mean "X"?`-style suffix (with a leading
// "; ") for the unique closest candidate, or "" when none qualifies. The
// candidate pool should be the known names of the same kind as the unknown.
func suggestSuffix(unknown string, candidates []string) string {
	best, ok := bestCandidate(unknown, candidates)
	if !ok {
		return ""
	}
	return `; did you mean ` + strconv.Quote(best) + `?`
}

// bestCandidate finds the unique candidate at the minimum case-sensitive
// Levenshtein distance to unknown, requiring that minimum to be in [1, 2] and
// strictly unique (no tie). It returns ("", false) otherwise.
func bestCandidate(unknown string, candidates []string) (string, bool) {
	// Deduplicate and sort for determinism (the uniqueness check below makes the
	// order irrelevant to the result, but sorting keeps behavior stable).
	seen := map[string]bool{}
	uniq := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if !seen[c] {
			seen[c] = true
			uniq = append(uniq, c)
		}
	}
	sort.Strings(uniq)

	minDist := maxSuggestDistance + 1
	var best string
	tie := false
	for _, c := range uniq {
		d := levenshtein(unknown, c)
		if d == 0 {
			continue // identical name is not a suggestion
		}
		if d < minDist {
			minDist = d
			best = c
			tie = false
		} else if d == minDist {
			tie = true
		}
	}
	if best == "" || tie || minDist > maxSuggestDistance {
		return "", false
	}
	return best, true
}

// levenshtein returns the case-sensitive Levenshtein edit distance between a and
// b (insertions, deletions, substitutions each cost 1).
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// --- candidate pools (same-kind, per spec 3.2) ---

// varNamesInScope returns every variable name visible in the current scope
// stack (the pool for an unknown variable reference / assignment target).
func (c *checker) varNamesInScope() []string {
	var names []string
	for _, sc := range c.scopes {
		for name := range sc {
			names = append(names, name)
		}
	}
	return names
}

// funcAndBuiltinNames returns user function names plus reserved builtin names
// (the pool for an unknown called function).
func (c *checker) funcAndBuiltinNames() []string {
	var names []string
	for name := range c.cur.funcs {
		names = append(names, name)
	}
	for name := range builtinSigs {
		names = append(names, name)
	}
	return names
}

// typeNames returns the current module's struct names plus type-alias names and
// the primitive/built-in type names (the pool for an unknown type annotation).
// Aliases are valid type-annotation names, so they belong here; they are
// deliberately NOT added to structNames() (an alias cannot be a constructor).
func (c *checker) typeNames() []string {
	names := []string{"int", "float", "bool", "string", "void", "error"}
	for name := range c.cur.structs {
		names = append(names, name)
	}
	for name := range c.cur.aliases {
		names = append(names, name)
	}
	return names
}

// structNames returns the current module's declared struct names (the pool for an
// unknown struct type in a struct construction).
func (c *checker) structNames() []string {
	var names []string
	for name := range c.cur.structs {
		names = append(names, name)
	}
	return names
}
