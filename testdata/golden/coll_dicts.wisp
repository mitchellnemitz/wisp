import "array"
import "dict"
import "string"
fn show(x: int) -> string { return to_string(x) }
fn main() -> int {
    let d: {string: int} = { "a": 1, "b": 2, "c": 3 }
    print(string.join(array.map(dict.values(d), show), " "))
    print(to_string(dict.get_or(d, "b", 0)) + " " + to_string(dict.get_or(d, "z", -1)))
    dict.remove(d, "b")
    print(string.join(dict.keys(d), ","))
    let m: {string: int} = dict.merge(d, { "a": 9, "x": 7 })
    print(string.join(dict.keys(m), ",") + " a=" + to_string(m["a"]))
    return 0
}
