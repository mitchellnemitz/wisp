import "array"
enum Expr { IntLit(int), Unit }
fn main() -> int {
  let xs: Expr[] = [Expr.Unit, Expr.Unit]
  let u: Expr[] = array.unique(xs)
  return 0
}
