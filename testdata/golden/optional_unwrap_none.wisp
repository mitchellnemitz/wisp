fn main() -> int {
    let n: Optional[int] = None
    print(to_string(unwrap(n)))
    return 0
}
