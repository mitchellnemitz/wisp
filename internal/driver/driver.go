// Package driver is the shared compile/build/run/check entry point used by both
// the CLI (cmd/wisp) and the golden harness. It wires the pipeline
// lexer -> parser -> type checker -> codegen and turns the per-stage results
// into a uniform diagnostic stream and process exit codes.
package driver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/codegen"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Severity classifies a diagnostic. Only Error gates compilation; Warning is
// informational and never changes an exit code (spec rules 6, 10).
type Severity int

const (
	// Error is a fatal compile diagnostic: no script is produced.
	Error Severity = iota
	// Warning is non-gating; the script still compiles.
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Diagnostic is a compiler message with a source position and severity.
type Diagnostic struct {
	Pos      token.Position
	Severity Severity
	Msg      string
}

// String renders the diagnostic as `file:line:col: message`, with the message
// sanitized for terminal display (control bytes replaced; see sanitizeForTerminal).
func (d Diagnostic) String() string {
	return d.Pos.String() + ": " + sanitizeForTerminal(d.Msg)
}

// Compile runs the full pipeline (lex -> parse -> check -> codegen) on src.
// It returns the generated script bytes, the per-generated-line source-position
// map (one entry per output line; nil entries have no wisp origin -- spec
// section 3.3), and all diagnostics (errors and warnings). On any error the
// script and lineMap are nil; warning-only programs still produce a script.
// filename is recorded in positions for diagnostics.
func Compile(filename, src string) (script []byte, lineMap []*codegen.SourcePos, diags []Diagnostic) {
	// Resolve the module graph (M8). For a single-file program with no
	// imports/includes and no wisp.json this yields one module and behaves exactly
	// as the pre-M8 single-program path (with modid-prefixed shell names).
	linked, ldiags := module.Load(filename, src, module.OSFS{})
	if len(ldiags) > 0 {
		out := make([]Diagnostic, len(ldiags))
		for i, d := range ldiags {
			out[i] = Diagnostic{Pos: d.Pos, Severity: Error, Msg: d.Msg}
		}
		return nil, nil, out
	}

	info := types.CheckLinked(linked)
	for _, e := range info.Errors {
		diags = append(diags, Diagnostic{Pos: e.Pos, Severity: Error, Msg: e.Msg})
	}
	for _, w := range info.Warnings {
		diags = append(diags, Diagnostic{Pos: w.Pos, Severity: Warning, Msg: w.Msg})
	}
	if len(info.Errors) > 0 {
		return nil, nil, diags
	}

	out, lm, err := codegen.GenerateLinked(linked, info)
	if err != nil {
		// An internal codegen inconsistency on an accepted program. Surface it
		// with no position rather than panicking.
		return nil, nil, append(diags, Diagnostic{Severity: Error, Msg: "codegen: " + err.Error()})
	}
	return out, lm, diags
}

// CompileCoverage is Compile in coverage mode (spec R15-R17): the generated
// script additionally writes a `__wisp_cov "<file>:<line>"` hit-marker before
// each executable statement, and the third return value is the instrumented
// (file,line) UNIVERSE -- the authoritative set of coverable lines the test
// runner reports against (NOT derived from the line map). On any error the
// script and universe are nil. Compile (coverage off) is unchanged and
// byte-identical; coverage is the only thing that alters the emitted script.
func CompileCoverage(filename, src string) (script []byte, universe []codegen.CoverInst, diags []Diagnostic) {
	linked, ldiags := module.Load(filename, src, module.OSFS{})
	if len(ldiags) > 0 {
		out := make([]Diagnostic, len(ldiags))
		for i, d := range ldiags {
			out[i] = Diagnostic{Pos: d.Pos, Severity: Error, Msg: d.Msg}
		}
		return nil, nil, out
	}

	info := types.CheckLinked(linked)
	for _, e := range info.Errors {
		diags = append(diags, Diagnostic{Pos: e.Pos, Severity: Error, Msg: e.Msg})
	}
	for _, w := range info.Warnings {
		diags = append(diags, Diagnostic{Pos: w.Pos, Severity: Warning, Msg: w.Msg})
	}
	if len(info.Errors) > 0 {
		return nil, nil, diags
	}

	out, _, universe, err := codegen.GenerateLinkedCoverage(linked, info)
	if err != nil {
		return nil, nil, append(diags, Diagnostic{Severity: Error, Msg: "codegen: " + err.Error()})
	}
	return out, universe, diags
}

// writeDiags writes every diagnostic to w, one per line, prefixing warnings
// with "warning: " so they are distinguishable from errors on stderr. When the
// diagnostic carries a usable source position IN THE ROOT FILE, the offending
// source line and a caret are appended below it (spec section 5). A diagnostic
// from an imported/included module (its Pos.File differs from rootFile) is
// printed as the located message line only, since this function holds only the
// root source -- the position is still exact (M8).
func writeDiags(w io.Writer, rootFile, src string, diags []Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(w, renderDiag(rootFile, src, d))
	}
}

