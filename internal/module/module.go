// Package module resolves a wisp build's module graph: starting from the root
// source file it discovers the project root, parses every imported/included
// module, assigns each a compiler module id (modid), detects cycles, and reads
// package manifests. It performs NO type checking or codegen (those are
// internal/types and internal/codegen); it is pure resolution plus filesystem
// reading.
//
// Dependency rule (no import cycle): this package imports only parser/ast/token.
// internal/types imports THIS package (for CheckLinked); the edge is one-way.
package module

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/core"
	"github.com/mitchellnemitz/wisp/internal/lexer"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// Diag is a located resolution diagnostic.
type Diag struct {
	Pos token.Position
	Msg string
}

// Module is one resolved .wisp file in the build.
type Module struct {
	ID         int              // compiler-assigned modid; root = 0
	Path       string           // canonical (symlink-resolved, absolute) path; identity
	Prog       *ast.Program     // parsed AST (positions carry this file's path)
	Src        string           // file contents (for diagnostic snippets)
	Namespaces map[string]int   // alias -> modid, from this module's imports/includes
	Chain      []token.Position // import/include positions from root to this module
	// Core is non-empty iff this is a synthetic core module (json, ...): the
	// reserved namespace name. A synthetic module has an empty Prog and is never
	// the root; the type checker resolves its members through the core catalog
	// rather than Prog-derived symbol tables.
	Core string
}

// Linked is the whole resolved program.
type Linked struct {
	Modules []*Module // index == modid; Modules[0] is the root
	Root    string    // resolved project-root directory (for diagnostics)
}

// FS abstracts the filesystem so the loader is testable. OSFS is the production
// implementation.
type FS interface {
	ReadFile(path string) ([]byte, error)
	// Stat reports whether path exists and, if so, whether it is a directory.
	Stat(path string) (exists, isDir bool)
	// EvalSymlinks returns the canonical (symlink-resolved) absolute path of an
	// existing path; it errors if the path does not exist.
	EvalSymlinks(path string) (string, error)
	// Abs returns a cleaned absolute path (no symlink resolution, no existence
	// requirement).
	Abs(path string) (string, error)
}

var (
	identRe       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	repoSegmentRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// loader holds the in-progress resolution state.
type loader struct {
	fs      FS
	root    string // canonical project-root directory
	wispDir string // <root>/.wisp (canonical-ish)
	modules []*Module
	byPath  map[string]*Module // canonical path -> module (dedup)
	stack   []string           // canonical paths currently being resolved (cycles)
	diags   []Diag
}

// Load resolves the module graph rooted at rootFile (whose already-read contents
// are rootSrc; every OTHER file is read through fs). It returns the linked program
// or, on any error, the collected diagnostics (Linked is nil iff len(diags) > 0).
func Load(rootFile, rootSrc string, fs FS) (*Linked, []Diag) {
	l := &loader{fs: fs, byPath: map[string]*Module{}}

	rootAbs, err := fs.Abs(rootFile)
	if err != nil {
		return nil, []Diag{{Msg: "cannot resolve source path " + rootFile + ": " + err.Error()}}
	}
	// Project root: nearest ancestor dir of the root file containing wisp.json,
	// else the root file's own directory.
	startDir := filepath.Dir(rootAbs)
	l.root = DiscoverRoot(l.fs, startDir)
	// Canonicalize the root so containment checks (which compare against
	// symlink-resolved candidate paths) use the same form; e.g. on macOS a temp
	// dir under /var is really /private/var.
	if c, err := fs.EvalSymlinks(l.root); err == nil {
		l.root = c
	}
	l.wispDir = filepath.Join(l.root, ".wisp")
	// Canonicalize .wisp too (when present) so the containment and .wisp-exclusion
	// checks compare symlink-resolved realpaths like-for-like with resolved targets;
	// a symlinked .wisp would otherwise misclassify or bypass the exclusion.
	if exists, isDir := fs.Stat(l.wispDir); exists && isDir {
		if c, err := fs.EvalSymlinks(l.wispDir); err == nil {
			l.wispDir = c
		}
	}

	// Root module identity: its canonical path if on disk, else its absolute path
	// (a not-on-disk root, e.g. an in-memory test/golden source, can never be the
	// realpath target of an include, so the fallback identity is harmless).
	rootPath := rootAbs
	if canon, err := fs.EvalSymlinks(rootAbs); err == nil {
		rootPath = canon
	}

	prog, perr := parser.Parse(rootSrc, rootFile)
	if perr != nil {
		return nil, []Diag{fromParseError(perr)}
	}
	rootMod := &Module{ID: 0, Path: rootPath, Prog: prog, Src: rootSrc, Namespaces: map[string]int{}}
	l.modules = append(l.modules, rootMod)
	l.byPath[rootPath] = rootMod

	l.resolveDirectives(rootMod)

	if len(l.diags) > 0 {
		return nil, l.diags
	}
	return &Linked{Modules: l.modules, Root: l.root}, nil
}

// containmentRoot returns the directory a module's includes may not escape: the
// package directory for an in-package module (under <root>/.wisp/modules/o/r/),
// else the project root.
func (l *loader) containmentRoot(modPath string) string {
	modulesDir := filepath.Join(l.wispDir, "modules")
	if within(modPath, modulesDir) {
		// in-package: containment root is <modulesDir>/owner/repo.
		rel, err := filepath.Rel(modulesDir, modPath)
		if err == nil {
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) >= 2 {
				return filepath.Join(modulesDir, parts[0], parts[1])
			}
		}
	}
	return l.root
}

