fn main() -> int {
    let a: Optional[bool] = parse_bool("true")
    let b: Optional[bool] = parse_bool("false")
    let c: Optional[bool] = parse_bool("yes")
    let d: Optional[bool] = parse_bool("1")
    let e: Optional[bool] = parse_bool("")
    let f: Optional[bool] = parse_bool("True")
    print(to_string(unwrap_or(a, false)))
    print(to_string(unwrap_or(b, true)))
    print(to_string(is_none(c)))
    print(to_string(is_none(d)))
    print(to_string(is_none(e)))
    print(to_string(is_none(f)))
    return 0
}
