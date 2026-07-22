enum Expr { IntLit(int), Ident(string), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  match (e) {
    case IntLit(_) { print("i") }
    case Ident(_) { print("s") }
    case Unit(x) { print("u") }
  }
  return 0
}
