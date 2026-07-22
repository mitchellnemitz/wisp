import "array"
enum Expr { IntLit(int), Unit }
fn main() -> int {
  let xs: Expr[] = [Expr.Unit]
  let i: Optional[int] = array.index_of(xs, Expr.Unit)
  return 0
}
