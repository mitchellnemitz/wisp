struct Inner { v: int }
struct Outer { inner: Inner, tags: string[] }
fn build(n: int) -> Outer {
  return Outer { inner: Inner { v: n }, tags: ["a", "b"] }
}
fn main() -> int {
  let o: Outer = build(7)
  print("v=${o.inner.v}")
  print("tag=${o.tags[1]}")
  o.inner.v = 42
  print("v=${o.inner.v}")
  return 0
}
