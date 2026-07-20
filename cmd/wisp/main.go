// Command wisp is the wisp compiler CLI. It is a thin dispatcher over
// internal/driver; all compile/build/run/check logic lives there.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/doc"
	"github.com/mitchellnemitz/wisp/internal/driver"
	"github.com/mitchellnemitz/wisp/internal/format"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/pkgtool"
	"github.com/mitchellnemitz/wisp/internal/testrunner"
	"github.com/mitchellnemitz/wisp/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = "usage: wisp <build|run|check|fmt|test|doc|add|install|remove> [args] (per-command syntax below)\n" +
	"  build <src> [-o <out>] [--source-map]   compile to a .sh script (default out: src with .sh ext)\n" +
	"  run <src> [args...]                     compile and execute, propagating its exit status\n" +
	"  check <src>                             type-check only; report diagnostics\n" +
	"  fmt <path>... [-w | --check]            canonically format files/dirs; -w rewrites in place, --check verifies\n" +
	"  test [path] [--tap] [--filter <re>] [--shell <name>] [--coverage]   run *_test.wisp files cross-shell\n" +
	"  doc <path>...                           extract /// doc-comments to Markdown\n" +
	"  add <owner/repo> [--git <url>] [--ref <ref>]   add/repin a dependency, then install\n" +
	"  install                                 fetch/verify dependencies from wisp.json + wisp.lock\n" +
	"  remove <owner/repo>                     drop a dependency, prune its module tree, update the lock\n"

// run dispatches the CLI. It returns the process exit code. Usage/argument
// errors exit 2; compile errors exit 1 (or non-zero); `run` propagates the
// script's exit status.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "wisp %s\n%s", version.Number, usage)
		return 2
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "help", "-h", "--help":
		return cmdHelp(rest, stdout, stderr)
	case "version", "--version":
		fmt.Fprintf(stdout, "wisp %s\n", version.Number)
		return 0
	}

	if len(rest) > 0 && (rest[0] == "-h" || rest[0] == "--help") {
		if h, ok := commandHelp(cmd); ok {
			fmt.Fprint(stdout, h)
			return 0
		}
	}

	switch cmd {
	case "build":
		return cmdBuild(rest, stderr)
	case "run":
		return cmdRun(rest, stdout, stderr)
	case "check":
		return cmdCheck(rest, stderr)
	case "fmt":
		return cmdFmt(rest, stdout, stderr)
	case "doc":
		return cmdDoc(rest, stdout, stderr)
	case "test":
		return cmdTest(rest, stdout, stderr)
	case "add", "install", "remove":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "wisp: %v\n", err)
			return 1
		}
		root := module.DiscoverRoot(module.OSFS{}, cwd)
		return pkgRun(root, cmd, rest, stdout, stderr, pkgtool.GitFetcher())
	default:
		fmt.Fprintf(stderr, "wisp: unknown command %q\n%s", cmd, usage)
		return 2
	}
}

// commandHelp returns the per-command help text (the usage summary line plus
// that command's own usage line) for a known command, derived from the usage
// const itself. ok is false for an unrecognized command.
func commandHelp(cmd string) (string, bool) {
	lines := strings.Split(usage, "\n")
	summary := lines[0]
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == cmd {
			return summary + "\n" + line + "\n", true
		}
	}
	return "", false
}

// cmdHelp implements `wisp help [cmd]`.
func cmdHelp(rest []string, stdout, stderr io.Writer) int {
	if len(rest) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if h, ok := commandHelp(rest[0]); ok {
		fmt.Fprint(stdout, h)
		return 0
	}
	fmt.Fprintf(stderr, "wisp: unknown command %q\n%s", rest[0], usage)
	return 2
}

// readSource loads a source file, writing a usage-style error to stderr and
// returning ok=false (caller should exit 2) on failure.
func readSource(path string, stderr io.Writer) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "wisp: %v\n", err)
		return "", false
	}
	return string(b), true
}

