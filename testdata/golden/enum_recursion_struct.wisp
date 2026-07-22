struct BinaryPayload { op: string, left: Expr, right: Expr }
enum Expr { IntLit(int), Binary(BinaryPayload) }

fn eval(e: Expr) -> int {
  match (e) {
    case IntLit(n) {
      print(to_string(n))
      return n
    }
    case Binary(b) {
      print(b.op)
      return eval(b.left) + eval(b.right)
    }
  }
  return 0
}

fn main() -> int {
  let tree: Expr = Expr.Binary(BinaryPayload{op: "+", left: Expr.IntLit(1), right: Expr.Binary(BinaryPayload{op: "*", left: Expr.IntLit(2), right: Expr.IntLit(3)})})
  let _ : int = eval(tree)
  return 0
}
