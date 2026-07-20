fn main() -> int {
    let r: Result[string] = Ok("$(echo HACKED 1>&2)")
    print("ok: ${unwrap(r)}")
    return 0
}
