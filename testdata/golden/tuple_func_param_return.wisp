fn pair(a: int, b: string) -> (int, string) {
    return (a, b)
}

fn get_int(t: (int, string)) -> int {
    return t[0]
}

fn get_str(t: (int, string)) -> string {
    return t[1]
}

fn main() -> int {
    let r: (int, string) = pair(42, "hi")
    print(to_string(get_int(r)))
    print(get_str(r))
    return 0
}
