test ("assertion failure is FAIL") {
  assert(false, "boom")
}

test ("an unexpected fault is ERROR") {
  let xs: int[] = []
  print(to_string(xs[5]))
}
