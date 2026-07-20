import "array"
fn identity(s: string) -> string { return s }
fn main() -> int {
    let payload: string = "$(echo HACKED 1>&2)"
    let o: Optional[string] = Some(payload)
    let r: Optional[string] = array.map(o, identity)
    print(unwrap(r))
    return 0
}
