import "array"
import "dict"

fn even(x: int) -> bool {
    return x % 2 == 0
}

fn main() -> int {
    let a: Optional[int] = Some(42)
    if (is_some(a)) {
        print("some: ${to_string(unwrap(a))}")
    }
    let n: Optional[int] = None
    print("none is_none: ${to_string(is_none(n))}")
    print("or: ${to_string(unwrap_or(n, -1))}")
    let xs: int[] = [1, 3, 4, 7]
    print("first even at: ${to_string(unwrap_or(array.find(xs, even), -1))}")
    let d: {string: int} = { "x": 10 }
    print("get x: ${to_string(unwrap_or(dict.get(d, "x"), 0))}")
    print("get y: ${to_string(unwrap_or(dict.get(d, "y"), 0))}")
    return 0
}
