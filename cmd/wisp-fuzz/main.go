// Command wisp-fuzz is the local-only cross-shell differential fuzzer driver.
// Never invoked by per-PR CI (FR-014).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mitchellnemitz/wisp/internal/fuzz"
	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "seeded":
		runSeeded(os.Args[2:])
	case "soak":
		runSoak(os.Args[2:])
	case "replay":
		runReplay(os.Args[2:])
	case "regen":
		runRegen(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: wisp-fuzz <seeded|soak|replay|regen> [flags]")
	os.Exit(2)
}

func requireShells() []testrunner.Shell {
	shells, err := fuzz.RequirePrereqs()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	return shells
}

// findingKind labels a finding as a recompile-INSTABILITY (FR-008 self-oracle) or a
// cross-shell DIVERGENCE (FR-007), so the two distinct bug classes are not conflated
// in operator output.
func findingKind(f fuzz.Finding) string {
	if f.Unstable {
		return "INSTABILITY"
	}
	return "DIVERGENCE"
}

// resultKind is the OracleResult-side counterpart of findingKind (replay/regen work
// with a live OracleResult rather than a stored Finding).
func resultKind(r fuzz.OracleResult) string {
	if r.Unstable {
		return "INSTABILITY"
	}
	return "DIVERGENCE"
}

// printCoverage emits the SC-009 observable witness report: every witnessed builtin
// FAMILY and every witnessed CATEGORY (pure-core + payload), each with a present/
// MISSING marker so a coverage gap is visible, not silently absent.
func printCoverage(cats fuzz.CategorySet, fams fuzz.FamilySet) {
	all := fuzz.AdmittedFamilies()
	var famPresent, famMissing []string
	for _, f := range all {
		if fams[f] {
			famPresent = append(famPresent, f)
		} else {
			famMissing = append(famMissing, f)
		}
	}
	fmt.Printf("coverage: families witnessed (%d/%d): %v\n", len(famPresent), len(all), famPresent)
	if len(famMissing) > 0 {
		fmt.Printf("coverage: families MISSING: %v\n", famMissing)
	}

	var present, missing []string
	for _, c := range fuzz.AllCategories {
		if cats.Has(c) {
			present = append(present, c.String())
		} else {
			missing = append(missing, c.String())
		}
	}
	fmt.Printf("coverage: categories witnessed (%d/%d): %v\n", len(present), len(fuzz.AllCategories), present)
	if len(missing) > 0 {
		fmt.Printf("coverage: categories MISSING: %v\n", missing)
	}
}

func runSeeded(args []string) {
	fs := flag.NewFlagSet("seeded", flag.ExitOnError)
	manifest := fs.String("manifest", "", "alternate manifest for ad-hoc exploration only (NON-CANONICAL: not the frozen baseline; results do not count as the SC-005 clean baseline)")
	seed := fs.Int64("seed", 0, "run a SINGLE ad-hoc (seed,bound) entry instead of the manifest (requires -bound-value; sets the FR-012 direct seeded workflow)")
	boundKind := fs.String("bound-kind", "program_count", "bound kind for -seed: program_count|generation_budget")
	boundValue := fs.Int("bound-value", 0, "bound value for -seed (number of programs)")
	out := fs.String("out", fuzz.DefaultFindingsDir, "directory for persisted findings")
	fs.Parse(args)

	shells := requireShells()
	// Detect explicit flag presence so seed 0 is requestable and "unset" is never
	// conflated with a real value (fs.Visit only reports flags actually set).
	seedSet, boundValueSet := false, false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "seed":
			seedSet = true
		case "bound-value":
			boundValueSet = true
		}
	})
	var entries []fuzz.ManifestEntry
	var err error
	switch {
	case seedSet || boundValueSet:
		// FR-012 direct single-seed workflow: run ONE explicit (seed, bound) entry
		// without authoring a manifest. Deterministic and self-contained; not the
		// SC-005 baseline (that is the canonical manifest below).
		if !boundValueSet || *boundValue <= 0 {
			fmt.Fprintln(os.Stderr, "fatal: -seed requires a positive -bound-value")
			os.Exit(2)
		}
		bk, bkErr := fuzz.ParseBoundKind(*boundKind)
		if bkErr != nil {
			fmt.Fprintln(os.Stderr, "fatal:", bkErr)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "NOTE: single ad-hoc seed run (seed=%d %s=%d) -- not the SC-005 baseline\n", *seed, bk, *boundValue)
		entries = []fuzz.ManifestEntry{{Seed: *seed, BoundKind: bk, BoundValue: *boundValue}}
	case *manifest != "":
		// Exploration escape hatch. It never masquerades as the canonical baseline:
		// the digest-guarded embedded manifest (SC-016) and the SC-005 baseline test
		// use LoadCanonicalManifest and are unaffected by this flag; we banner it so a
		// non-canonical run is never mistaken for the frozen baseline.
		fmt.Fprintf(os.Stderr, "WARNING: NON-CANONICAL manifest %q -- exploration only, not the SC-005 baseline\n", *manifest)
		entries, err = fuzz.LoadManifest(*manifest)
	default:
		entries, err = fuzz.LoadCanonicalManifest() // embedded, digest-authoritative
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	total := 0
	var cats fuzz.CategorySet
	fams := fuzz.FamilySet{}
	for _, e := range entries {
		rep, err := fuzz.RunSeeded(fuzz.RunConfig{Shells: shells, GenCfg: fuzz.DefaultGenConfig(), OutDir: *out}, e)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fatal (seed %d): %v\n", e.Seed, err)
			os.Exit(1)
		}
		total += len(rep.Findings)
		cats |= rep.Categories
		for f := range rep.Families {
			fams[f] = true
		}
		for _, f := range rep.Findings {
			fmt.Printf("%s seed=%d idx=%d: %s\n", findingKind(f), f.Provenance.Seed, f.Provenance.ProgramIndex, f.Detail)
		}
	}
	printCoverage(cats, fams)
	if total > 0 {
		os.Exit(1)
	}
	fmt.Println("clean: no divergences")
}

