fn main() -> int {
    let e: Result[int] = Err(error("bad"))
    try {
        let v: int = unwrap(e)
        print("v ${to_string(v)}")
    } catch (x) {
        print("caught")
    }
    return 0
}
