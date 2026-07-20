fn effectful() -> (int, string) {
    print("ran")
    return (1, "x")
}

fn main() -> int {
    let (_, _) = effectful()
    return 0
}