func runSoak(args []string) {
	fs := flag.NewFlagSet("soak", flag.ExitOnError)
	seed := fs.Int64("seed", 1, "seed for the soak run")
	out := fs.String("out", fuzz.DefaultFindingsDir, "directory for persisted findings")
	fs.Parse(args)
	shells := requireShells()
	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; close(stop) }()
	fmt.Println("soak running; Ctrl-C to stop")
	if _, err := fuzz.RunSoak(fuzz.RunConfig{Shells: shells, GenCfg: fuzz.DefaultGenConfig(), OutDir: *out}, *seed, stop); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	fmt.Println("soak stopped")
}

func runReplay(args []string) {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	// Repo-root-relative: the CLI is invoked from the repo root (go run ./cmd/wisp-fuzz).
	// The fuzzshell tests address the SAME directory as the package-relative
	// "corpus/regressions" because `go test` runs with cwd = the package dir
	// (internal/fuzz). Both paths resolve to internal/fuzz/corpus/regressions; the
	// prefix differs only because the cwd differs. Override -dir for a corpus elsewhere.
	dir := fs.String("dir", "internal/fuzz/corpus/regressions", "curated corpus dir (repo-root-relative)")
	fs.Parse(args)
	shells := requireShells()
	entries, err := fuzz.LoadCorpus(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		// A misconfigured path or empty regression dir must FAIL, not report a
		// false "0 entries pass" clean -- an empty regression set proves nothing
		// and silently green would mask the SC-008 done-signal going stale (F8).
		fmt.Fprintf(os.Stderr, "fatal: no corpus entries under %q (empty/misconfigured regression set)\n", *dir)
		os.Exit(1)
	}
	fail := 0
	for _, f := range entries {
		res, err := fuzz.ReplayEntry(shells, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "replay error (%s): %v\n", f.Provenance.Filename(), err)
			fail++
			continue
		}
		if res.Diverged || res.Unstable {
			fmt.Printf("REGRESSION (%s) seed=%d idx=%d: %s\n", resultKind(res), f.Provenance.Seed, f.Provenance.ProgramIndex, res.Detail)
			fail++
		}
	}
	if fail > 0 {
		os.Exit(1)
	}
	fmt.Printf("corpus clean: %d entries pass\n", len(entries))
}

// runRegen is the FR-017 provenance path: regenerate the exact ORIGINAL pre-shrink
// program from (seed, bound_kind, bound_value, program_index), print its source, and
// run it under the four shells so a recorded failing provenance reproduces its
// pre-shrink divergence (the debugging/traceability path, distinct from `replay`).
func runRegen(args []string) {
	fs := flag.NewFlagSet("regen", flag.ExitOnError)
	seed := fs.Int64("seed", 0, "provenance seed")
	boundKind := fs.String("bound-kind", "program_count", "program_count|generation_budget")
	boundValue := fs.Int("bound-value", 0, "provenance bound value (0 -> derived as index+1 for a partial invocation)")
	index := fs.Int("index", 0, "provenance program_index")
	fs.Parse(args)
	bk, bkErr := fuzz.ParseBoundKind(*boundKind)
	if bkErr != nil {
		fmt.Fprintln(os.Stderr, "fatal:", bkErr)
		os.Exit(2)
	}
	bv := *boundValue
	if bv == 0 {
		// Convenience for a partial debug invocation (just -seed -index): a bound of
		// index+1 addresses exactly this program. A real finding copies all four
		// provenance fields, so the exact bound is preserved when given.
		bv = *index + 1
	}
	prov := fuzz.Provenance{Seed: *seed, BoundKind: bk, BoundValue: bv, ProgramIndex: *index}
	p, err := fuzz.RegenerateProgram(prov, fuzz.DefaultGenConfig())
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(2)
	}
	fmt.Print(fuzz.Print(p))
	shells := requireShells()
	res, err := fuzz.RunOracle(shells, p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	if res.Diverged || res.Unstable {
		fmt.Printf("%s (pre-shrink): %s\n", resultKind(res), res.Detail)
		os.Exit(1)
	}
	fmt.Println("clean: regenerated program does not diverge on the current compiler")
}
