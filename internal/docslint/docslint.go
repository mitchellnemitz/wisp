// Package docslint guards docs/stdlib-index.md and docs/guide/language.md
// against drift from the compiler's builtin tables.
package docslint

import (
	"fmt"
	"regexp"
	"strings"
)

var bulletLineRe = regexp.MustCompile(`(?m)^- ` + "`")
var spanRe = regexp.MustCompile("`([^`]*)`")
var identRe = regexp.MustCompile(`^(?:(?:\[(?:x| )\]|\[ref\])\s*){0,2}(\w+)\(`)

// StaleSpelling is one offending entry found by StaleBareSpellings.
type StaleSpelling struct {
	Line int    // 1-indexed line number within doc
	Name string // the stale bare identifier found
}

// StaleBareSpellings scans doc (the full text of stdlib-index.md, or a
// synthetic stand-in for testing) for bullet lines documenting a builtin
// call spelling, and flags any extracted identifier that is a member of
// removable (the RemovableBuiltins() set) -- i.e. documented using its old
// bare spelling instead of its RemovedHint()-qualified form.
//
// Only backtick spans appearing before a line's first " -- " prose
// separator are scanned, so prose usage notes like "use `exp(1.0)`" or
// "(`exit(n)`, or ...)" after " -- " are correctly excluded.
func StaleBareSpellings(doc string, removable []string) []StaleSpelling {
	removableSet := make(map[string]bool, len(removable))
	for _, n := range removable {
		removableSet[n] = true
	}

	var out []StaleSpelling
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if !bulletLineRe.MatchString(line) {
			continue
		}
		boundary := len(line)
		if idx := strings.Index(line, " -- "); idx >= 0 {
			boundary = idx
		}
		head := line[:boundary]
		for _, spanMatch := range spanRe.FindAllStringSubmatch(head, -1) {
			inner := spanMatch[1]
			m := identRe.FindStringSubmatch(inner)
			if m == nil {
				continue
			}
			name := m[1]
			if removableSet[name] {
				out = append(out, StaleSpelling{Line: i + 1, Name: name})
			}
		}
	}
	return out
}

// LanguageMdFuncrefExamples extracts every backtick-delimited name from
// bullet lines (lines starting with "- " after trimming) under the
// "Referenceable builtins" section of doc (the full text of language.md,
// or a synthetic stand-in), split by the section's three positive-class
// sub-headings, from the "Monomorphic-generatable" paragraph through the
// "Rejected" paragraph (rejected names are not collected -- they aren't
// required to belong to any funcref table). Only bullet lines are
// scanned, not prose paragraphs, since prose under a class heading may
// contain unrelated inline code spans (e.g. a `fn(string) -> string`
// type signature) that are not funcref examples. The section runs from
// the "#### Referenceable builtins" heading to the next heading of any
// level.
func LanguageMdFuncrefExamples(doc string) (mono, overloaded, generic []string, err error) {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "#### Referenceable builtins" {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return nil, nil, nil, fmt.Errorf("no %q heading found", "#### Referenceable builtins")
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
			end = i
			break
		}
	}

	section := lines[start:end]
	var current *[]string
	for _, line := range section {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "**Monomorphic-generatable**"):
			current = &mono
		case strings.HasPrefix(trimmed, "**Overloaded**"):
			current = &overloaded
		case strings.HasPrefix(trimmed, "**Generic**"):
			current = &generic
		case strings.HasPrefix(trimmed, "**Rejected"):
			current = nil
		}
		if current == nil || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		for _, spanMatch := range spanRe.FindAllStringSubmatch(line, -1) {
			name := spanMatch[1]
			if dot := strings.LastIndex(name, "."); dot >= 0 {
				name = name[dot+1:]
			}
			*current = append(*current, name)
		}
	}
	return mono, overloaded, generic, nil
}
