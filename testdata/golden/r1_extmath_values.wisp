// AC1: exp/ln/log10/log2/pi compute correct values within float precision.
// Each accurate case is scaled by a power of ten and rounded to an int so the
// printed form is exact and byte-identical across dash/busybox/bash/zsh (a raw
// %.17g float would differ in the last ULP between awks). The reciprocal-based
// exp handles negative x accurately: exp(-5.0) ~= 0.006737947, NOT a negative
// or wrong value. log10/log2 use a baked ln-constant that matches the series
// (log10(10.0) ~= 1, log2(2.0) ~= 1).
import "math"
fn main() -> int {
  // exp(0.0) == 1 exactly (the series' first term).
  print(to_string(math.exp(0.0)))
  // ln(exp(1.0)) ~= 1; *1e9 rounds to 1000000000.
  print(to_string(math.round(math.ln(math.exp(1.0)) * 1000000000.0)))
  // exp(ln(5.0)) ~= 5; *1e6 rounds to 5000000.
  print(to_string(math.round(math.exp(math.ln(5.0)) * 1000000.0)))
  // exp(1.0) ~= 2.718281828; *1e8 (not 1e9: 2.7e9 overflows 32-bit busybox awk int).
  print(to_string(math.round(math.exp(1.0) * 100000000.0)))
  // exp(-5.0) ~= 0.006737947 (POSITIVE, accurate -- the reciprocal fix); *1e9.
  print(to_string(math.round(math.exp(-5.0) * 1000000000.0)))
  // ln(1.0) == 0.
  print(to_string(math.round(math.ln(1.0) * 1000000.0)))
  // log10(1000.0) ~= 3; log10(10.0) ~= 1. The ~=3 case uses *1e8 (3e9 overflows
  // 32-bit busybox awk int); the ~=1 case fits at *1e9.
  print(to_string(math.round(math.log10(1000.0) * 100000000.0)))
  print(to_string(math.round(math.log10(10.0) * 1000000000.0)))
  // log2(8.0) ~= 3 (*1e8, overflow-safe); log2(2.0) ~= 1 (*1e9).
  print(to_string(math.round(math.log2(8.0) * 100000000.0)))
  print(to_string(math.round(math.log2(2.0) * 1000000000.0)))
  // pi() is the float literal 3.141592653589793.
  print(to_string(math.pi()))
  return 0
}
