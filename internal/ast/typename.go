package ast

import "strings"

// CanonicalType renders a type annotation (TypeName, a structural encoding)
// in canonical form: one space after ':' in a dict type and after ',' in a
// function-type parameter list; no inner brackets spacing otherwise. The result
// is re-parseable to the same TypeName.
func CanonicalType(t TypeName) string {
	s := string(t)
	switch {
	case strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"):
		// Array type: emit the element then postfix `[]`. A funcref element must
		// be parenthesized so the trailing `[]` binds to the whole funcref and
		// does not reparse as a funcref returning an array (round-trip guard).
		elem := s[1 : len(s)-1]
		inner := CanonicalType(TypeName(elem))
		if isFuncrefAnn(elem) {
			inner = "(" + inner + ")"
		}
		return inner + "[]"
	case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && strings.Contains(s, ":"):
		inner := s[1 : len(s)-1]
		i := strings.IndexByte(inner, ':')
		return "{" + CanonicalType(TypeName(inner[:i])) + ": " + CanonicalType(TypeName(inner[i+1:])) + "}"
	case isFuncrefAnn(s):
		params, ret := splitFuncTypeAnn(s)
		var parts []string
		for _, pa := range params {
			parts = append(parts, CanonicalType(TypeName(pa)))
		}
		return "fn(" + strings.Join(parts, ", ") + ") -> " + CanonicalType(TypeName(ret))
	case strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && !strings.HasPrefix(s, "fn("):
		inner := s[1 : len(s)-1]
		// Re-format each element type recursively. splitTopLevelCommas does
		// not split inside nested (), [], or {}.
		elems := splitTopLevelCommas(inner)
		parts := make([]string, len(elems))
		for i, e := range elems {
			parts[i] = CanonicalType(TypeName(e))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case strings.Contains(s, "[") && !strings.HasPrefix(s, "["):
		// Generic named type: Name[T1,T2,...] -> Name[T1, T2, ...]
		bi := strings.IndexByte(s, '[')
		base := s[:bi]
		inner := s[bi+1 : len(s)-1]
		elems := splitTopLevelCommas(inner)
		parts := make([]string, len(elems))
		for i, e := range elems {
			parts[i] = CanonicalType(TypeName(e))
		}
		return base + "[" + strings.Join(parts, ", ") + "]"
	default:
		return s
	}
}

// isFuncrefAnn reports whether s is a funcref type annotation encoding, i.e.
// "fn(...)->R". Both the array-element parenthesization guard and the funcref
// dispatch branch in CanonicalType share this shape test so the "is this a
// funcref encoding" predicate lives in one place.
func isFuncrefAnn(s string) bool {
	return strings.HasPrefix(s, "fn(") && strings.Contains(s, ")->")
}

// splitFuncTypeAnn decomposes "fn(P1,P2,...)->R" into parameter annotation
// strings and the return annotation, splitting on top-level commas (commas
// inside nested composites are skipped via bracket-depth tracking).
func splitFuncTypeAnn(s string) (params []string, ret string) {
	// strip leading "fn(" and split at the matching ")->".
	body := s[len("fn("):]
	// find the ")->" that closes the parameter list at depth 0.
	depth := 0
	close := -1
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				close = i
				break
			}
			depth--
		}
		if close >= 0 {
			break
		}
	}
	if close < 0 {
		return nil, ""
	}
	paramStr := body[:close]
	ret = body[close+len(")->"):]
	if paramStr != "" {
		params = splitTopLevelCommas(paramStr)
	}
	return params, ret
}

// splitTopLevelCommas splits s on commas that are not nested inside (), [], {},
// or angle-free funcref parens.
func splitTopLevelCommas(s string) []string {
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}
