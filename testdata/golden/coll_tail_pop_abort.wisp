import "array"
fn main() -> int {
    let empty: int[] = []
    let v: int = array.pop(empty)
    print(to_string(v))
    return 0
}
