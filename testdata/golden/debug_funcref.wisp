fn add(x: int, y: int) -> int { return x + y }
fn main() -> int {
  let f: fn(int,int)->int = add
  print(debug(f))
  return 0
}
