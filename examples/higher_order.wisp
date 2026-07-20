// Higher-order functions and first-class function references.
import "array"

fn dbl(n: int) -> int {
    return n * 2
}

fn is_even(n: int) -> bool {
    return n % 2 == 0
}

fn add(a: int, b: int) -> int {
    return a + b
}

fn main() -> int {
    let xs: int[] = [1, 2, 3, 4, 5]
    let doubled: int[] = array.map(xs, dbl)
    print("doubled first: ${doubled[0]}")
    let evens: int[] = array.filter(xs, is_even)
    print("even count: ${length(evens)}")
    let total: int = array.reduce(xs, 0, add)
    print("sum: ${total}")
    let op: fn(int, int) -> int = add
    print("indirect add: ${op(20, 22)}")
    return 0
}
