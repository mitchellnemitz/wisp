// AC2: log10(0.0) aborts located -- log10 composes ln, so the ln domain guard
// fires (the abort is reported as ln() since that is where the guard lives).
import "math"
fn main() -> int {
    print(to_string(math.log10(0.0)))
    return 0
}
