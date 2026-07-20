import "array"
import "string"
fn show(x: int) -> string { return to_string(x) }
fn main() -> int {
    let arr: int[] = [10, 20, 30, 40, 50]
    let v: int = array.pop(arr)
    print(to_string(v))
    print(to_string(length(arr)))
    array.remove_at(arr, 1)
    print(string.join(array.map(arr, show), " "))
    array.insert_at(arr, 1, 99)
    print(string.join(array.map(arr, show), " "))
    array.insert_at(arr, length(arr), 77)
    print(string.join(array.map(arr, show), " "))
    return 0
}
