import "string"
fn main() -> int {
  let trailing: string[] = string.split("a,b,", ",")
  print("${length(trailing)}")
  let empty: string[] = string.split("", ",")
  print("${length(empty)}")
  print("[${empty[0]}]")
  let multi: string[] = string.split("a::b::c", "::")
  print(string.join(multi, "-"))
  return 0
}
