import "dict"
fn main() -> int {
    let d: {string: int} = { "a": 1, "b": 2, "c": 3 }
    print(to_string(dict.size(d)))
    dict.clear(d)
    print(to_string(dict.size(d)))
    print(to_string(length(dict.keys(d))))
    d["x"] = 7
    print(to_string(dict.size(d)))
    return 0
}
