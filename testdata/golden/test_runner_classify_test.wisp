test ("a passes") {
  assert_eq(1 + 1, 2)
}

test ("b fails an assertion") {
  assert_eq(1, 2)
}

test ("c is skipped") {
  skip("not ready yet")
}
