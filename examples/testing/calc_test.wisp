// Tests for the calc library. Run with: wisp test examples/testing/
//
// This file has no fn main -- test files use the test construct instead.
// Each test block is a named, isolated unit; the runner executes all of
// them and reports pass/fail/skip per shell.
include "./calc.wisp" as calc

test ("add: basic arithmetic") {
    assert_eq(calc.add(2, 3), 5)
    assert_eq(calc.add(0, 0), 0)
    assert_eq(calc.add(-1, 1), 0)
}

test ("subtract: basic arithmetic") {
    assert_eq(calc.subtract(10, 4), 6)
    assert_eq(calc.subtract(0, 5), -5)
}

test ("multiply: basic arithmetic") {
    assert_eq(calc.multiply(3, 4), 12)
    assert_eq(calc.multiply(-2, 5), -10)
    assert_eq(calc.multiply(0, 100), 0)
}

test ("divide: nonzero divisor returns Some") {
    let result: Optional[int] = calc.divide(10, 2)
    assert_some(result)
    assert_eq(unwrap(result), 5)
}

test ("divide: zero divisor returns None") {
    assert_none(calc.divide(7, 0))
}

test ("assert_contains: substring and membership") {
    assert_contains("hello, world", "world")
    let xs: int[] = [1, 2, 3]
    assert_contains(xs, 2)
}
