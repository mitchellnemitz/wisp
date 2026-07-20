// AC2: ln(0.0) aborts located PROMPTLY -- the x <= 0 domain guard fires before
// the range-reduction loop, so it does not hang.
import "math"
fn main() -> int {
    print(to_string(math.ln(0.0)))
    return 0
}