func cmdBuild(args []string, stderr io.Writer) int {
	var src, out string
	var sourceMap bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			// Reject a missing or flag-shaped following token so `-o --source-map`
			// does not create a file literally named "--source-map" (L5).
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				fmt.Fprintf(stderr, "wisp: -o requires a path argument\n%s", usage)
				return 2
			}
			out = args[i+1]
			i++
		case "--source-map":
			sourceMap = true
		default:
			// A leading-dash token in the source position is an unknown flag, not a
			// filename (consistent with check/run/fmt, L3).
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(stderr, "wisp: unknown flag %q\n%s", args[i], usage)
				return 2
			}
			if src != "" {
				fmt.Fprintf(stderr, "wisp: unexpected argument %q\n%s", args[i], usage)
				return 2
			}
			src = args[i]
		}
	}
	if src == "" {
		fmt.Fprintf(stderr, "wisp: build requires a source file\n%s", usage)
		return 2
	}
	if out == "" {
		out = driver.DefaultOutPath(src)
	}
	// Refuse to overwrite the source with the build output (H7): a usage error,
	// before reading or compiling anything.
	if driver.SamePath(out, src) {
		fmt.Fprintf(stderr, "wisp: refusing to build %s: the output path is the same as the source; pass -o to choose a different path\n%s", src, usage)
		return 2
	}
	source, ok := readSource(src, stderr)
	if !ok {
		return 2
	}
	return driver.Build(src, source, out, sourceMap, stderr)
}

func cmdRun(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "wisp: run requires a source file\n%s", usage)
		return 2
	}
	// run does not define compiler flags (e.g. --source-map is build-only). The
	// source file comes first; a flag in its place is a usage error. Tokens after
	// the source are passed through as the script's own argv.
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "wisp: unknown flag %q\n%s", args[0], usage)
		return 2
	}
	src := args[0]
	scriptArgs := args[1:]
	source, ok := readSource(src, stderr)
	if !ok {
		return 2
	}
	return driver.Run(src, source, scriptArgs, stdout, stderr)
}

// walkWispFiles returns all *.wisp files recursively under root, sorted
// lexically by cleaned path, excluding any directory named ".wisp" (the
// package/module cache directory, internal/module/module.go:104). Mirrors the
// filepath.WalkDir shape of testrunner.discoverTestFiles, generalized for
// fmt's directory mode: a different suffix (.wisp, not _test.wisp) and an
// added .wisp/ skip that discoverTestFiles does not have.
func walkWispFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".wisp" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".wisp") {
			files = append(files, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// cmdFmt implements `wisp fmt`. With no flag it prints the canonically
// formatted source to stdout, which requires exactly one resolved file. `-w`
// rewrites each resolved file in place (nothing to stdout, skipping a write
// when the file is already canonical so its mtime is preserved). `--check`
// writes nothing to stdout except one path per line (lexical order) for each
// non-canonical file, and exits non-zero if any file is non-canonical or
// failed to parse. `-w` and `--check` are mutually exclusive. A path may be a
// file or a directory (walked recursively for *.wisp, excluding .wisp/);
// multiple paths may be given and are deduped/merged. A parse error on any
// file is reported to stderr and processing continues with the rest.
func cmdFmt(args []string, stdout, stderr io.Writer) int {
	var rawPaths []string
	var write, check bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-w":
			write = true
		case "--check":
			check = true
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(stderr, "wisp: unknown flag %q\n%s", args[i], usage)
				return 2
			}
			rawPaths = append(rawPaths, args[i])
		}
	}
	if len(rawPaths) == 0 {
		fmt.Fprintf(stderr, "wisp: fmt requires a source file\n%s", usage)
		return 2
	}
	if write && check {
		fmt.Fprintf(stderr, "wisp: fmt -w and --check are mutually exclusive\n%s", usage)
		return 2
	}

	// Fast path: a single path that is not an existing directory runs the
	// original single-file body verbatim, guaranteeing byte-for-byte
	// compatibility with today's behavior in all three modes.
	singleFile := len(rawPaths) == 1
	if singleFile {
		if info, err := os.Stat(rawPaths[0]); err == nil && info.IsDir() {
			singleFile = false
		}
	}
	if singleFile {
		src := rawPaths[0]
		source, ok := readSource(src, stderr)
		if !ok {
			return 2
		}

		formatted, err := format.Format(source, src)
		if err != nil {
			// A parse error: write the located error to stderr, nothing to stdout.
			fmt.Fprintln(stderr, err.Error())
			return 1
		}

		switch {
		case check:
			// Silent on stdout in both cases; exit reflects whether it was formatted.
			if source != formatted {
				return 1
			}
			return 0
		case write:
			if source == formatted {
				return 0 // no change needed; avoid touching the file's mtime
			}
			if err := os.WriteFile(src, []byte(formatted), 0o644); err != nil {
				fmt.Fprintf(stderr, "wisp: write %s: %v\n", src, err)
				return 1
			}
			return 0
		default:
			fmt.Fprint(stdout, formatted)
			return 0
		}
	}

	// Multi-path branch: resolve each path (file -> itself, dir -> walked
	// recursively), dedupe by cleaned path, sort lexically. Reaching here
	// means the input is not a single explicit file, so the no-flag guard
	// collapses to one check covering every sub-case (a bare directory, an
	// empty directory, >1 explicit files, or a mix).
	resolved := map[string]bool{}
	for _, p := range rawPaths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(stderr, "wisp: %v\n", err)
			return 2
		}
		if info.IsDir() {
			files, err := walkWispFiles(p)
			if err != nil {
				fmt.Fprintf(stderr, "wisp: %v\n", err)
				return 2
			}
			for _, f := range files {
				resolved[f] = true
			}
			continue
		}
		resolved[filepath.Clean(p)] = true
	}
	var files []string
	for f := range resolved {
		files = append(files, f)
	}
	sort.Strings(files)

	if !write && !check {
		fmt.Fprintf(stderr, "wisp: fmt to stdout requires exactly one file (use -w or --check)\n")
		return 2
	}

	var sawParseErr, sawDirty bool
	for _, f := range files {
		source, ok := readSource(f, stderr)
		if !ok {
			sawParseErr = true
			continue
		}
		formatted, err := format.Format(source, f)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			sawParseErr = true
			continue
		}
		switch {
		case check:
			if source != formatted {
				fmt.Fprintln(stdout, f)
				sawDirty = true
			}
		case write:
			if source == formatted {
				continue // no change needed; avoid touching the file's mtime
			}
			if err := os.WriteFile(f, []byte(formatted), 0o644); err != nil {
				fmt.Fprintf(stderr, "wisp: write %s: %v\n", f, err)
				sawParseErr = true
			}
		}
	}
	if sawParseErr {
		return 1
	}
	if check && sawDirty {
		return 1
	}
	return 0
}