// resolveDirectives resolves every import/include of m (recursively), recording
// alias->modid in m.Namespaces and detecting binding collisions and cycles.
func (l *loader) resolveDirectives(m *Module) {
	l.stack = append(l.stack, m.Path)
	defer func() { l.stack = l.stack[:len(l.stack)-1] }()

	for _, inc := range m.Prog.Includes {
		l.resolveInclude(m, inc)
	}
	for _, imp := range m.Prog.Imports {
		l.resolveImport(m, imp)
	}
}

// bindNamespace records alias -> id in m.Namespaces, reporting a collision.
func (l *loader) bindNamespace(m *Module, alias string, id int, pos token.Position) {
	if _, dup := m.Namespaces[alias]; dup {
		l.errf(pos, "namespace %q is already bound in this module; use a distinct `as` alias", alias)
		return
	}
	m.Namespaces[alias] = id
}

func (l *loader) resolveInclude(m *Module, inc *ast.IncludeDecl) {
	path := inc.Path
	if filepath.IsAbs(path) {
		l.errf(inc.PathPos, "include path must be relative, got %q", path)
		return
	}
	if !strings.HasSuffix(path, ".wisp") {
		l.errf(inc.PathPos, "include path must end in .wisp, got %q", path)
		return
	}
	includingDir := filepath.Dir(m.Path)
	candidate := filepath.Clean(filepath.Join(includingDir, path))

	// Must exist and be a file.
	if exists, isDir := l.fs.Stat(candidate); !exists || isDir {
		l.errf(inc.PathPos, "include cannot find %q (resolved to %s)", path, candidate)
		return
	}
	canon, err := l.fs.EvalSymlinks(candidate)
	if err != nil {
		l.errf(inc.PathPos, "include cannot resolve %q: %v", path, err)
		return
	}
	// Containment: must stay within the including module's containment root.
	cRoot := l.containmentRoot(m.Path)
	if !within(canon, cRoot) {
		l.errf(inc.PathPos, "include %q escapes its containment root %s", path, cRoot)
		return
	}
	// A root-project include may not reach into <root>/.wisp/ (packages are
	// import-only). In-package includes have containment root = the package dir,
	// which is itself under .wisp/, so this check applies only to root-project.
	if cRoot == l.root && within(canon, l.wispDir) {
		l.errf(inc.PathPos, "include cannot reach into .wisp/ (installed packages are reachable only via import)")
		return
	}

	alias := inc.Alias
	if alias == "" {
		alias = stem(path)
		if !validIdent(alias) {
			l.errf(inc.PathPos, "include default namespace %q (the file stem) is not a valid identifier; add an explicit `as alias`", alias)
			return
		}
	} else if !validIdent(alias) {
		l.errf(inc.AliasPos, "namespace alias %q is not a valid identifier or is a reserved word", alias)
		return
	}

	target := l.loadModule(canon, inc.PathPos, m.Chain)
	if target == nil {
		return // error already reported (cycle or parse)
	}
	l.bindNamespace(m, alias, target.ID, inc.PathPos)
}

