// Package residualcheck implements the PR-A residual-usage gate: a
// content-scoped scan for the pre-rename wisp-language spellings that PR A
// (mitchellnemitz/wisp#66) replaced --
//
//	strings.* / arrays.*                 -> string.* / array.*   (namespaces)
//	string(x) / int(x) / float(x) / bool(x) -> to_string/to_int/to_float/to_bool
//
// The scan is intentionally content-scoped, not a blind grep: it must not
// flag Go's own "strings" stdlib package (strings.Contains, ...), Go's own
// type-conversion syntax (int(x) as ordinary Go code), or negative tests
// whose entire purpose is to assert an old spelling is now rejected.
//
// This package is deliberately kept separate and reusable: the plan
// (docs/plans/2026-07-01-modules-only-universal-funcrefs.md, PR C scope)
// commits PR C's completeness gate to reusing "the same content-scoped
// allowlist/classifier mechanism and exclusions" as this one, so the two
// gates share one consistent scope.
package residualcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Kind identifies which residual spelling a Violation reports.
type Kind string

const (
	KindNamespace  Kind = "old-namespace"  // strings.<member> / arrays.<member>
	KindConversion Kind = "old-conversion" // bare string(/int(/float(/bool(
)

// Violation is one flagged residual old-spelling occurrence.
type Violation struct {
	File string
	Line int
	Kind Kind
	Text string // the offending line, trimmed
}

// oldNamespaceRe matches old-style namespace.member references: strings.foo,
// arrays.bar. The member must start lowercase, which excludes Go's own
// "strings" stdlib package (every exported member is capitalized:
// strings.Contains, strings.Split, ...) while still matching wisp-shaped
// member names (string.trim, string.reverse are wisp's lowercase convention).
var oldNamespaceRe = regexp.MustCompile(`\b(strings|arrays)\.([a-z_][a-zA-Z0-9_]*)`)

// oldConversionRe matches a bare pre-rename conversion call. It is only ever
// applied to content already classified as wisp source (see looksLikeWisp),
// never to raw Go code or comments, where string(x)/int(x)/etc. are ordinary
// Go type conversions or unrelated English.
var oldConversionRe = regexp.MustCompile(`\b(string|int|float|bool)\(`)

// wispShapeRe is a heuristic for "this text is (or embeds) wisp source". It
// matches common statement shapes seen in embedded wisp snippets and .wisp
// fixtures (a full `fn main() { ... }` program, or a bare body fragment such
// as `let s: string = string(1)` as used by table-driven negative tests).
var wispShapeRe = regexp.MustCompile(`\bfn\s+\w*\s*\([^)]*\)\s*(->|\{)|\blet\s+\w+\s*:\s*\w|\bimport\s+"[a-z]+"`)

// namespaceMemberExclude lists dot-member names that trip oldNamespaceRe by
// coincidence but are never a wisp namespace reference -- e.g. the literal
// filenames examples/arrays.wisp and examples/strings.wisp, referenced as
// plain Go string literals (not wisp source) in test tables.
var namespaceMemberExclude = map[string]bool{
	"wisp": true,
}

// allowedSites documents Go source locations where an old spelling is
// expected to appear ON PURPOSE -- a negative test asserting the old
// spelling is correctly rejected. Extend this list (never weaken the
// regexes above) when a new legitimate negative test is added. Keyed by
// repo-relative file path -> set of enclosing top-level function names.
var allowedSites = map[string]map[string]bool{
	"internal/types/checker_test.go": {
		"TestBuiltin_BareConversionSpellingsRejected": true,
	},
}

// SkipDirs lists repo-relative directories excluded from the scan entirely:
// this package (the gate itself, whose source necessarily mentions the old
// spellings in regex literals and doc comments) and the historical, dated
// planning archive (docs/notes, docs/plans, docs/specs), which intentionally
// preserves pre-rename spellings as a record of what the repo looked like
// when those documents were written, not as living documentation.
var SkipDirs = []string{
	"internal/residualcheck",
	"docs/notes",
	"docs/plans",
	"docs/specs",
}

// ScanRoots are the directories (relative to repo root) this gate scans, per
// the plan's migration-surfaces list: compiler/tooling internals, examples,
// docs, and editor integrations.
var ScanRoots = []string{
	"internal",
	"examples",
	"docs",
	"editors",
}

// skipped expects relPath already normalized to forward slashes (see
// filepath.ToSlash in ScanRepo); SkipDirs entries are slash-separated too, so
// this comparison must not use filepath.Separator (backslash on Windows).
func skipped(relPath string) bool {
	for _, d := range SkipDirs {
		if relPath == d || strings.HasPrefix(relPath, d+"/") {
			return true
		}
	}
	return false
}