// renderDiag renders one diagnostic: the `file:line:col: message` line (warnings
// carry a "warning: " marker), then, when the position points at a real source
// line, that source line and a caret line under the column (spec section 5).
//
// Caret alignment (normative, spec section 5): the caret line copies the source
// line's own leading whitespace verbatim up to the column (so a source tab
// becomes a tab in the caret line and the ^ lands correctly), then pads with
// spaces to the byte column, then a single ^. Columns are byte-based to match
// the lexer.
func renderDiag(rootFile, src string, d Diagnostic) string {
	var head string
	if d.Severity == Warning {
		head = d.Pos.String() + ": warning: " + sanitizeForTerminal(d.Msg)
	} else {
		head = d.String()
	}

	// Only the root file's source is available here; skip the snippet for a
	// diagnostic that originates in another module (its position is still exact).
	if d.Pos.File != "" && d.Pos.File != rootFile {
		return head
	}

	line := sourceLine(src, d.Pos.Line)
	if line == "" || d.Pos.Line <= 0 || d.Pos.Col <= 0 {
		return head
	}

	// Gutter: "  <n> | " for the source line, "    | " for the caret line, sized
	// to the line-number width.
	num := strconv.Itoa(d.Pos.Line)
	srcGutter := "  " + num + " | "
	caretGutter := "  " + strings.Repeat(" ", len(num)) + " | "

	caret := buildCaret(line, d.Pos.Col)

	var b strings.Builder
	b.WriteString(head)
	b.WriteByte('\n')
	b.WriteString(srcGutter)
	b.WriteString(sanitizeForTerminal(line))
	b.WriteByte('\n')
	b.WriteString(caretGutter)
	b.WriteString(caret)
	return b.String()
}

// sourceLine returns the 1-based line n of src without its terminating newline
// (and without a trailing carriage return from a CRLF source, L6), or "" when n
// is out of range.
func sourceLine(src string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(src, "\n")
	if n > len(lines) {
		return ""
	}
	return strings.TrimSuffix(lines[n-1], "\r")
}

