struct Op { name: string, f: fn(int, int) -> int }
fn add(a: int, b: int) -> int { return a + b }
fn mul(a: int, b: int) -> int { return a * b }
fn main() -> int {
  let o: Op = Op { name: "mul", f: mul }
  print("${o.name}=${o.f(6, 7)}")
  let p: Op = Op { name: "add", f: add }
  print("${p.name}=${p.f(6, 7)}")
  return 0
}
