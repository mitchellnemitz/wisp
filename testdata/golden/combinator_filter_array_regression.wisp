import "array"
fn even(x: int) -> bool { return x == (x / 2) * 2 }
fn main() -> int {
    let xs: int[] = [1, 2, 3, 4, 5]
    let evens: int[] = array.filter(xs, even)
    for (e in evens) { print(to_string(e)) }
    return 0
}
