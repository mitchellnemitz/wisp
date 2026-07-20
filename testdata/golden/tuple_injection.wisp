fn main() -> int {
    let t: (int, string) = (1, "$(echo HACKED 1>&2)")
    print(t[1])
    return 0
}
