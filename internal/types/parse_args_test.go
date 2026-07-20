package types

import "testing"

// TestParseArgsTypeChecks: parse_args(string[], string[]) ->
// ({string:string}, string[], string[]).
func TestParseArgsTypeChecks(t *testing.T) {
	// The result binds to a matching annotated tuple variable.
	expectOK(t, wrapMain(
		"let args: string[] = [\"--name\", \"ada\"]\n"+
			"let vf: string[] = [\"--name\"]\n"+
			"let r: ({string:string}, string[], string[]) = parse_args(args, vf)"))
}

// TestParseArgsDestructures: a `let (values, switches, pos) = parse_args(...)`
// destructures, with slot 0 typed {string:string} (proving splitTopLevel
// balances the nested braces) and slots 1/2 typed string[].
func TestParseArgsDestructures(t *testing.T) {
	expectOK(t, wrapMain(
		"let args: string[] = [\"--name\", \"ada\", \"f1\"]\n"+
			"let vf: string[] = [\"--name\"]\n"+
			"let (vals: {string:string}, sws: string[], pos: string[]) = parse_args(args, vf)\n"+
			"let nsw: int = length(sws)\n"+
			"let files: string[] = pos"))
}

// TestParseArgsSlot0IsDict: slot 0 must resolve to the full {string:string} dict
// type; declaring it as anything else is a slot-located mismatch.
func TestParseArgsSlot0IsDict(t *testing.T) {
	src := wrapMain(
		"let args: string[] = [\"a\"]\n" +
			"let vf: string[] = [\"--n\"]\n" +
			"let (vals: string[], sws: string[], pos: string[]) = parse_args(args, vf)")
	expectErr(t, src, "")
}

// TestParseArgsWrongArity: one or three args is a positioned error.
func TestParseArgsWrongArity(t *testing.T) {
	expectErr(t, wrapMain("let a: string[] = [\"x\"]\nlet r: ({string:string},string[],string[]) = parse_args(a)"), "argument")
	expectErr(t, wrapMain("let a: string[] = [\"x\"]\nlet r: ({string:string},string[],string[]) = parse_args(a, a, a)"), "argument")
}

// TestParseArgsWrongTypes: non-string[] arguments are an error.
func TestParseArgsWrongTypes(t *testing.T) {
	expectErr(t, wrapMain("let r: ({string:string},string[],string[]) = parse_args(1, [\"x\"])"), "string")
	expectErr(t, wrapMain("let a: string[] = [\"x\"]\nlet r: ({string:string},string[],string[]) = parse_args(a, [1])"), "string")
	// an array of non-string is rejected.
	expectErr(t, wrapMain("let a: int[] = [1]\nlet b: string[] = [\"x\"]\nlet r: ({string:string},string[],string[]) = parse_args(a, b)"), "string")
}

// TestParseArgsReserved: parse_args is a reserved builtin name -- a user may not
// define it.
func TestParseArgsReserved(t *testing.T) {
	expectErr(t, "fn parse_args() -> int { return 0 }\nfn main() -> int { return 0 }", "")
}
