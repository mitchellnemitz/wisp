// AC2 + T2 FIX proof: exp(-40.0) ~= 4.2e-18. The negative-x reciprocal computes
// 1/exp(40.0) -- a CORRECT, tiny, positive magnitude -- rather than the wrong
// (even negative) value the raw alternating series returned before the fix. That
// magnitude is not representable in wisp's exponent-free float range, so it
// renders in exponent notation and aborts located, instead of silently passing
// a finite-looking wrong number through the finite-check glob.
import "math"
fn main() -> int {
    print(to_string(math.exp(-40.0)))
    return 0
}
