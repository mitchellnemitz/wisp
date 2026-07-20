fn main() -> int {
    let n: Optional[int] = None
    try {
        print(to_string(unwrap(n)))
    } catch (e) {
        print("caught")
    }
    return 0
}
