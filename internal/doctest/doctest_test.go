package doctest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
	"github.com/mitchellnemitz/wisp/internal/lexer"
)

const repoRoot = "../.."

// exampleCase is one program under examples/ with the arguments to run it with
// and the exact stdout it must produce. Pinning the output guards both the
// programs and the behavior the docs describe.
type exampleCase struct {
	file string
	args []string
	want string
}

var exampleCases = []exampleCase{
	{file: "hello.wisp", want: "hello, world\n"},
	{file: "structs.wisp", want: "point (3, -4)\nmoved to (10, -4)\nmanhattan distance: 14\n"},
	{file: "arrays.wisp", want: "count: 4\nfirst: 3\nreversed first: 5\nsum: 11\nhas 2: true\n"},
	{file: "dicts.wisp", want: "alice is 30\nbob is 25\ncarol is 41\nhas dave: false\nnumber of people: 3\n"},
	{file: "errors.wisp", want: "10 / 2 = 5\ncaught: examples/errors.wisp:4:14: division by zero\ncleanup runs either way\n"},
	{file: "higher_order.wisp", want: "doubled first: 2\neven count: 2\nsum: 15\nindirect add: 42\n"},
	{file: "stdlib.wisp", want: "trimmed: Hello, World\nlower: hello, world\nupper: HELLO, WORLD\nreplaced: Hello, wisp\nparts: 3\njoined: a | b | c\nstarts with Hello: true\nindex of comma: 5\ndivider: ==========\n"},
	{file: "wordcount.wisp", args: []string{"apple", "banana", "apple"}, want: "apple: 2\nbanana: 1\n"},
	{file: "io.wisp", want: "ran: hello from run\nboth: first / second\nhas PATH: true\nPATH is non-empty: true\n"},
	{file: "collections.wisp", want: "1 1 2 3 4 5 6 9\n9 6 5 4 3 2 1 1\n2\ntrue false\n4 1 5\n0 3 1 4 1 5 9 2 6\n31\n0 1 2 3 4\n3 6\napple,pear,plum\n3,2,5\n2 0\napple,plum\napple,plum,kiwi apple=10\n"},
	{file: "collections_tail.wisp", want: "index 20: 1\nindex 99 none: true\ncount >2: 3\nflat len: 5\nflat[2]: 3\nunique len: 3\nunique[0]: 3\ntake 3 len: 3\ndrop 2 len: 3\ndrop 2 [0]: 3\npopped: 40\nafter pop len: 3\nafter remove_at[0]: 20\nafter insert_at[0]=99: 99\nafter insert_at len: 3\nsize: 3\nafter clear size: 0\nafter re-add size: 1\n"},
	{file: "strings.wisp", want: "Hello\nW\nindex: 8\ncount l: 3\nHell0, World\n[spaced]\n[spaced]\nhappy\nrun\n[00042]\n[hi...]\nempty: true\nlines: 3\nalpha / beta / gamma\n"},
	{file: "numeric.wisp", want: "int_or: 42 -1\nfloat_or: 3.1400000000000001\nclamp: 10 2\nsign: -1 0 1\nfloor/ceil/round: 2 3 3\ntrunc: -3\nsqrt: 12 0.5\ngcd/lcm: 12 12\n"},
	{file: "optional.wisp", want: "some: 42\nnone is_none: true\nor: -1\nfirst even at: 2\nget x: 10\nget y: 0\n"},
	{file: "generics.wisp", want: "first int: 10\nfirst str: a\nidentity: 42\nempty has none\n"},
	{file: "generics_bounds.wisp", want: "has 7: true\nhas 8: false\nhas b: true\n"},
	{file: "fs.wisp", want: "cwd absolute: true\ncwd exists: true\nsh resolvable: true\nPATH set: true\ncwd not empty: true\n"},
	{file: "regex.wisp", want: "matches digits: true\nfirst match: 1\nall matches: 1,22,333\nreplaced: a#b#\n"},
	{file: "program_path.wisp", want: "/usr/local/bin\nscript.sh\n.\nscript.sh\nself dir is non-empty: true\n"},
	{file: "tuples.wisp", want: "code: 0\nout: success\nmsg: failed\n17/5 = 3 rem 2\n"},
	{file: "enum.wisp", want: "color: go\nvalue: 1\nis green: true\nis red: false\nexit code: 2\n"},
	{file: "args.wisp", want: "hello, ada\nverbose: true\nfiles: report.txt, notes.txt\n= form name: ada\nout: stdout\n"},
}

