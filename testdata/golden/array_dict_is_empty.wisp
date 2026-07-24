import "array"
import "dict"
fn main() -> int {
    let xs: int[] = []
    let ys: int[] = [1, 2]
    let d: {string: int} = {}
    let e: {string: int} = { "a": 1 }
    print(to_string(array.is_empty(xs)))
    print(to_string(array.is_empty(ys)))
    print(to_string(dict.is_empty(d)))
    print(to_string(dict.is_empty(e)))
    return 0
}
