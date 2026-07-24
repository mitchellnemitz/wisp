fn empty_list[T]() -> T[] {
    let xs: T[] = []
    return xs
}

fn make() -> int[] {
    return empty_list()
}

fn takes_strings(xs: string[]) -> int {
    return length(xs)
}

fn main() -> int {
    let a: int[] = empty_list()
    print(to_string(length(a)))

    let b: int[] = make()
    print(to_string(length(b)))

    let n: int = takes_strings(empty_list())
    print(to_string(n))

    let m: {string: int[]} = {"k": empty_list()}
    print(to_string(length(m["k"])))

    return 0
}