func TestExamplesCompileRunAndMatchOutput(t *testing.T) {
	// Every example under examples/ must be covered by a case here, so a new
	// example cannot be added without pinning its behavior.
	matches, err := filepath.Glob(filepath.Join(repoRoot, "examples", "*.wisp"))
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, c := range exampleCases {
		covered[c.file] = true
	}
	for _, m := range matches {
		if !covered[filepath.Base(m)] {
			t.Errorf("examples/%s has no doctest case; add it to exampleCases", filepath.Base(m))
		}
	}

	for _, c := range exampleCases {
		c := c
		t.Run(c.file, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(repoRoot, "examples", c.file))
			if err != nil {
				t.Fatal(err)
			}
			// The label is the filename the located-abort machinery prints, so it
			// is fixed here rather than tied to the on-disk path.
			label := "examples/" + c.file
			var out, errb bytes.Buffer
			code := driver.Run(label, string(src), c.args, &out, &errb)
			if code != 0 {
				t.Fatalf("exit %d, stderr:\n%s", code, errb.String())
			}
			if out.String() != c.want {
				t.Errorf("stdout mismatch\n got: %q\nwant: %q", out.String(), c.want)
			}
		})
	}
}

// docFiles are the user-facing docs whose wisp code blocks must stay valid. The
// design specs and plans under docs/specs and docs/plans are intentionally
// excluded: they are historical and may contain illustrative pseudo-code.
func docFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	guide, err := filepath.Glob(filepath.Join(repoRoot, "docs", "guide", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, guide...)
	for _, f := range []string{"docs/README.md", "README.md", "CONTRIBUTING.md", "AGENTS.md"} {
		files = append(files, filepath.Join(repoRoot, f))
	}
	return files
}

type codeBlock struct {
	file string
	line int // 1-based line of the opening fence
	src  string
}

// wispBlocks extracts every fenced block whose info string is exactly "wisp".
func wispBlocks(t *testing.T, path string) []codeBlock {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var blocks []codeBlock
	lines := strings.Split(string(b), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```wisp" {
			continue
		}
		start := i + 1
		var body []string
		i++
		for i < len(lines) && strings.TrimSpace(lines[i]) != "```" {
			body = append(body, lines[i])
			i++
		}
		blocks = append(blocks, codeBlock{file: path, line: start, src: strings.Join(body, "\n")})
	}
	return blocks
}

// TestDocBlocksLex requires every wisp code block in the user docs to tokenize
// without a lexical error. This catches typos in illustrative fragments that
// are not complete programs.
func TestDocBlocksLex(t *testing.T) {
	total := 0
	for _, f := range docFiles(t) {
		for _, blk := range wispBlocks(t, f) {
			total++
			if _, err := lexer.Lex(blk.src, blk.file); err != nil {
				t.Errorf("%s:%d: wisp block fails to lex: %v", blk.file, blk.line, err)
			}
		}
	}
	if total == 0 {
		t.Fatal("found no wisp code blocks; the extractor or the docs changed")
	}
	t.Logf("lexed %d wisp code blocks", total)
}

// TestDocProgramsCompileAndRun compiles and runs every doc block that is a
// complete program, identified by containing a main function. Fragments
// without a main are covered by the lex test above and by examples/.
func TestDocProgramsCompileAndRun(t *testing.T) {
	programs := 0
	for _, f := range docFiles(t) {
		for _, blk := range wispBlocks(t, f) {
			if !strings.Contains(blk.src, "fn main(") && !strings.Contains(blk.src, "fn main ") {
				continue
			}
			programs++
			label := blk.file
			_, _, diags := driver.Compile(label, blk.src)
			var errs []string
			for _, d := range diags {
				if d.Severity == driver.Error {
					errs = append(errs, d.String())
				}
			}
			if len(errs) > 0 {
				t.Errorf("%s:%d: doc program does not compile:\n%s", blk.file, blk.line, strings.Join(errs, "\n"))
				continue
			}
			var out, errb bytes.Buffer
			if code := driver.Run(label, blk.src, nil, &out, &errb); code != 0 {
				t.Errorf("%s:%d: doc program exited %d, stderr:\n%s", blk.file, blk.line, code, errb.String())
			}
		}
	}
	t.Logf("compiled and ran %d doc programs", programs)
}
