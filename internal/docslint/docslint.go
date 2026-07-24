// Package docslint guards www/src/content/docs/stdlib-index.md and
// www/src/content/docs/guide/language.md against drift from the compiler's
// builtin tables.
package docslint

import (
	"fmt"
	"regexp"
	"sort"
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

// DocEntry is one parsed member entry from a bullet line of a stdlib doc.
// Arity/Return are only meaningful when HasStaticSig is true; Malformed is
// true when the bullet's first backtick span could not be parsed into
// either the static-signature grammar or the no-static-signature marker
// grammar, even though a leading identifier was extractable (FR-009).
type DocEntry struct {
	Line         int
	ID           string
	Raw          string // verbatim first backtick-span text (for malformed reporting)
	HasStaticSig bool
	NoSigMarker  bool
	Arity        int
	Return       string
	Malformed    bool
}

var (
	docFirstSpanRe = regexp.MustCompile("^- `([^`]*)`")
	docCheckboxRe  = regexp.MustCompile(`^\[(?:x| )\]\s`)
	// The optional (?:\[[^\]]*\])? segment tolerates a generic type-param
	// list between the id and its params, e.g. "assert_eq[T: comparable](got:
	// T, want: T) -> void" -- the bracket is not part of the DocMember ID
	// (DocumentableMembers() ids never carry one) and is skipped, not captured.
	docSigRe    = regexp.MustCompile(`^(?:\[(?:x| )\]\s+)(?:\[ref\]\s+)?([A-Za-z_][\w]*(?:\.[A-Za-z_][\w]*)?)(?:\[[^\]]*\])?\((.*)\)\s*->\s*(.+)$`)
	docNoSigRe  = regexp.MustCompile(`^(?:\[(?:x| )\]\s+)([A-Za-z_][\w]*(?:\.[A-Za-z_][\w]*)?)$`)
	docLeadIDRe = regexp.MustCompile(`^(?:\[(?:x| )\]\s+)(?:\[ref\]\s+)?([A-Za-z_][\w]*(?:\.[A-Za-z_][\w]*)?)`)
	// docKeywordSkipRe matches a checkbox span whose leading identifier is
	// immediately followed by a bare "<placeholder>" token, e.g.
	// "[x] throw <error>" -- pseudo-syntax documenting a language KEYWORD
	// (throw/try/catch/finally are not DocumentableMembers() ids), not a
	// catalog member entry. Such a span must be skipped as non-member,
	// never reported Malformed (FR-009's Malformed case is reserved for a
	// checkbox span that genuinely looks like a broken member entry).
	docKeywordSkipRe = regexp.MustCompile(`^(?:\[(?:x| )\]\s+)(?:\[ref\]\s+)?[A-Za-z_]\w*\s+<`)
	noSigMarker      = "-- no static signature"
)

// splitTopLevel splits s on commas that are not nested inside (), [], or {},
// so a param list containing a bracketed/braced type (e.g. "a: {K: V}") is
// not miscounted as extra parameters.
func splitTopLevel(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var parts []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// ParseDocMembers scans doc for every MEMBER bullet -- a line starting with
// "- `" whose first backtick span begins with the "[x]"/"[ ]" checkbox marker
// (the doc-format signal that the bullet documents a catalog member) -- and
// parses that span into a DocEntry. A "- `...`" bullet whose first span has no
// checkbox is prose or an example (e.g. a bare combinator illustration), not a
// member entry, and is skipped entirely so it can never be misreported as a
// malformed member. Likewise, a checkbox span documenting a language keyword's
// pseudo-syntax (docKeywordSkipRe, e.g. "[x] throw <error>") is skipped as
// non-member. A checkbox-bearing span that matches neither the
// static-signature grammar nor the no-static-signature marker grammar is
// recorded as Malformed (with Raw + a lenient leading ID when extractable),
// caught later by ParseViolations (FR-009).
func ParseDocMembers(doc string) []DocEntry {
	var out []DocEntry
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		m := docFirstSpanRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		span := m[1]
		if !docCheckboxRe.MatchString(span) {
			continue // not a member entry (no checkbox) -- prose/example
		}
		if docKeywordSkipRe.MatchString(span) {
			continue // not a member entry -- language-keyword pseudo-syntax
		}
		rest := line[len(m[0]):]
		if sm := docSigRe.FindStringSubmatch(span); sm != nil {
			out = append(out, DocEntry{
				Line: i + 1, ID: sm[1], Raw: span, HasStaticSig: true,
				Arity: len(splitTopLevel(sm[2])), Return: strings.TrimSpace(sm[3]),
			})
			continue
		}
		if sm := docNoSigRe.FindStringSubmatch(span); sm != nil && strings.HasPrefix(strings.TrimSpace(rest), noSigMarker) {
			out = append(out, DocEntry{Line: i + 1, ID: sm[1], Raw: span, NoSigMarker: true})
			continue
		}
		entry := DocEntry{Line: i + 1, Raw: span, Malformed: true}
		if lm := docLeadIDRe.FindStringSubmatch(span); lm != nil {
			entry.ID = lm[1]
		}
		out = append(out, entry)
	}
	return out
}

// Completeness reports every id in ids with no corresponding DocEntry.ID in
// entries, sorted. No allowlist: FR-001 requires every catalog member to be
// documented, with no opt-out.
func Completeness(entries []DocEntry, ids []string) []string {
	documented := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.ID != "" {
			documented[e.ID] = true
		}
	}
	var missing []string
	for _, id := range ids {
		if !documented[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}

// ParseViolations reports doc-format failures that would let the
// machine-parseable format be silently abandoned (FR-007/FR-009): every
// Malformed member entry (a checkbox bullet whose span parsed as neither a
// static signature nor the no-static-signature marker) and every duplicate
// member ID (the same ID documented on more than one bullet). Each failure
// carries path:line, and for malformed entries the expected grammar and the
// actual span. Returned sorted for stable output.
func ParseViolations(entries []DocEntry, path string) []string {
	var out []string
	firstLine := make(map[string]int, len(entries))
	for _, e := range entries {
		if e.Malformed {
			// FR-010: id + expected + actual + path:line. When the span is too
			// broken to extract an id, use the "<unparsed>" sentinel.
			id := e.ID
			if id == "" {
				id = "<unparsed>"
			}
			out = append(out, fmt.Sprintf("%s:%d: member %q: expected a parseable entry (%q or %q), actual an unparseable span %q", path, e.Line, id, "[x] ns.member(params) -> Return", "[x] ns.member (with -- no static signature)", e.Raw))
			continue
		}
		if e.ID == "" {
			continue
		}
		if prev, dup := firstLine[e.ID]; dup {
			out = append(out, fmt.Sprintf("%s:%d: member %q: expected exactly one entry, actual a duplicate (first documented at line %d)", path, e.Line, e.ID, prev))
			continue
		}
		firstLine[e.ID] = e.Line
	}
	sort.Strings(out)
	return out
}
