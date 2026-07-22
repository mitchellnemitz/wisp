package types

import "testing"

func TestTaggedEnumIsHandleValueEnumIsNot(t *testing.T) {
	info := checkSrc(t, `enum Expr { IntLit(int), Unit }
enum Color: int { Red, Green }
fn main() -> int { return 0 }`)
	var exprTok, colorTok Type
	for tok, ei := range info.Enums {
		if ei.Name == "Expr" {
			exprTok = Type(tok)
		}
		if ei.Name == "Color" {
			colorTok = Type(tok)
		}
	}
	ck := &checker{info: info}
	if !ck.isHandle(exprTok) {
		t.Errorf("tagged-union enum Expr must be a handle type")
	}
	if ck.isHandle(colorTok) {
		t.Errorf("value enum Color must NOT be a handle type")
	}
}
