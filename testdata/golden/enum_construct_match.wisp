enum Expr { IntLit(int), Ident(string), Unit }
fn main() -> int {
  let e: Expr = Expr.IntLit(3)
  match (e) {
    case IntLit(n) { print(to_string(n)) }
    case Ident(name) { print(name) }
    case Unit { print("u") }
  }
  return 0
}
