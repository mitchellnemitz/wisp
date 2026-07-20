fn pair() -> (int, string) {
    return (3, "x")
}

fn main() -> int {
    final (a: int, b: string) = pair()
    print(to_string(a))
    print(b)
    return 0
}
