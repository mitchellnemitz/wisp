// Collections-tail standard library: index_of, count_where, flatten,
// unique, take, drop, pop, remove_at, insert_at, size, clear.
import "array"
import "dict"
import "string"

fn gt2(x: int) -> bool {
    return x > 2
}

fn main() -> int {
    // index_of
    let xs: int[] = [10, 20, 30, 20, 10]
    let pos: Optional[int] = string.index_of(xs, 20)
    let absent: Optional[int] = string.index_of(xs, 99)
    print("index 20: " + to_string(unwrap(pos)))
    print("index 99 none: " + to_string(is_none(absent)))
    // count_where
    let nums: int[] = [1, 2, 3, 4, 5]
    print("count >2: " + to_string(array.count_where(nums, gt2)))
    // flatten
    let nested: int[][] = [[1, 2], [], [3, 4, 5]]
    let flat: int[] = array.flatten(nested)
    print("flat len: " + to_string(length(flat)))
    print("flat[2]: " + to_string(flat[2]))
    // unique
    let dups: int[] = [3, 1, 2, 1, 3, 2]
    let uniq: int[] = array.unique(dups)
    print("unique len: " + to_string(length(uniq)))
    print("unique[0]: " + to_string(uniq[0]))
    // take / drop
    let src: int[] = [1, 2, 3, 4, 5]
    let t3: int[] = array.take(src, 3)
    let d2: int[] = array.drop(src, 2)
    print("take 3 len: " + to_string(length(t3)))
    print("drop 2 len: " + to_string(length(d2)))
    print("drop 2 [0]: " + to_string(d2[0]))
    // pop / remove_at / insert_at
    let arr: int[] = [10, 20, 30, 40]
    let v: int = array.pop(arr)
    print("popped: " + to_string(v))
    print("after pop len: " + to_string(length(arr)))
    array.remove_at(arr, 0)
    print("after remove_at[0]: " + to_string(arr[0]))
    array.insert_at(arr, 0, 99)
    print("after insert_at[0]=99: " + to_string(arr[0]))
    print("after insert_at len: " + to_string(length(arr)))
    // dict size / clear
    let d: {string: int} = { "a": 1, "b": 2, "c": 3 }
    print("size: " + to_string(dict.size(d)))
    dict.clear(d)
    print("after clear size: " + to_string(dict.size(d)))
    d["x"] = 7
    print("after re-add size: " + to_string(dict.size(d)))
    return 0
}
