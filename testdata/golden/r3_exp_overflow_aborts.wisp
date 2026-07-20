// AC2: exp(100.0) ~= 2.7e43 is finite but too large to represent in wisp's
// exponent-free float range, so it renders in exponent notation and aborts
// located (the always-finite-AND-representable float invariant), exactly like
// an over-large fmul.
import "math"
fn main() -> int {
    print(to_string(math.exp(100.0)))
    return 0
}
