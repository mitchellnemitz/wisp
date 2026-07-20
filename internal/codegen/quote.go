package codegen

import "strings"

// shellSingleQuote re-encodes an arbitrary byte string as a safe POSIX shell
// single-quoted literal (spec section 9.6 invariant 1). Inside single quotes
// every byte is literal except the single quote itself, which is emitted by
// closing the quote, writing an escaped quote, and reopening: ' -> '\”.
//
// The source bytes are never pasted between double quotes and never used as a
// printf format, so a literal containing ' " $ ` \ % or a newline cannot
// terminate or escape the emitted quoting or be reinterpreted by the shell.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellDoubleQuote re-encodes an arbitrary byte string as a safe POSIX shell
// DOUBLE-quoted literal (spec section 9.6 invariant 1, N1). Inside double quotes
// only ` $ \ and " retain meaning; each is backslash-escaped so the literal is
// inert -- a path containing a metacharacter is never re-evaluated, expanded, or
// globbed. Used for the coverage hit-marker's <file> component, which the spec
// requires to be emitted double-quoted.
func shellDoubleQuote(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"`", "\\`",
		`$`, `\$`,
	)
	return `"` + r.Replace(s) + `"`
}
