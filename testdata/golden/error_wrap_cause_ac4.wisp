fn main() -> int {
    try {
        throw wrap(error("inner"), "outer")
    } catch (e) {
        print(e.message)
        let o: Optional[error] = cause(e)
        print(to_string(is_none(o)))
    }
    let a: int = 1
    let b: int = 0
    try {
        print(to_string(a / b))
    } catch (f) {
        let o2: Optional[error] = cause(f)
        print(to_string(is_none(o2)))
    }
    return 0
}
