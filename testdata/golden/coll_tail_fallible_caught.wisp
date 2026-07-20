import "array"
fn fault_cb(x: int) -> bool {
    let empty: int[] = []
    let z: int = array.first(empty)
    return z > 0
}
fn main() -> int {
    let empty: int[] = []
    try {
        let v: int = array.pop(empty)
        print(to_string(v))
    } catch (e) {
        print("pop caught")
    }
    let arr: int[] = [10, 20, 30]
    try {
        array.remove_at(arr, 5)
        print("no")
    } catch (e) {
        print("remove_at caught")
    }
    try {
        array.insert_at(arr, 9, 0)
        print("no")
    } catch (e) {
        print("insert_at caught")
    }
    let nums: int[] = [1, 2]
    try {
        let c: int = array.count_where(nums, fault_cb)
        print(to_string(c))
    } catch (e) {
        print("count_where caught")
    }
    return 0
}
