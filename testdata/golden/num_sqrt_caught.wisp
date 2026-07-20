import "math"
fn main() -> int {
    try {
        print(to_string(math.sqrt(-1.0)))
    } catch (e) {
        print("caught sqrt")
    }
    return 0
}
