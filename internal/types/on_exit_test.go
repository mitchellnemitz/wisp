package types

import "testing"

// Checker tests for on_exit (Task 1).

func TestOnExit_OK(t *testing.T) {
	expectOK(t, `fn cleanup() -> void { print("bye") }
fn main() -> int {
on_exit(cleanup)
return 0
}`)
}

func TestOnExit_Arity0_Error(t *testing.T) {
	expectErr(t, `fn main() -> int {
on_exit()
return 0
}`, "on_exit expects 1 argument")
}

func TestOnExit_Arity2_Error(t *testing.T) {
	expectErr(t, `fn cleanup() -> void { print("bye") }
fn cleanup2() -> void { print("bye2") }
fn main() -> int {
on_exit(cleanup, cleanup2)
return 0
}`, "on_exit expects 1 argument")
}

func TestOnExit_WrongArity_Handler(t *testing.T) {
	// fn(int)->void has wrong arity (takes a parameter)
	expectErr(t, `fn cleanup(x: int) -> void { print("${x}") }
fn main() -> int {
on_exit(cleanup)
return 0
}`, "no parameters")
}

func TestOnExit_WrongReturn_Handler(t *testing.T) {
	// fn()->int returns non-void
	expectErr(t, `fn cleanup() -> int { return 42 }
fn main() -> int {
on_exit(cleanup)
return 0
}`, "return void")
}

func TestOnExit_BuiltinHandler_Error(t *testing.T) {
	// on_exit(print) -- passing a builtin as a funcref is already a compile error
	expectErr(t, `fn main() -> int {
on_exit(print)
return 0
}`, "")
}

func TestOnExit_VoidResult_NotAValue(t *testing.T) {
	// let x = on_exit(h) is a type error (Void not a value)
	expectErr(t, `fn cleanup() -> void { print("bye") }
fn main() -> int {
let x: int = on_exit(cleanup)
return 0
}`, "")
}

func TestOnExit_Reserved(t *testing.T) {
	expectErr(t, `fn on_exit() -> int { return 0 }
fn main() -> int { return 0 }`, "reserved builtin")
}
