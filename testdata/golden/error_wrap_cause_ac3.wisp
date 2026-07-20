fn boom() -> void {
    try {
        throw error_with(7, "low")
    } catch (e) {
        throw wrap(wrap(e, "mid"), "context")
    }
}

fn main() -> int {
    try {
        boom()
    } catch (top) {
        print(top.message)
        print(to_string(top.code))
        let o1: Optional[error] = cause(top)
        match (o1) {
            case Some(mid) {
                print(mid.message)
                let o2: Optional[error] = cause(mid)
                match (o2) {
                    case Some(root) {
                        print(root.message)
                        print(to_string(root.code))
                        let o3: Optional[error] = cause(root)
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
    }
    return 0
}
