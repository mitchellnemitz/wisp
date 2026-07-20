fn main() -> int {
    let t: ((int, bool), string) = ((3, true), "x")
    print(to_string(t[0][0]))
    print(to_string(t[0][1]))
    print(t[1])
    return 0
}
