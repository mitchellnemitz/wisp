fn main() -> int {
    try {
        throw wrap(error("inner"), "outer")
    } catch (e) {
        let o: Optional[error] = cause(e)
        print(to_string(is_none(o)))
    }
    try {
        throw error("plain")
    } catch (f) {
        print(f.message)
        let o2: Optional[error] = cause(f)
        print(to_string(is_none(o2)))
    }
    return 0
}
