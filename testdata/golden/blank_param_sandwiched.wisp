fn f(a: int, _: int, c: int) -> int {
    return a * 100 + c
}
fn main() -> int {
    print(to_string(f(1, 2, 3)))
    return 0
}
