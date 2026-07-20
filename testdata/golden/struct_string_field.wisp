struct Msg { text: string }
fn main() -> int {
  let m: Msg = Msg { text: "a'b\"c$d`e;f|g" }
  print(m.text)
  let xs: string[] = ["$(echo x)", "y`z`"]
  for (s in xs) {
    print(s)
  }
  return 0
}
