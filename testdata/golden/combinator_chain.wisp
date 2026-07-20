import "array"
fn double(x: int) -> int { return x * 2 }
fn maybe_double(x: int) -> Optional[int] {
    if (x > 100) { return None }
    return Some(x * 2)
}
fn fallback() -> Optional[int] { return Some(-1) }
fn positive(x: int) -> bool { return x > 0 }
fn main() -> int {
    let start: Optional[int] = Some(10)
    let r: Optional[int] = or_else(array.filter(and_then(array.map(start, double), maybe_double), positive), fallback)
    print(to_string(unwrap(r)))
    let big: Optional[int] = Some(200)
    let r2: Optional[int] = or_else(and_then(array.map(big, double), maybe_double), fallback)
    print(to_string(unwrap(r2)))
    return 0
}
