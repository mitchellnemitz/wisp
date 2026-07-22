enum Expr { IntLit(int), Ident(string), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  match (e) {
    case IntLit(_) { print("i") }
    case Ident(_) { print("s") }
    case Unit { print("u") }
    case Nope(x) { print("n") }
  }
  return 0
}
