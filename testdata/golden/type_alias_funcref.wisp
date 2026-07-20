type BinOp = fn(int, int) -> int

fn add(a: int, b: int) -> int {
    return a + b
}

fn apply(f: BinOp, a: int, b: int) -> int {
    return f(a, b)
}

fn main() -> int {
    let f: BinOp = add
    print("apply=${apply(f, 6, 7)}")
    print("direct=${f(3, 4)}")
    return 0
}
