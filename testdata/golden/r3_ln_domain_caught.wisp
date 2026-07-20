// The located domain abort is catchable, exactly like sqrt(-1).
import "math"
fn main() -> int {
    try {
        print(to_string(math.ln(0.0)))
    } catch (e) {
        print("caught ln domain")
    }
    return 0
}
