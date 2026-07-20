package types

import "testing"

// Runtime expression context (not const): an int literal exceeding int64 range
// must be rejected, matching the const path's "integer literal out of range".

func TestIntLit_Runtime_OverRange_Rejected(t *testing.T) {
	expectErr(t, wrapMain(`let x: int = 99999999999999999999999`), "integer literal out of range")
}

func TestIntLit_Runtime_IntMax_Accepted(t *testing.T) {
	expectOK(t, wrapMain(`let x: int = 9223372036854775807`))
}

func TestIntLit_Runtime_OnePastMax_Rejected(t *testing.T) {
	expectErr(t, wrapMain(`let x: int = 9223372036854775808`), "integer literal out of range")
}

func TestIntLit_Runtime_IntMin_Accepted(t *testing.T) {
	// INT_MIN reaches the checker as unary minus over the unsigned magnitude.
	expectOK(t, wrapMain(`let x: int = -9223372036854775808`))
}

func TestIntLit_Runtime_OnePastMin_Rejected(t *testing.T) {
	expectErr(t, wrapMain(`let x: int = -9223372036854775809`), "integer literal out of range")
}
