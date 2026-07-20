import "array"
enum Color { Red, Green, Blue }

fn main() -> int {
  let cs: Color[] = [Color.Red, Color.Green, Color.Red, Color.Blue]
  let uniq: Color[] = array.unique(cs)
  print(to_string(length(uniq)))
  print(to_string(to_int(uniq[0])))
  print(to_string(to_int(uniq[1])))
  print(to_string(to_int(uniq[2])))
  return 0
}
