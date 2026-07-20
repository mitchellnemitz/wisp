fn main() -> int {
    try {
        try {
            throw error("orig")
        } catch (e) {
            throw wrap(error("outer-cause"), "outer-rethrown")
        } finally {
            try {
                throw wrap(error("inner-cause"), "inner-rethrown")
            } catch (e2) {
                print("inner-caught:" + e2.message)
                let oi: Optional[error] = cause(e2)
                match (oi) {
                    case Some(ic) {
                        print("inner-cause:" + ic.message)
                    }
                    case None {
                        print("inner-unexpected-none")
                    }
                }
            }
        }
    } catch (eo) {
        print("outer-msg:" + eo.message)
        let oo: Optional[error] = cause(eo)
        match (oo) {
            case Some(oc) {
                print("outer-cause:" + oc.message)
            }
            case None {
                print("outer-unexpected-none")
            }
        }
    }
    return 0
}
