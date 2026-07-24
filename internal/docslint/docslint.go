// Package docslint guards www/src/content/docs/stdlib-index.md,
// www/src/content/docs/guide/stdlib.md, and www/src/content/docs/guide/language.md
// against drift from the compiler's builtin tables: every catalog member must
// be documented (Completeness), a documented member's return type and arity
// must match its source signature where one is statically known
// (SignatureDrift), four funcref-class prose counts must match their
// source-of-truth set sizes (FuncrefCounts), and a moved builtin's old bare
// call spelling must not reappear anywhere in either stdlib doc
// (StaleBareSpellings).
//
// Deliberately out of scope: free-prose semantic claims ("float is
// excluded", "X remains deferred", "spans more than one container", and
// similar English assertions) have no clean anchor to a source symbol and
// are not checked here -- they are left to manual audit. This package
// checks structure and mechanically-derivable facts, not prose correctness
// in general.
package docslint

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/types"
)

var spanRe = regexp.MustCompile("`([^`]*)`")
var identRe = regexp.MustCompile(`^(?:(?:\[(?:x| )\]|\[ref\])\s*){0,2}(\w+)\(`)

// StaleSpelling is one offending entry found by StaleBareSpellings.
type StaleSpelling struct {
	Line int    // 1-indexed line number within doc
	Name string // the stale bare identifier found
}

