import "array"
import "dict"
fn even(x: int) -> bool { return x % 2 == 0 }
fn main() -> int {
    let xs: int[] = [1, 3, 4, 7]
    let hit: Optional[int] = array.find(xs, even)
    if (is_some(hit)) { print("hit: ${to_string(unwrap(hit))}") }
    let ys: int[] = [1, 3, 5]
    let miss: Optional[int] = array.find(ys, even)
    if (is_none(miss)) { print("miss") }
    let d: {string: int} = {"x": 10}
    let px: Optional[int] = dict.get(d, "x")
    if (is_some(px)) { print("present: ${to_string(unwrap(px))}") }
    let py: Optional[int] = dict.get(d, "y")
    if (is_none(py)) { print("absent") }
    print(to_string(length(unwrap(Some(xs)))))
    print(to_string(unwrap(unwrap(Some(Some(7))))))
    return 0
}
