import "dict"
fn main() -> int {
  let m: {int: string} = { 10: "ten", 20: "twenty" }
  m[30] = "thirty"
  let total: int = 0
  for (k in m) { total = total + k }
  print("sum=${total}")
  print("doubled=${total * 2}")
  let ks: int[] = dict.keys(m)
  print("k0=${ks[0]}")
  print("at20=${m[20]}")
  return 0
}
