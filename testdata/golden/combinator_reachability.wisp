import "array"
fn only_used_as_combinator_arg(x: int) -> int { return x + 100 }
fn main() -> int {
    let o: Optional[int] = Some(5)
    let r: Optional[int] = array.map(o, only_used_as_combinator_arg)
    print(to_string(unwrap(r)))
    return 0
}
