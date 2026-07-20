fn pair() -> (int, string) {
    return (1, "$(echo HACKED 1>&2)")
}

fn main() -> int {
    let (a: int, b: string) = pair()
    print(b)
    return 0
}