func (l *loader) resolveImport(m *Module, imp *ast.ImportDecl) {
	// A reserved bare path (json, ...) resolves to a synthetic core module rather
	// than a fetched package. Table-driven via internal/core; this branch never
	// names a specific namespace.
	if core.IsNamespace(imp.Path) {
		l.resolveCoreImport(m, imp)
		return
	}
	owner, repo, ok := l.splitRepo(imp.Path, imp.PathPos)
	if !ok {
		return
	}
	pkgDir := filepath.Join(l.wispDir, "modules", owner, repo)
	if exists, isDir := l.fs.Stat(pkgDir); !exists || !isDir {
		l.errf(imp.PathPos, "import %q: package not found under %s (project root %s)", imp.Path, pkgDir, l.root)
		return
	}
	manifestPath := filepath.Join(pkgDir, "wisp.json")
	man, merr := l.readManifest(manifestPath)
	if merr != "" {
		l.errf(imp.PathPos, "import %q: %s", imp.Path, merr)
		return
	}
	if !validIdent(man.Name) {
		l.errf(imp.PathPos, "import %q: package name %q is not a valid identifier or is a reserved word", imp.Path, man.Name)
		return
	}
	// Entry file: relative, .wisp, confined to the package dir.
	entry := man.Main
	if entry == "" {
		l.errf(imp.PathPos, "import %q: wisp.json has no \"main\" entry file", imp.Path)
		return
	}
	if filepath.IsAbs(entry) || !strings.HasSuffix(entry, ".wisp") {
		l.errf(imp.PathPos, "import %q: wisp.json \"main\" must be a relative .wisp path, got %q", imp.Path, entry)
		return
	}
	entryPath := filepath.Clean(filepath.Join(pkgDir, entry))
	if exists, isDir := l.fs.Stat(entryPath); !exists || isDir {
		l.errf(imp.PathPos, "import %q: entry file %q not found in package", imp.Path, entry)
		return
	}
	canon, err := l.fs.EvalSymlinks(entryPath)
	if err != nil {
		l.errf(imp.PathPos, "import %q: cannot resolve entry %q: %v", imp.Path, entry, err)
		return
	}
	pkgCanon, err := l.fs.EvalSymlinks(pkgDir)
	if err != nil {
		l.errf(imp.PathPos, "import %q: cannot resolve package dir: %v", imp.Path, err)
		return
	}
	if !within(canon, pkgCanon) {
		l.errf(imp.PathPos, "import %q: entry file escapes the package directory", imp.Path)
		return
	}

	alias := imp.Alias
	if alias == "" {
		alias = man.Name
	} else if !validIdent(alias) {
		l.errf(imp.AliasPos, "namespace alias %q is not a valid identifier or is a reserved word", alias)
		return
	}

	target := l.loadModule(canon, imp.PathPos, m.Chain)
	if target == nil {
		return
	}
	l.bindNamespace(m, alias, target.ID, imp.PathPos)
}

// resolveCoreImport binds a reserved bare path (json, ...) to a synthetic core
// module. Exactly one synthetic Module is interned per reserved namespace and
// shared across all importers, keyed in byPath under a sentinel path that can
// never be a real filesystem path (a NUL byte is illegal in a POSIX path). The
// synthetic module has an empty Prog, so it is a no-op in every checker pass; its
// members resolve through the core catalog, not Prog symbol tables.
func (l *loader) resolveCoreImport(m *Module, imp *ast.ImportDecl) {
	name := imp.Path
	key := "\x00core/" + name
	target, ok := l.byPath[key]
	if !ok {
		target = &Module{
			ID:         len(l.modules),
			Path:       key,
			Prog:       &ast.Program{},
			Namespaces: map[string]int{},
			Core:       name,
		}
		l.modules = append(l.modules, target)
		l.byPath[key] = target
	}

	alias := imp.Alias
	if alias == "" {
		alias = name
	} else if !validIdent(alias) {
		l.errf(imp.AliasPos, "namespace alias %q is not a valid identifier or is a reserved word", alias)
		return
	}
	l.bindNamespace(m, alias, target.ID, imp.PathPos)
}

