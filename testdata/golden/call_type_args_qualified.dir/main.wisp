include "./lib/box.wisp" as box
import "array"

fn main() -> int {
    let n: int = box.identity[int](9)
    let xs: string[] = box.empty[string]()
    array.push(xs, "a")
    print("n=${n}")
    print("len=${length(xs)}")
    return 0
}
