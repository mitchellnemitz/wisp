import "array"
import "string"
fn main() -> int {
  // Hostile values: a glob and a word-split/semicolon payload. If any operand
  // reached the shell unquoted, ordering/sort would glob-expand or word-split
  // instead of comparing bytes. The exact output proves byte-literal handling.
  let a: string = "*"
  let b: string = "a; rm -rf x"
  print(to_string(a < b))
  print(to_string(a >= b))
  let xs: string[] = [b, a]
  print(string.join(array.sort(xs), "|"))
  return 0
}
