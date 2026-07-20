import "process"
fn glue(a: string, b: string) -> string {
  let r: string = process.run(["echo", "ignored"])
  return a + b
}
fn main() -> int {
  print(glue("p", "q"))
  return 0
}
