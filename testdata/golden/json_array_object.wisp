import "json"
fn main() -> int {
  let a: json.Value[] = [json.from_int(1), json.from_string("x"), json.null()]
  print(json.encode(json.array(a)))
  let d: {string: json.Value} = {"k": json.from_int(1), "s": json.from_string("v"), "b": json.from_bool(false)}
  print(json.encode(json.object(d)))
  return 0
}