// buildCaret builds the caret-line content for byte column col (1-based) under
// line. Lexer columns are byte-based, but the caret must align by DISPLAY width:
// the prefix walks the runes of line up to byte offset col-1, emitting a tab
// verbatim for a source tab (so the ^ shares the tab stop) and a single space
// for every other rune -- one display column per rune, so a multibyte rune
// (counted once) does not push the caret right by its extra bytes (M7). A
// trailing partial rune (col-1 landing mid-rune, which should not happen for a
// real token position) contributes one space defensively.
func buildCaret(line string, col int) string {
	var b strings.Builder
	target := col - 1
	for i, r := range line {
		if i >= target {
			break
		}
		if r == '\t' {
			b.WriteByte('\t')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteByte('^')
	return b.String()
}

// sanitizeForTerminal replaces every C0 control byte (except \t) and DEL with
// its Unicode Control Pictures glyph (U+2400+r; DEL -> U+2421), one rune out per
// rune in, so a hostile source line or message cannot inject terminal escape
// sequences or spoof output. Bytes >= 0x80 (valid multibyte UTF-8) are left
// untouched; \n/\r never reach this function (sourceLine strips them first).
func sanitizeForTerminal(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 0x7f:
			b.WriteRune(0x2421)
		case r < 0x20 && r != '\t':
			b.WriteRune(0x2400 + r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hasError reports whether diags contains a gating (Error) diagnostic.
func hasError(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// Check compiles src for diagnostics only (no output). It writes every
// diagnostic to stderr and returns an exit code: non-zero when an error is
// present, 0 otherwise (warnings never gate).
func Check(filename, src string, stderr io.Writer) int {
	_, _, diags := Compile(filename, src)
	writeDiags(stderr, filename, src, diags)
	if hasError(diags) {
		return 1
	}
	return 0
}

// Build compiles src and, on success, writes the generated script to outPath
// (mode 0o755). Diagnostics go to stderr. It returns 0 on success (even with
// warnings) and 1 on a compile error, in which case no file is written.
//
// When sourceMap is true it additionally writes the source map to outPath+".map"
// AFTER the .sh is written; if that write fails the .sh remains as the
// authoritative artifact but Build returns non-zero (spec section 6,
// partial-failure). The .sh bytes are identical whether or not sourceMap is set.
func Build(filename, src, outPath string, sourceMap bool, stderr io.Writer) int {
	// Refuse to write when the output path resolves to the source file itself
	// (e.g. `wisp build foo.sh` -> DefaultOutPath returns foo.sh). Writing would
	// destroy the source (H7). This is a defensive backstop; the CLI also rejects
	// it as a usage error before reaching here.
	if SamePath(outPath, filename) {
		fmt.Fprintf(stderr, "wisp: refusing to build %s: the output path is the same as the source; pass -o to choose a different path\n", filename)
		return 1
	}
	script, lineMap, diags := Compile(filename, src)
	writeDiags(stderr, filename, src, diags)
	if hasError(diags) {
		return 1
	}
	if err := os.WriteFile(outPath, script, 0o755); err != nil {
		fmt.Fprintf(stderr, "wisp: write %s: %v\n", outPath, err)
		return 1
	}
	// os.WriteFile applies the mode only when creating the file; an existing
	// output keeps its old mode. Force 0755 so a rebuild over a stale, non-exec
	// file is still executable (L4).
	if err := os.Chmod(outPath, 0o755); err != nil {
		fmt.Fprintf(stderr, "wisp: chmod %s: %v\n", outPath, err)
		return 1
	}
	if sourceMap {
		mapPath := outPath + ".map"
		mapBytes, err := buildSourceMap(outPath, filename, script, lineMap)
		if err != nil {
			fmt.Fprintf(stderr, "wisp: source map %s: %v\n", mapPath, err)
			return 1
		}
		if err := os.WriteFile(mapPath, mapBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "wisp: write %s: %v\n", mapPath, err)
			return 1
		}
	}
	return 0
}

// Run compiles src to a temporary script, executes it under the host /bin/sh
// with args, removes the temp afterward, and returns the script's exit status
// (which equals main's return value, or 1 from __wisp_fail). Compile errors are
// written to stderr and return 1 without executing. Warnings are written to
// stderr but do not change the exit code. stdout/stderr/stdin are wired to the
// passed writers (and os.Stdin).
func Run(filename, src string, args []string, stdout, stderr io.Writer) int {
	script, _, diags := Compile(filename, src)
	writeDiags(stderr, filename, src, diags)
	if hasError(diags) {
		return 1
	}

	tmp, err := os.CreateTemp("", "wisp-*.sh")
	if err != nil {
		fmt.Fprintf(stderr, "wisp: temp file: %v\n", err)
		return 1
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(script); err != nil {
		tmp.Close()
		fmt.Fprintf(stderr, "wisp: write temp: %v\n", err)
		return 1
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(stderr, "wisp: close temp: %v\n", err)
		return 1
	}

	cmd := exec.Command("/bin/sh", append([]string{tmpName}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code >= 0 {
			return code
		}
		return 1 // killed by signal
	}
	fmt.Fprintf(stderr, "wisp: exec: %v\n", err)
	return 1
}

// DefaultOutPath returns the build output path for src when -o is absent: the
// source path with its extension replaced by .sh.
func DefaultOutPath(src string) string {
	ext := filepath.Ext(src)
	return src[:len(src)-len(ext)] + ".sh"
}

// SamePath reports whether a and b denote the same filesystem location. It first
// compares symlink-resolved absolute paths (so two different spellings of an
// existing file match); when a path does not exist yet (the common build case,
// where the output has not been created), it falls back to a cleaned absolute
// comparison. It is conservative: any error resolving either side falls back to
// the cleaned-absolute compare rather than reporting a false negative.
func SamePath(a, b string) bool {
	abs := func(p string) string {
		if r, err := filepath.Abs(p); err == nil {
			return filepath.Clean(r)
		}
		return filepath.Clean(p)
	}
	aa, ab := abs(a), abs(b)
	if aa == ab {
		return true
	}
	ra, ea := filepath.EvalSymlinks(aa)
	rb, eb := filepath.EvalSymlinks(ab)
	if ea == nil && eb == nil {
		return ra == rb
	}
	return false
}
