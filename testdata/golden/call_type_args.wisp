import "array"
fn identity[T](x: T) -> T {
    return x
}

fn empty_list[T]() -> T[] {
    let xs: T[] = []
    return xs
}

fn add[T: numeric](a: T, b: T) -> T {
    return a + b
}

fn main() -> int {
    let n: int = identity[int](42)
    let xs: int[] = empty_list[int]()
    array.push(xs, n)
    array.push(xs, identity[int](8))
    let f: float = add[float](1.5, 2.0)
    let i: int = add[int](3, 4)
    print("n=${n}")
    print("len=${length(xs)}")
    print("f=${f}")
    print("i=${i}")
    return 0
}
