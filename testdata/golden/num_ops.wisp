import "math"
import "string"
fn main() -> int {
    print(to_string(unwrap_or(parse_int("007"), 0)) + " " + to_string(unwrap_or(parse_int("x"), 9)) + " " + to_string(unwrap_or(parse_int("-0"), 5)))
    print(to_string(unwrap_or(parse_float("1.50"), 0.0)) + " " + to_string(unwrap_or(parse_float("1e9"), 0.0)))
    print(to_string(math.clamp(15, 0, 10)) + " " + to_string(math.clamp(-3, 0, 10)) + " " + to_string(math.clamp(5, 0, 10)))
    print(to_string(math.clamp(2.5, 0.0, 2.0)))
    print(to_string(math.sign(-9)) + " " + to_string(math.sign(0)) + " " + to_string(math.sign(7)) + " " + to_string(math.sign(-1.5)) + " " + to_string(math.sign(-0.0)))
    print(to_string(math.floor(2.7)) + " " + to_string(math.floor(-1.5)) + " " + to_string(math.ceil(2.1)) + " " + to_string(math.ceil(-1.5)))
    print(to_string(math.round(0.5)) + " " + to_string(math.round(-0.5)) + " " + to_string(math.round(2.5)) + " " + to_string(math.trunc(-1.9)))
    print(to_string(math.sqrt(9.0)) + " " + to_string(math.sqrt(2.25)) + " " + to_string(math.sqrt(144.0)) + " " + to_string(math.sqrt(0.25)))
    print(to_string(math.gcd(12, 18)) + " " + to_string(math.gcd(0, 0)) + " " + to_string(math.gcd(-12, 8)) + " " + to_string(math.lcm(4, 6)) + " " + to_string(math.lcm(0, 5)))
    return 0
}
