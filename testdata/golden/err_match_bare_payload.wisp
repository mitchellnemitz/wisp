enum Expr { IntLit(int), Ident(string), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  match (e) {
    case IntLit { print("i") }
    case Ident(_) { print("s") }
    case Unit { print("u") }
  }
  return 0
}
