// run_full-based testing (AC11): a test runs an external program via run_full
// and asserts on its typed RunResult fields.

import "process"
test ("run_full captures stdout and exit code") {
  let r: RunResult = process.run_full(["printf", "%s", "hello"])
  assert_eq(r.code, 0)
  assert_eq(r.stdout, "hello")
}

test ("run_full captures a nonzero exit") {
  let r: RunResult = process.run_full(["sh", "-c", "exit 7"])
  assert_eq(r.code, 7)
}
