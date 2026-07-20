fn add(a: int, b: int) -> int { return a + b }
fn mul(a: int, b: int) -> int { return a * b }
fn getOp() -> fn(int, int) -> int { return add }
fn apply(g: fn(int, int) -> int, a: int, b: int) -> int { return g(a, b) }
fn main() -> int {
  print("direct=${add(3, 4)}")
  let f: fn(int, int) -> int = add
  print("indirect=${f(3, 4)}")
  print("getter=${getOp()(2, 5)}")
  print("apply=${apply(mul, 6, 7)}")
  let fns: (fn(int, int) -> int)[] = [add, mul]
  print("arr=${fns[1](6, 7)}")
  let m: {string: fn(int, int) -> int} = { "x": mul }
  print("dict=${m["x"](2, 8)}")
  return 0
}
