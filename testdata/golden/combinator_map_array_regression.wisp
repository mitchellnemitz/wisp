import "array"
fn triple(x: int) -> int { return x * 3 }
fn main() -> int {
    let xs: int[] = [1, 2, 3]
    let ys: int[] = array.map(xs, triple)
    for (y in ys) { print(to_string(y)) }
    return 0
}