// pkgRun dispatches add/install/remove with the given Fetcher. Separated from
// run so CLI tests can inject a fake fetcher directly.
func pkgRun(root, cmd string, rest []string, stdout, stderr io.Writer, f pkgtool.Fetcher) int {
	switch cmd {
	case "add":
		args, ok := parseAddArgs(rest, stderr)
		if !ok {
			return 2
		}
		return pkgtool.Add(root, args, f, stderr)
	case "install":
		if len(rest) != 0 {
			fmt.Fprintf(stderr, "wisp: install takes no arguments\n%s", usage)
			return 2
		}
		return pkgtool.Install(root, f, stderr)
	case "remove":
		if len(rest) != 1 {
			fmt.Fprintf(stderr, "wisp: remove requires exactly one owner/repo\n%s", usage)
			return 2
		}
		if _, _, err := module.SplitRepo(rest[0]); err != nil {
			fmt.Fprintf(stderr, "wisp: remove: invalid owner/repo %q: %v\n%s", rest[0], err, usage)
			return 2
		}
		return pkgtool.Remove(root, rest[0], f, stderr)
	}
	return 2
}

// parseAddArgs parses the rest of the `wisp add` command line.
// Returns (args, true) on success, or writes usage to stderr and returns (_, false).
func parseAddArgs(rest []string, stderr io.Writer) (pkgtool.AddArgs, bool) {
	if len(rest) < 1 {
		fmt.Fprintf(stderr, "wisp: add requires an owner/repo\n%s", usage)
		return pkgtool.AddArgs{}, false
	}
	args := pkgtool.AddArgs{Repo: rest[0]}
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case "--git":
			if i+1 >= len(rest) {
				fmt.Fprintf(stderr, "wisp: --git requires a URL argument\n%s", usage)
				return pkgtool.AddArgs{}, false
			}
			args.Git = rest[i+1]
			args.HasGit = true
			i++
		case "--ref":
			if i+1 >= len(rest) {
				fmt.Fprintf(stderr, "wisp: --ref requires a ref argument\n%s", usage)
				return pkgtool.AddArgs{}, false
			}
			args.Ref = rest[i+1]
			args.HasRef = true
			i++
		default:
			fmt.Fprintf(stderr, "wisp: unknown flag %q\n%s", rest[i], usage)
			return pkgtool.AddArgs{}, false
		}
	}
	// Validate the owner/repo key up front so an invalid key is a usage error
	// (exit 2) BEFORE wisp.json is touched, not a late exit-1 from the manifest
	// re-read inside install.
	if _, _, err := module.SplitRepo(args.Repo); err != nil {
		fmt.Fprintf(stderr, "wisp: add: invalid owner/repo %q: %v\n%s", args.Repo, err, usage)
		return pkgtool.AddArgs{}, false
	}
	return args, true
}

