import "array"
fn main() -> int {
    let a: int[] = [1, 2, 3]
    let b: string[] = ["a", "b"]
    let z: (int, string)[] = array.zip(a, b)
    print(to_string(length(z)))
    print(to_string(z[0][0]))
    print(z[0][1])
    print(to_string(z[1][0]))
    print(z[1][1])
    return 0
}
