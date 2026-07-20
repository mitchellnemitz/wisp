fn main() -> int {
  let ok: Result[int] = Ok(10)
  let err: Result[int] = Err(error("fail"))
  print(debug(ok))
  print(debug(err))
  return 0
}
