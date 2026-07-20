fn main() -> int {
    let payload: string = "$(echo HACKED 1>&2) `echo BAD` ; rm -rf * end\n"
    let inner: error = error("inner-payload")
    let w: error = wrap(inner, payload)
    print(w.message + "MSGEND")
    let o: Optional[error] = cause(w)
    match (o) {
        case Some(got) {
            print(got.message + "CAUSEEND")
        }
        case None {
            print("unexpected-none")
        }
    }
    return 0
}
