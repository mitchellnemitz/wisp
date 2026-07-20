package parser

import "testing"

// TestTrailingCommaParams: fn param lists accept a single trailing comma.
func TestTrailingCommaParams(t *testing.T) {
	parseOK(t, "fn f(a: int, b: int,) -> int { return a }\nfn main() -> int { return 0 }")
	parseOK(t, "fn f(a: int,) -> int { return a }\nfn main() -> int { return 0 }")
}

// TestTrailingCommaCallArgs: call arg lists accept a single trailing comma.
func TestTrailingCommaCallArgs(t *testing.T) {
	parseOK(t, "fn add(a: int, b: int) -> int { return a }\nfn main() -> int { return add(1, 2,) }")
	parseOK(t, "fn f(a: int) -> int { return a }\nfn main() -> int { return f(1,) }")
}

// TestTrailingCommaTypeParams: generic fn type-param lists accept a single trailing comma.
func TestTrailingCommaTypeParams(t *testing.T) {
	parseOK(t, "fn id[T,](x: T) -> T { return x }\nfn main() -> int { return 0 }")
	parseOK(t, "fn apply[T, U,](x: T, f: fn(T) -> U) -> U { return f(x) }\nfn main() -> int { return 0 }")
}

// TestTrailingCommaStructTypeParams: struct generic type-param lists accept a single trailing comma.
func TestTrailingCommaStructTypeParams(t *testing.T) {
	parseOK(t, "struct Box[T,] { v: T }\nfn main() -> int { return 0 }")
}

// TestTrailingCommaTypeArgs: generic type-argument lists accept a single trailing comma.
func TestTrailingCommaTypeArgs(t *testing.T) {
	parseOK(t, "struct Box[T] { v: T }\nfn main() -> int { let b: Box[int,] = Box { v: 1 }\n return 0 }")
}

// TestDoubleTrailingCommaErrors: double trailing comma stays a parse error.
func TestDoubleTrailingCommaErrors(t *testing.T) {
	// double trailing comma in params
	parseErr(t, "fn f(a: int,,) -> int { return a }\nfn main() -> int { return 0 }")
	// double trailing comma in call args
	parseErr(t, "fn f(a: int) -> int { return a }\nfn main() -> int { return f(1,,) }")
	// double trailing comma in type params
	parseErr(t, "fn id[T,,](x: T) -> T { return x }\nfn main() -> int { return 0 }")
	// double trailing comma in struct type params
	parseErr(t, "struct Box[T,,] { v: T }\nfn main() -> int { return 0 }")
}

// TestLeadingCommaErrors: leading comma stays a parse error.
func TestLeadingCommaErrors(t *testing.T) {
	parseErr(t, "fn f(a: int) -> int { return a }\nfn main() -> int { return f(,1) }")
	parseErr(t, "struct Box[T] { v: T }\nfn main() -> int { let b: Box[,int] = Box[int] { v: 1 }\n return 0 }")
}

// TestLoneCommaErrors: lone comma stays a parse error.
func TestLoneCommaErrors(t *testing.T) {
	parseErr(t, "fn f(a: int) -> int { return a }\nfn main() -> int { return f(,) }")
	parseErr(t, "struct Box[T] { v: T }\nfn main() -> int { let b: Box[,] = Box[int] { v: 1 }\n return 0 }")
}

// TestTrailingCommaRegressionLiterals: already-working forms still parse.
func TestTrailingCommaRegressionLiterals(t *testing.T) {
	// array literal trailing comma
	parseOK(t, wrap("let xs: int[] = [1, 2,]\nreturn 0"))
	// dict literal trailing comma
	parseOK(t, wrap(`let d: {string: int} = {"a": 1,}`+"\nreturn 0"))
	// struct literal trailing comma
	parseOK(t, "struct P { x: int }\nfn main() -> int { let p: P = P { x: 1, }\n return 0 }")
	// tuple literal trailing comma (2-elem)
	parseOK(t, wrap("let t: (int,int) = (1, 2,)\nreturn 0"))
	// struct decl field list trailing comma
	parseOK(t, "struct P { x: int, }\nfn main() -> int { return 0 }")
}
