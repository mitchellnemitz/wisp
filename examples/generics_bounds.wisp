// The `comparable` type-parameter bound: writing `[T: comparable]` lets a
// generic function use `==` / `!=` on a value of type T, where T is inferred to
// be one of int, bool, or string. Everything else (ordered comparison,
// arithmetic, field access, ...) stays barred for a type parameter, and float is
// not comparable. One shell function still serves every instantiation.
fn contains_eq[T: comparable](xs: T[], target: T) -> bool {
    for (x in xs) {
        if (x == target) {
            return true
        }
    }
    return false
}

fn main() -> int {
    let xs: int[] = [3, 7, 9]
    print("has 7: ${contains_eq(xs, 7)}")
    print("has 8: ${contains_eq(xs, 8)}")
    let ws: string[] = ["a", "b", "c"]
    print("has b: " + to_string(contains_eq(ws, "b")))
    return 0
}
