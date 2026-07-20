// AC2: log2(-2.0) aborts located via the composed ln domain guard.
import "math"
fn main() -> int {
    print(to_string(math.log2(-2.0)))
    return 0
}
