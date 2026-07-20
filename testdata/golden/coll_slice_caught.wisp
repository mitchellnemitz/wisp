import "array"
import "string"
fn show(x: int) -> string { return to_string(x) }
fn main() -> int {
    let xs: int[] = [1, 2]
    try {
        print(string.join(array.map(array.slice(xs, 0, 9), show), " "))
        print("no")
    } catch (e) {
        print("caught out-of-range")
    }
    return 0
}
