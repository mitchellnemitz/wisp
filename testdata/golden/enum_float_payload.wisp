enum Num { F(float), Z }
fn main() -> int {
  let n: Num = Num.F(1.5)
  match (n) {
    case F(x) { print(to_string(x)) }
    case Z { print("z") }
  }
  return 0
}
