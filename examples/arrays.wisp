// Arrays: literals, push, indexing, length, for-in, reverse, membership.
import "array"
import "string"

fn main() -> int {
    let xs: int[] = [3, 1, 2]
    array.push(xs, 5)
    print("count: ${length(xs)}")
    print("first: ${xs[0]}")
    let r: int[] = array.reverse(xs)
    print("reversed first: ${r[0]}")
    let total: int = 0
    for (x in xs) {
        total = total + x
    }
    print("sum: ${total}")
    print("has 2: ${to_string(string.contains(xs, 2))}")
    return 0
}
