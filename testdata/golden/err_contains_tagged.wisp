import "array"
enum Expr { IntLit(int), Unit }
fn main() -> int {
  let xs: Expr[] = [Expr.Unit]
  if (array.contains(xs, Expr.Unit)) { print("y") }
  return 0
}
