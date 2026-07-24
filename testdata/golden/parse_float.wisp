fn main() -> int {
    let a: Optional[float] = parse_float("3.14")
    let b: Optional[float] = parse_float("x")
    let c: Optional[float] = parse_float(".5")
    let d: Optional[float] = parse_float("5.")
    let e: Optional[float] = parse_float("-2.0")
    print(to_string(unwrap_or(a, -1.0)))
    print(to_string(is_none(b)))
    print(to_string(is_none(c)))
    print(to_string(is_none(d)))
    print(to_string(unwrap_or(e, -1.0)))
    return 0
}
