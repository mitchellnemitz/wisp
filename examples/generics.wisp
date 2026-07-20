// Generic functions: a function may declare type parameters in [brackets] and
// use them in its signature and body. The compiler infers the type arguments at
// each call site, so one shell function serves every instantiation.
fn identity[T](x: T) -> T {
    return x
}

fn first_of[T](xs: T[]) -> Optional[T] {
    if (length(xs) > 0) {
        return Some(xs[0])
    }
    return None
}

fn main() -> int {
    let ints: int[] = [10, 20, 30]
    let strs: string[] = ["a", "b"]
    let fi: Optional[int] = first_of(ints)
    let fs: Optional[string] = first_of(strs)
    print("first int: ${unwrap(fi)}")
    print("first str: ${unwrap(fs)}")
    print("identity: ${identity(42)}")
    let empty: int[] = []
    let none: Optional[int] = first_of(empty)
    if (is_none(none)) {
        print("empty has none")
    }
    return 0
}
