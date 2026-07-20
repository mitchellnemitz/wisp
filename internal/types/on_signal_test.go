package types

import "testing"

// Checker tests for on_signal (Task 2).

func TestOnSignal_OK(t *testing.T) {
	expectOK(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("INT", handler)
return 0
}`)
}

func TestOnSignal_OK_DoubleQuoted(t *testing.T) {
	// A double-quoted constant string with no interpolation is a single text part.
	expectOK(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("TERM", handler)
return 0
}`)
}

func TestOnSignal_AllAllowed(t *testing.T) {
	for _, sig := range []string{"INT", "TERM", "HUP", "QUIT", "USR1", "USR2"} {
		expectOK(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("`+sig+`", handler)
return 0
}`)
	}
}

func TestOnSignal_NonLiteralSig_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
let s: string = "INT"
on_signal(s, handler)
return 0
}`, "signal name must be a string literal")
}

func TestOnSignal_InterpolatedSig_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
let n: string = "T"
on_signal("${n}ERM", handler)
return 0
}`, "signal name must be a string literal")
}

func TestOnSignal_UnsupportedEXIT_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("EXIT", handler)
return 0
}`, "unsupported signal: EXIT")
}

func TestOnSignal_UnsupportedKILL_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("KILL", handler)
return 0
}`, "unsupported signal: KILL")
}

func TestOnSignal_UnsupportedBOGUS_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("BOGUS", handler)
return 0
}`, "unsupported signal: BOGUS")
}

func TestOnSignal_UnsupportedEmpty_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("", handler)
return 0
}`, "unsupported signal:")
}

func TestOnSignal_Arity1_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("INT")
return 0
}`, "on_signal expects 2 arguments")
}

func TestOnSignal_Arity3_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal("INT", handler, handler)
return 0
}`, "on_signal expects 2 arguments")
}

func TestOnSignal_Arg0NotString_Error(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
on_signal(42, handler)
return 0
}`, "signal name must be a string literal")
}

func TestOnSignal_BuiltinHandler_Error(t *testing.T) {
	// on_signal("INT", print) -- passing a builtin as a funcref is already a compile error.
	expectErr(t, `fn main() -> int {
on_signal("INT", print)
return 0
}`, "")
}

func TestOnSignal_WrongArity_Handler(t *testing.T) {
	expectErr(t, `fn handler(x: int) -> void { print("${x}") }
fn main() -> int {
on_signal("INT", handler)
return 0
}`, "no parameters")
}

func TestOnSignal_WrongReturn_Handler(t *testing.T) {
	expectErr(t, `fn handler() -> int { return 42 }
fn main() -> int {
on_signal("INT", handler)
return 0
}`, "return void")
}

func TestOnSignal_VoidResult_NotAValue(t *testing.T) {
	expectErr(t, `fn handler() -> void { print("caught") }
fn main() -> int {
let x: int = on_signal("INT", handler)
return 0
}`, "")
}

func TestOnSignal_Reserved(t *testing.T) {
	expectErr(t, `fn on_signal() -> int { return 0 }
fn main() -> int { return 0 }`, "reserved builtin")
}
