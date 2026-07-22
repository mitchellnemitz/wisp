enum Msg { Text(string) }
fn main() -> int {
  let m: Msg = Msg.Text("a;b|c$d`e\n")
  match (m) {
    case Text(s) { print(s) }
  }
  return 0
}
