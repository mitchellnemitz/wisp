package types

import "testing"

func TestBitwise_AC2(t *testing.T) {
	// happy path: int operands -> int result
	expectOK(t, `fn main()->int{ let a:int=1&2; let b:int=5|2; let c:int=6^3; let d:int=1<<4; let e:int=256>>2; print("${a}"); return 0 }`)

	// float operand
	expectErr(t, `fn main()->int{ let x:int=1.0 & 2; return 0 }`, "int")
	expectErr(t, `fn main()->int{ let x:int=1 << 2.0; return 0 }`, "int")

	// string operand
	expectErr(t, `fn main()->int{ let x:int="x" & 1; return 0 }`, "int")

	// bool operand WITH the &&/|| typo hint
	expectErr(t, `fn main()->int{ let x:int=true & false; return 0 }`, "&&")
	expectErr(t, `fn main()->int{ let x:int=true | false; return 0 }`, "||")

	// result is int, not bool -> assigning to bool let mismatches
	expectErr(t, `fn main()->int{ let y:bool=1 & 2; return 0 }`, "bool")

	// type-var operand rejected; recovery yields int (no error cascade)
	expectErr(t, `fn f[T](a: T, b: T) -> int { return a & b } fn main()->int{ return 0 }`, "type parameter")
}
