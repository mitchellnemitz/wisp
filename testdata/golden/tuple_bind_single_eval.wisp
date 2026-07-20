fn effectful() -> (int, string) {
    print("evaluated")
    return (5, "v")
}

fn main() -> int {
    let (a: int, b: string) = effectful()
    print(to_string(a))
    print(b)
    return 0
}
