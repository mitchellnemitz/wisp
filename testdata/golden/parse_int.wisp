fn main() -> int {
    let a: Optional[int] = parse_int("42")
    let b: Optional[int] = parse_int("-9223372036854775808")
    let c: Optional[int] = parse_int("abc")
    let d: Optional[int] = parse_int("007")
    print(to_string(unwrap_or(a, 0)))
    print(to_string(unwrap_or(b, 0)))
    print(to_string(is_none(c)))
    print(to_string(unwrap_or(d, 0)))
    return 0
}
