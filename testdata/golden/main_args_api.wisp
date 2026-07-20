import "array"
fn main(args: string[]) -> int {
  array.push(args, "appended")
  print(to_string(length(args)))
  print(args[0])
  print(args[1])
  return 0
}
