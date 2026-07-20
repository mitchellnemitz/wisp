import "math"
fn main() -> int {
  let hi: int = math.int_max()
  let lo: int = math.int_min()
  print(to_string(lo == -hi - 1))
  return 0
}
