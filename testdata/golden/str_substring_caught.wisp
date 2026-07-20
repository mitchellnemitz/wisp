import "string"
fn main() -> int {
    try {
        print(string.substring("ab", 0, 9))
    } catch (e) {
        print("caught range")
    }
    return 0
}
