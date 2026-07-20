fn main() -> int {
    let inner: error = error("inner")
    let w: error = wrap(inner, "outer")
    print(w.message)
    print(to_string(w.code))
    let o: Optional[error] = cause(w)
    match (o) {
        case Some(got) {
            print("some")
            print(got.message)
        }
        case None {
            print("none")
        }
    }
    let o2: Optional[error] = cause(error("x"))
    print(to_string(is_none(o2)))
    let o3: Optional[error] = cause(error_with(7, "x"))
    print(to_string(is_none(o3)))
    return 0
}
