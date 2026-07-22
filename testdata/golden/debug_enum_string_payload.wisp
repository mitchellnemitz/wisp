enum Msg { Text(string) }
fn main() -> int {
  print(debug(Msg.Text("a;b\n")))
  return 0
}
