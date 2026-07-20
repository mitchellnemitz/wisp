import "array"
import "string"
fn show(x: int) -> string { return to_string(x) }
fn big(x: int) -> bool { return x > 3 }
fn main() -> int {
    let xs: int[] = [3, 1, 4, 1, 5]
    print(string.join(array.map(array.sort(xs), show), " "))
    print(to_string(unwrap_or(array.find(xs, big), -1)))
    print(string.join(array.map(array.slice(xs, 1, 4), show), " "))
    print(string.join(array.map(array.concat(xs, [9]), show), " "))
    print(to_string(array.sum(xs)))
    print(string.join(array.map(array.range(4), show), " "))
    print(to_string(array.first(xs)) + "/" + to_string(array.last(xs)))
    return 0
}
