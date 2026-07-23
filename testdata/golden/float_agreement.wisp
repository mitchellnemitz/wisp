import "dict"

fn sw(x: float) -> string {
  switch (x) {
    case 0.0 { return "zero" }
    case 1.0 { return "one" }
    default { return "other" }
  }
}

fn main() -> int {
  // MUST collide: -0.0 vs 0.0
  print("${-0.0 == 0.0}")
  print(sw(-0.0))
  // MUST collide: 1.0 vs 1.00, and 1.0 vs a computed 0.5 + 0.5
  print("${1.0 == 1.00}")
  print("${1.0 == 0.5 + 0.5}")
  print(sw(0.5 + 0.5))
  // MUST NOT collide: 0.1 + 0.2 vs 0.3
  print("${(0.1 + 0.2) == 0.3}")
  let m: {float: int} = {}
  m[0.1 + 0.2] = 1
  m[0.3] = 2
  print("${length(dict.keys(m))}")
  // Each collision pair MUST also collide as DICT KEYS, not only ==/switch:
  let c1: {float: int} = {}
  c1[0.0] = 1
  c1[-0.0] = 2
  print("${length(dict.keys(c1))}")
  let c2: {float: int} = {}
  c2[1.0] = 1
  c2[1.00] = 2
  print("${length(dict.keys(c2))}")
  return 0
}
