fn f(_: int, _: string, x: int) -> int {
    return x
}
fn main() -> int {
    print(to_string(f(1, "a", 99)))
    return 0
}