// StaleBareSpellings scans doc (the full text of a stdlib doc, or a
// synthetic stand-in for testing) for any call-shaped backtick span (`name(`)
// naming a builtin in removable (the RemovableBuiltins() set, minus any
// caller-applied carve-out -- see internal/docslint's callers), and flags it
// regardless of where in the line or file it appears: a signature bullet, a
// prose sentence, or after a bullet's " -- " separator. A legitimate
// reference to a moved builtin must use its namespaced form (ns.member),
// which never matches identRe's bare \w+( shape -- there is no bare-name
// exception (FR-006).
func StaleBareSpellings(doc string, removable []string) []StaleSpelling {
	removableSet := make(map[string]bool, len(removable))
	for _, n := range removable {
		removableSet[n] = true
	}

	var out []StaleSpelling
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		for _, spanMatch := range spanRe.FindAllStringSubmatch(line, -1) {
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

// SignatureDrift compares each member in members (the compiler's catalog,
// from types.DocumentableMembers()) against its parsed DocEntry in entries,
// returning one formatted failure per drift. A member absent from entries is
// NOT reported here (that is Completeness's job) -- SignatureDrift only
// checks members that ARE documented.
//
//   - FR-002/FR-003: for a member with HasStaticSig true, its documented
//     entry must have HasStaticSig true with matching Arity and Return.
//   - FR-004: a member with HasStaticSig false is exempt from the arity/
//     return check, but FR-009 still requires its entry to carry the
//     no-static-signature marker (NoSigMarker true) -- a member with no
//     static signature that IS documented with a full "(params) -> Return"
//     signature, or with a malformed entry, is a failure (the exemption must
//     never silently swallow a real mismatch).
//   - FR-009: a member with HasStaticSig true whose entry is Malformed (the
//     signature grammar didn't parse) is a failure, not a silent skip.
func SignatureDrift(entries []DocEntry, members []types.DocMember, path string) []string {
	byID := make(map[string]DocEntry, len(entries))
	for _, e := range entries {
		if e.ID != "" {
			byID[e.ID] = e
		}
	}
	var out []string
	for _, m := range members {
		e, ok := byID[m.ID]
		if !ok {
			continue // Completeness's responsibility
		}
		if !m.HasStaticSig {
			if !e.NoSigMarker {
				out = append(out, fmt.Sprintf("%s:%d: %q: expected the %q marker (member has no static signature), actual an entry without it", path, e.Line, m.ID, noSigMarker))
			}
			continue
		}
		if e.Malformed {
			out = append(out, fmt.Sprintf("%s:%d: %q: expected a parseable %q signature (arity %d, return %s), actual an unparseable doc entry", path, e.Line, m.ID, "(params) -> Return", m.Arity, m.Return))
			continue
		}
		if !e.HasStaticSig {
			out = append(out, fmt.Sprintf("%s:%d: %q: expected a static signature (arity %d, return %s), actual the no-static-signature marker", path, e.Line, m.ID, m.Arity, m.Return))
			continue
		}
		if e.Arity != m.Arity {
			out = append(out, fmt.Sprintf("%s:%d: %q documented arity %d, source arity %d", path, e.Line, m.ID, e.Arity, m.Arity))
		}
		if e.Return != m.Return {
			out = append(out, fmt.Sprintf("%s:%d: %q documented return type %q, source return type %q", path, e.Line, m.ID, e.Return, m.Return))
		}
	}
	sort.Strings(out)
	return out
}

var (
	// NOTE: the mono anchor is deliberately the short prefix "Any of the N
	// builtins", NOT "...in this class": in the real language.md today that
	// full phrase is line-wrapped (L582 ends "Any of the 71 builtins in",
	// L583 begins "this class..."), and FuncrefCounts scans line by line, so
	// the longer regex would never match. "Any of the" occurs exactly once in
	// language.md (verified), so the short form is unambiguous.
	monoCountRe       = regexp.MustCompile(`Any of the (\d+) builtins`)
	labelCountRe      = regexp.MustCompile(`checker tracks (\d+) finer-grained labels`)
	overloadedCountRe = regexp.MustCompile(`\*\*Overloaded\*\* \(annotation selects the arm\)\. (\d+) builtins`)
	genericCountRe    = regexp.MustCompile(`\*\*Generic\*\* \(annotation selects the container shape\)\. (\d+) builtins`)
)

// FuncrefCount is one documented funcref-class count located in a doc: its
// class key, the documented number, and the 1-based line it was found on (so a
// drift failure can name file:line per FR-010).
type FuncrefCount struct {
	Class string // "mono" | "labels" | "overloaded" | "generic"
	Value int
	Line  int
}

// FuncrefCounts extracts the four documented funcref-class counts from doc
// (the full text of guide/language.md, or a synthetic stand-in), keyed by
// class: the monomorphic-generatable count ("mono"), the BuiltinFuncrefClass
// label-set count ("labels"), the overloaded count ("overloaded"), and the
// generic count ("generic"). It scans line by line so each returned
// FuncrefCount carries the exact line its phrase was found on. Each anchor
// regex is scoped to text that fits on a single source line in language.md
// today (the mono anchor is intentionally the short "Any of the N builtins"
// prefix because its full "...in this class" continuation wraps to the next
// line); if a future edit wraps an anchor's captured portion across lines it
// will surface as a loud "not found" error here, not a silent miss. It errors
// if any of the four phrases is absent.
func FuncrefCounts(doc string) (map[string]FuncrefCount, error) {
	specs := []struct {
		class string
		re    *regexp.Regexp
		desc  string
	}{
		{"mono", monoCountRe, "monomorphic-generatable count"},
		{"labels", labelCountRe, "label-set count"},
		{"overloaded", overloadedCountRe, "overloaded count"},
		{"generic", genericCountRe, "generic count"},
	}
	out := make(map[string]FuncrefCount, len(specs))
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		for _, s := range specs {
			if _, done := out[s.class]; done {
				continue
			}
			if m := s.re.FindStringSubmatch(line); m != nil {
				v, err := strconv.Atoi(m[1])
				if err != nil {
					return nil, fmt.Errorf("%s: %v", s.desc, err)
				}
				out[s.class] = FuncrefCount{Class: s.class, Value: v, Line: i + 1}
			}
		}
	}
	for _, s := range specs {
		if _, ok := out[s.class]; !ok {
			// Failure-message contract: class id + expected anchor + actual.
			return nil, fmt.Errorf("count class %q (%s): expected a line matching %q, actual none found (anchor phrase removed or reworded)", s.class, s.desc, s.re.String())
		}
	}
	return out, nil
}
