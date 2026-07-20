import "dict"
fn main() -> int {
  let d: {string: int} = {}
  d["a\nb"] = 7
  d["x\ny\n"] = 9
  let ks: string[] = dict.keys(d)
  print(to_string(length(ks[0])))
  print(to_string(d[ks[0]]))
  print(to_string(length(ks[1])))
  print(to_string(d[ks[1]]))
  return 0
}
