import "array"
fn show(s: string) -> void { print("got ${s}") }
fn dbl(x: int) -> int { return x * 2 }
fn even(x: int) -> bool { return x % 2 == 0 }
fn noop(x: int) -> void { print("never") }
fn main() -> int {
  let names: string[] = ["a", "b", "c"]
  array.each(names, show)
  let empty: int[] = []
  let m: int[] = array.map(empty, dbl)
  let f: int[] = array.filter(empty, even)
  array.each(empty, noop)
  print("m=${length(m)}")
  print("f=${length(f)}")
  return 0
}
