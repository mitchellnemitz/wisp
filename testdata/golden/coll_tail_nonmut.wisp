import "array"
import "string"
fn gt2(x: int) -> bool { return x > 2 }
fn main() -> int {
    let xs: int[] = [3, 1, 4, 1, 5]
    let pos: Optional[int] = string.index_of(xs, 4)
    let absent: Optional[int] = string.index_of(xs, 9)
    print(to_string(unwrap(pos)))
    print(to_string(is_none(absent)))
    print(to_string(array.count_where(xs, gt2)))
    let nested: int[][] = [[1, 2], [], [3, 4]]
    let flat: int[] = array.flatten(nested)
    print(to_string(length(flat)))
    print(to_string(flat[0]) + " " + to_string(flat[3]))
    let dups: int[] = [2, 1, 2, 3, 1]
    let uniq: int[] = array.unique(dups)
    print(to_string(length(uniq)))
    print(to_string(uniq[0]) + " " + to_string(uniq[1]) + " " + to_string(uniq[2]))
    let src: int[] = [10, 20, 30, 40, 50]
    let t3: int[] = array.take(src, 3)
    let d2: int[] = array.drop(src, 2)
    print(to_string(length(t3)) + " " + to_string(t3[2]))
    print(to_string(length(d2)) + " " + to_string(d2[0]))
    return 0
}
