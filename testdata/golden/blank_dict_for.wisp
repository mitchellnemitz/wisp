fn main() -> int {
    let d: {string: int} = {"a": 1, "b": 2}
    for (_ in d) {
        print("tick")
    }
    return 0
}
