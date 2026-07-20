fn pair() -> (int, string) {
    return (7, "hello")
}

fn main() -> int {
    let (a: int, b: string) = pair()
    print(to_string(a))
    print(b)
    return 0
}