// loadModule returns the Module for canonical path canon, parsing+recursing on
// first sight (dedup otherwise). reachedAt is the directive position; parentChain
// is the chain of the module that reached it. Returns nil on a cycle or parse
// error (already reported).
func (l *loader) loadModule(canon string, reachedAt token.Position, parentChain []token.Position) *Module {
	// Cycle FIRST: a module currently on the resolution stack is mid-resolution, so
	// it is also already in byPath; returning it as a dedup hit would mask the
	// cycle. A fully-resolved module has been popped from the stack, so a diamond
	// (re-reaching a completed module) falls through to the byPath dedup below.
	for _, p := range l.stack {
		if p == canon {
			l.errf(reachedAt, "import/include cycle: %s is already being resolved", canon)
			return nil
		}
	}
	if existing, ok := l.byPath[canon]; ok {
		return existing
	}
	b, err := l.fs.ReadFile(canon)
	if err != nil {
		l.errf(reachedAt, "cannot read %s: %v", canon, err)
		return nil
	}
	src := string(b)
	prog, perr := parser.Parse(src, canon)
	if perr != nil {
		d := fromParseError(perr)
		d.Msg = d.Msg + chainSuffix(append(parentChain, reachedAt))
		l.diags = append(l.diags, d)
		return nil
	}
	chain := append(append([]token.Position{}, parentChain...), reachedAt)
	m := &Module{ID: len(l.modules), Path: canon, Prog: prog, Src: src, Namespaces: map[string]int{}, Chain: chain}
	l.modules = append(l.modules, m)
	l.byPath[canon] = m
	l.resolveDirectives(m)
	return m
}

// splitRepo validates and splits an `owner/repo` import string.
func (l *loader) splitRepo(s string, pos token.Position) (owner, repo string, ok bool) {
	owner, repo, err := SplitRepo(s)
	if err != nil {
		l.errf(pos, "import %s", err.Error())
		return "", "", false
	}
	return owner, repo, true
}

type manifest struct {
	Name string `json:"name"`
	Main string `json:"main"`
}

// readManifest parses wisp.json, returning the manifest or a non-empty error
// string. Unknown fields (dependencies, etc.) are ignored (M9 tool's).
func (l *loader) readManifest(path string) (manifest, string) {
	b, err := l.fs.ReadFile(path)
	if err != nil {
		return manifest{}, "cannot read wisp.json: " + err.Error()
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return manifest{}, "malformed wisp.json: " + err.Error()
	}
	if m.Name == "" {
		return manifest{}, "wisp.json has no \"name\""
	}
	return m, ""
}

func (l *loader) errf(pos token.Position, format string, args ...any) {
	l.diags = append(l.diags, Diag{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// --- helpers ---

// within reports whether path is dir or a descendant of dir (string comparison on
// cleaned absolute paths).
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// stem returns a path's base name without its .wisp extension.
func stem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".wisp")
}

// validIdent reports whether s is a valid wisp identifier and not a reserved word.
func validIdent(s string) bool {
	if !identRe.MatchString(s) {
		return false
	}
	k, isKeyword := token.Lookup(s)
	if !isKeyword {
		return true
	}
	switch k {
	case token.TypeInt, token.TypeBool, token.TypeString, token.Float, token.Error:
		return true
	default:
		return false
	}
}

// chainSuffix renders an import/include chain as a diagnostic trailer.
func chainSuffix(chain []token.Position) string {
	if len(chain) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(" (imported from ")
	for i, p := range chain {
		if i > 0 {
			b.WriteString(" -> ")
		}
		b.WriteString(p.String())
	}
	b.WriteString(")")
	return b.String()
}

func fromParseError(err error) Diag {
	var pe *parser.Error
	if errors.As(err, &pe) {
		return Diag{Pos: pe.Pos, Msg: pe.Msg}
	}
	var le *lexer.Error
	if errors.As(err, &le) {
		return Diag{Pos: le.Pos, Msg: le.Msg}
	}
	return Diag{Msg: err.Error()}
}
