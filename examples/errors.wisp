// Error handling: a runtime fault becomes catchable inside try, and finally
// always runs.
fn divide(a: int, b: int) -> int {
    return a / b
}

fn main() -> int {
    try {
        print("10 / 2 = ${divide(10, 2)}")
        print("10 / 0 = ${divide(10, 0)}")
        print("this line is skipped")
    } catch (e) {
        print("caught: ${e.message}")
    } finally {
        print("cleanup runs either way")
    }
    return 0
}
