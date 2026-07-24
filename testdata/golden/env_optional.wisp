import "env"
fn main() -> int {
    env.set("WISP_SET", "hello")
    env.set("WISP_EMPTY", "")
    let a: Optional[string] = env.get("WISP_SET")
    let b: Optional[string] = env.get("WISP_EMPTY")
    let c: Optional[string] = env.get("WISP_DEFINITELY_UNSET_XZ")
    print(unwrap_or(a, "MISSING"))
    print(to_string(is_some(b)))
    print(unwrap_or(b, "MISSING"))
    print(to_string(is_none(c)))
    return 0
}
