// AC2: ln(-1.0) aborts located (domain x > 0).
import "math"
fn main() -> int {
    print(to_string(math.ln(-1.0)))
    return 0
}
