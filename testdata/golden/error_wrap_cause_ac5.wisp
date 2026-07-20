fn add_context(e: error) -> error { return wrap(e, "ctx") }
fn recover_with(e: error) -> Result[int] { return Err(wrap(e, "recovered")) }
fn main() -> int {
    let orig: error = error("orig")
    let err: Result[int] = Err(orig)
    let r1: Result[int] = map_err(err, add_context)
    let wrapped: error = unwrap_err(r1)
    print(wrapped.message)
    print(to_string(wrapped.code))
    let o: Optional[error] = cause(wrapped)
    match (o) {
        case Some(inner) {
            print(inner.message)
        }
        case None {
            print("unexpected-none")
        }
    }
    let pre_wrapped: error = wrap(orig, "pre")
    let err2: Result[int] = Err(pre_wrapped)
    let r2: Result[int] = map_err(err2, add_context)
    let double: error = unwrap_err(r2)
    print(double.message)
    let o2: Optional[error] = cause(double)
    match (o2) {
        case Some(middle) {
            print(middle.message)
            let o3: Optional[error] = cause(middle)
            match (o3) {
                case Some(innermost) {
                    print(innermost.message)
                }
                case None {
                    print("unexpected-none-inner")
                }
            }
        }
        case None {
            print("unexpected-none-double")
        }
    }
    let ok: Result[int] = Ok(42)
    let r3: Result[int] = or_else(ok, recover_with)
    print(to_string(unwrap(r3)))
    let err3: Result[int] = Err(pre_wrapped)
    let r4: Result[int] = or_else(err3, recover_with)
    let preserved_via_or_else: error = unwrap_err(r4)
    print(preserved_via_or_else.message)
    let o4: Optional[error] = cause(preserved_via_or_else)
    match (o4) {
        case Some(chain_inner) {
            print(chain_inner.message)
        }
        case None {
            print("unexpected-none-or-else")
        }
    }
    return 0
}
