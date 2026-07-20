// A small arithmetic library used by the example tests.
export fn add(a: int, b: int) -> int {
    return a + b
}

export fn subtract(a: int, b: int) -> int {
    return a - b
}

export fn multiply(a: int, b: int) -> int {
    return a * b
}

// divide returns None when the divisor is zero.
export fn divide(a: int, b: int) -> Optional[int] {
    if (b == 0) {
        return None
    }
    return Some(a / b)
}
