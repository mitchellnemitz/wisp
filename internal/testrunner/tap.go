package testrunner

import (
	"strings"
)

// TAPResult holds the parsed result for a single test case from TAP-13 output.
type TAPResult struct {
	Number     int
	Name       string
	OK         bool // true for "ok", false for "not ok"
	Skip       bool // true when the line contains "# SKIP"
	SkipReason string
	Diag       string // captured diagnostic lines following a "not ok"
}

// TAPSuite holds the parsed TAP-13 output for one runner execution.
type TAPSuite struct {
	Plan         int
	PlanExplicit bool // true iff the TAP output contained a "1..N" plan line
	Results      []TAPResult
}

// Complete reports whether the TAP output is well-formed: the plan line was
// present and the number of parsed results matches the declared plan count.
func (s TAPSuite) Complete() bool {
	return s.PlanExplicit && len(s.Results) == s.Plan
}

// splitTAPDirective splits a TAP result description from its directive at the
// first UNESCAPED `#`. A `#` preceded by a backslash is escaped (it is a literal
// `#` inside the description, as the codegen emits for a test name containing
// `#`) and does not start a directive. Returns the raw (still-escaped)
// description and the directive text after the `#` (empty if none).
func splitTAPDirective(s string) (desc, directive string) {
	for i := 0; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}
		// Count the run of backslashes immediately before this `#`. An odd count
		// means the `#` is escaped.
		bs := 0
		for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
			bs++
		}
		if bs%2 == 0 {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// unescapeTAPDesc reverses escapeTAPDesc (codegen): `\#` -> `#`, `\\` -> `\`.
// It scans left to right so a `\\` is consumed as one escaped backslash and its
// trailing characters are not re-interpreted.
func unescapeTAPDesc(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '#' || next == '\\' {
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// ParseTAP parses TAP-13 output. It is intentionally lenient: extra lines are
// ignored. A missing or malformed plan is treated as plan = len(results).
func ParseTAP(output string) TAPSuite {
	var suite TAPSuite
	lines := strings.Split(output, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Plan line: "1..N"
		if strings.HasPrefix(line, "1..") {
			n := 0
			for _, c := range line[3:] {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			suite.Plan = n
			suite.PlanExplicit = true
			continue
		}

		// TAP version line: skip
		if strings.HasPrefix(line, "TAP version") {
			continue
		}

		// Result line: "ok N - name" or "not ok N - name"
		var ok bool
		rest := line
		if strings.HasPrefix(line, "not ok ") {
			ok = false
			rest = line[7:]
		} else if strings.HasPrefix(line, "ok ") {
			ok = true
			rest = line[3:]
		} else {
			continue
		}

		// rest = "N - name [# SKIP reason]"
		// Parse the test number.
		num := 0
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			num = num*10 + int(rest[j]-'0')
			j++
		}
		// Skip " - "
		name := ""
		if j < len(rest) && rest[j] == ' ' {
			j++ // space
		}
		if j+1 < len(rest) && rest[j] == '-' && rest[j+1] == ' ' {
			j += 2
		}
		name = rest[j:]

		// Split the description from any TAP directive at the FIRST UNESCAPED
		// `#`. The codegen escapes a `#` inside a test name as `\#`, so an
		// escaped `#` is part of the name and must not start a directive. Only a
		// real directive line (emitted unescaped by the runner) is recognized.
		desc, directive := splitTAPDirective(name)

		// Detect SKIP in the directive segment only.
		skip := false
		skipReason := ""
		if directive != "" {
			d := strings.TrimSpace(directive)
			if d == "SKIP" {
				skip = true
			} else if rest, ok := strings.CutPrefix(d, "SKIP "); ok {
				skip = true
				skipReason = rest
			}
		}

		r := TAPResult{
			Number:     num,
			Name:       strings.TrimSpace(unescapeTAPDesc(desc)),
			OK:         ok,
			Skip:       skip,
			SkipReason: skipReason,
		}

		// Collect diagnostic lines that immediately follow a "not ok" result.
		if !ok {
			var diagLines []string
			for i+1 < len(lines) {
				next := lines[i+1]
				if strings.HasPrefix(next, "#") || strings.HasPrefix(next, "  ") {
					diagLines = append(diagLines, next)
					i++
				} else {
					break
				}
			}
			r.Diag = strings.Join(diagLines, "\n")
		}

		suite.Results = append(suite.Results, r)
	}

	if !suite.PlanExplicit {
		suite.Plan = len(suite.Results)
	}
	return suite
}
