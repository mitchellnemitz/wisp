// Numeric / math standard library helpers.
import "math"
import "string"

fn main() -> int {
    print("parse_int: ${to_string(unwrap_or(parse_int("42"), 0))} ${to_string(unwrap_or(parse_int("oops"), -1))}")
    print("parse_float: ${to_string(unwrap_or(parse_float("3.14"), 0.0))}")
    print("clamp: ${to_string(math.clamp(15, 0, 10))} ${to_string(math.clamp(2.5, 0.0, 2.0))}")
    print("sign: ${to_string(math.sign(-9))} ${to_string(math.sign(0))} ${to_string(math.sign(7))}")
    print("floor/ceil/round: ${to_string(math.floor(2.7))} ${to_string(math.ceil(2.1))} ${to_string(math.round(2.5))}")
    print("trunc: ${to_string(math.trunc(-3.9))}")
    print("sqrt: ${to_string(math.sqrt(144.0))} ${to_string(math.sqrt(0.25))}")
    print("gcd/lcm: ${to_string(math.gcd(24, 36))} ${to_string(math.lcm(4, 6))}")
    return 0
}
