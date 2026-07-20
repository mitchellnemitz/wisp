struct Box { v: int }
fn ret_min() -> int { return -9223372036854775808 }
fn main() -> int {
  let a: int = -9223372036854775808
  print(to_string(a))
  print(to_string(-9223372036854775808))
  let arr: int[] = [-9223372036854775808]
  print(to_string(arr[0]))
  print(to_string(ret_min()))
  let d: {string: int} = { "k": -9223372036854775808 }
  print(to_string(d["k"]))
  let b: Box = Box { v: -9223372036854775808 }
  print(to_string(b.v))
  return 0
}