// cmdTest implements `wisp test [path] [--tap] [--filter <re>] [--shell <name>]`.
// Usage errors exit 2. Missing shells or test failures exit 1. All pass/skip exits 0.
func cmdTest(args []string, stdout, stderr io.Writer) int {
	var path, filter, shellOnly string
	var tap, coverage bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tap":
			tap = true
		case "--coverage":
			coverage = true
		case "--filter":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				fmt.Fprintf(stderr, "wisp: --filter requires a regexp argument\n%s", usage)
				return 2
			}
			filter = args[i+1]
			i++
		case "--shell":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				fmt.Fprintf(stderr, "wisp: --shell requires a shell name argument\n%s", usage)
				return 2
			}
			shellOnly = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(stderr, "wisp: unknown flag %q\n%s", args[i], usage)
				return 2
			}
			if path != "" {
				fmt.Fprintf(stderr, "wisp: unexpected argument %q\n%s", args[i], usage)
				return 2
			}
			path = args[i]
		}
	}

	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "wisp: %v\n", err)
			return 1
		}
	}

	opts := testrunner.Options{
		Path:      path,
		Filter:    filter,
		ShellOnly: shellOnly,
		TAP:       tap,
		Coverage:  coverage,
		Stdout:    stdout,
		Stderr:    stderr,
	}
	return testrunner.Run(opts)
}

func cmdCheck(args []string, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "wisp: check requires exactly one source file\n%s", usage)
		return 2
	}
	// check defines no flags (--source-map is build-only); a flag in the source
	// position is a usage error.
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "wisp: unknown flag %q\n%s", args[0], usage)
		return 2
	}
	src := args[0]
	source, ok := readSource(src, stderr)
	if !ok {
		return 2
	}
	return driver.Check(src, source, stderr)
}

// cmdDoc implements `wisp doc <path>...`. Each path may be a .wisp file or a
// directory; directories contribute their immediate (non-recursive) .wisp
// files sorted by filename. Output is buffered: on any error nothing is written
// to stdout. Usage/arg errors exit 2; IO/parse errors exit 1.
func cmdDoc(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: wisp doc <path>...")
		return 2
	}

	// Expand args to an ordered list of canonical .wisp paths.
	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(stderr, "wisp doc: %v\n", err)
			return 1
		}
		if info.IsDir() {
			entries, err := os.ReadDir(arg)
			if err != nil {
				fmt.Fprintf(stderr, "wisp doc: %v\n", err)
				return 1
			}
			var names []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".wisp") {
					names = append(names, e.Name())
				}
			}
			sort.Strings(names)
			for _, n := range names {
				files = append(files, filepath.Join(arg, n))
			}
			// empty dir (no .wisp files) contributes nothing -- not an error
		} else {
			if !strings.HasSuffix(arg, ".wisp") {
				fmt.Fprintf(stderr, "wisp doc: %s is not a .wisp file\n", arg)
				return 1
			}
			files = append(files, filepath.Clean(arg))
		}
	}

	// Parse and render all files into sections; fail-fast on any error.
	var sections []string
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "wisp doc: %v\n", err)
			return 1
		}
		prog, comments, perr := parser.ParseWithComments(string(src), path)
		if perr != nil {
			fmt.Fprintln(stderr, perr.Error())
			return 1
		}
		// Skip files with no documentable decls -- they would produce a bare
		// "## path\n\n" header with no content, which adds no useful information.
		if len(doc.Extract(prog, comments)) == 0 {
			continue
		}
		sections = append(sections, doc.Render(path, prog, comments))
	}

	// Write only on full success; zero sections (e.g. empty dir) -> empty stdout.
	if len(sections) > 0 {
		io.WriteString(stdout, strings.Join(sections, "\n"))
	}
	return 0
}
