fn pair() -> (int, string) {
    return (1, "kept")
}

fn main() -> int {
    let (_, out: string) = pair()
    print(out)
    return 0
}
