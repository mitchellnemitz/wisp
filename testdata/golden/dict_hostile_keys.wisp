import "dict"
enum Hostile: string { K = "-a\\b'c\"d$e`f g\nh*i?j[k;l" }

fn main() -> int {
  let k: string = "-a\\b'c\"d$e`f g\nh*i?j[k;l"
  let s: {string: int} = {}
  s[k] = 42
  print("${unwrap_or(dict.get(s, k), -1)}")
  let ks: string[] = dict.keys(s)
  print("${length(ks)}")
  print("${ks[0] == k}")
  let e: {Hostile: int} = {}
  e[Hostile.K] = 7
  print("${unwrap_or(dict.get(e, Hostile.K), -1)}")
  let eks: Hostile[] = dict.keys(e)
  print("${eks[0] == Hostile.K}")
  return 0
}
