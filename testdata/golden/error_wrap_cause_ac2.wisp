fn main() -> int {
    let root: error = error("root")
    let mid: error = wrap(root, "mid")
    let top: error = wrap(mid, "top")
    print(top.message)
    let o1: Optional[error] = cause(top)
    match (o1) {
        case Some(mid_got) {
            print(mid_got.message)
            let o2: Optional[error] = cause(mid_got)
            match (o2) {
                case Some(root_got) {
                    print(root_got.message)
                    let o3: Optional[error] = cause(root_got)
                    print(to_string(is_none(o3)))
                }
                case None {
                    print("unexpected-none-at-root")
                }
            }
        }
        case None {
            print("unexpected-none-at-mid")
        }
    }
    return 0
}