// ScanRepo walks ScanRoots under repoRoot and returns every residual
// old-spelling occurrence not covered by allowedSites.
func ScanRepo(repoRoot string) ([]Violation, error) {
	var out []Violation
	for _, root := range ScanRoots {
		start := filepath.Join(repoRoot, root)
		if _, err := os.Stat(start); err != nil {
			continue
		}
		err := filepath.Walk(start, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if skipped(rel) {
				return nil
			}
			vs, err := scanFile(rel, path)
			if err != nil {
				return err
			}
			out = append(out, vs...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func scanFile(relPath, absPath string) ([]Violation, error) {
	switch filepath.Ext(absPath) {
	case ".wisp":
		return scanWispFile(relPath, absPath)
	case ".go":
		return scanGoFile(relPath, absPath)
	case ".md":
		return scanMarkdownFile(relPath, absPath)
	default:
		return nil, nil
	}
}

func scanWispFile(relPath, absPath string) ([]Violation, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	var out []Violation
	for i, line := range strings.Split(string(content), "\n") {
		out = append(out, findNamespace(relPath, i+1, line)...)
		out = append(out, findConversion(relPath, i+1, line)...)
	}
	return out, nil
}

// scanMarkdownFile checks docs prose for old namespace mentions everywhere,
// and old conversion spellings only inside ```wisp fenced code blocks (wisp
// source examples), so generic English use of "int(...)" in prose is not
// flagged.
func scanMarkdownFile(relPath, absPath string) ([]Violation, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	var out []Violation
	inWispFence := false
	for i, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inWispFence && trimmed == "```wisp" {
				inWispFence = true
			} else if inWispFence {
				inWispFence = false
			}
			continue
		}
		out = append(out, findNamespace(relPath, i+1, line)...)
		if inWispFence {
			out = append(out, findConversion(relPath, i+1, line)...)
		}
	}
	return out, nil
}

func findNamespace(relPath string, line int, text string) []Violation {
	var out []Violation
	for _, m := range oldNamespaceRe.FindAllStringSubmatch(text, -1) {
		if namespaceMemberExclude[m[2]] {
			continue
		}
		out = append(out, Violation{File: relPath, Line: line, Kind: KindNamespace, Text: strings.TrimSpace(text)})
	}
	return out
}

func findConversion(relPath string, line int, text string) []Violation {
	if !oldConversionRe.MatchString(text) {
		return nil
	}
	return []Violation{{File: relPath, Line: line, Kind: KindConversion, Text: strings.TrimSpace(text)}}
}

// scanGoFile parses the file and inspects:
//   - every comment, for old namespace mentions (prose describing wisp
//     syntax, e.g. a stale "strings.reverse" doc comment);
//   - every string literal, for old namespace mentions always, and for old
//     bare conversion calls when the literal's content looks like wisp
//     source (see wispShapeRe) -- never for raw Go code, where int(x)/
//     string(x)/etc. are ordinary Go type conversions.
//
// Matches inside a function listed in allowedSites for this file are
// dropped: those are negative tests whose entire purpose is asserting the
// old spelling is now rejected.
func scanGoFile(relPath, absPath string) ([]Violation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	allowedFuncs := allowedSites[relPath]

	type funcRange struct {
		name       string
		start, end token.Pos
	}
	var funcRanges []funcRange
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			funcRanges = append(funcRanges, funcRange{fd.Name.Name, fd.Pos(), fd.End()})
		}
	}
	enclosingFunc := func(pos token.Pos) string {
		for _, fr := range funcRanges {
			if pos >= fr.start && pos <= fr.end {
				return fr.name
			}
		}
		return ""
	}
	isAllowed := func(pos token.Pos) bool {
		if len(allowedFuncs) == 0 {
			return false
		}
		return allowedFuncs[enclosingFunc(pos)]
	}

	var out []Violation
	emit := func(pos token.Pos, kind Kind, text string) {
		if isAllowed(pos) {
			return
		}
		p := fset.Position(pos)
		out = append(out, Violation{File: relPath, Line: p.Line, Kind: kind, Text: strings.TrimSpace(text)})
	}

	for _, cg := range f.Comments {
		for _, c := range cg.List {
			for _, m := range oldNamespaceRe.FindAllStringSubmatchIndex(c.Text, -1) {
				member := c.Text[m[4]:m[5]]
				if namespaceMemberExclude[member] {
					continue
				}
				emit(c.Pos(), KindNamespace, c.Text)
			}
		}
	}

	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		content := unquoteBestEffort(lit.Value)
		for _, m := range oldNamespaceRe.FindAllStringSubmatchIndex(content, -1) {
			member := content[m[4]:m[5]]
			if namespaceMemberExclude[member] {
				continue
			}
			emit(lit.Pos(), KindNamespace, firstLine(content))
		}
		if wispShapeRe.MatchString(content) && oldConversionRe.MatchString(content) {
			emit(lit.Pos(), KindConversion, firstLine(content))
		}
		return true
	})
	return out, nil
}

// unquoteBestEffort returns the literal's decoded content for both
// interpreted ("...") and raw (`...`) Go string literals. Raw literals have
// no escape sequences, so backtick-stripping is exact; interpreted literals
// go through strconv.Unquote, falling back to the raw token text if that
// somehow fails (never expected for a lexed BasicLit).
func unquoteBestEffort(tok string) string {
	if strings.HasPrefix(tok, "`") {
		return strings.Trim(tok, "`")
	}
	if s, err := strconv.Unquote(tok); err == nil {
		return s
	}
	return tok
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
