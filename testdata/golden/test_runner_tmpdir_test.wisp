// test_tmpdir() (AC7): writable, per-test, unique; exists during the test and is
// gone after. teardown can still see it (the runner removes it only AFTER
// teardown). The body writes a file under it and reads it back; setup seeds a
// file the body asserts present; teardown asserts the file still readable (so
// the dir is alive through teardown).

import "fs"
fn setup() -> void {
  fs.write_file("${test_tmpdir()}/from_setup", "seed")
}

fn teardown() -> void {
  assert_eq(fs.read_file("${test_tmpdir()}/from_setup"), "seed")
}

test ("writes and reads under the per-test tmpdir") {
  let p: string = "${test_tmpdir()}/data"
  fs.write_file(p, "payload")
  assert_eq(fs.read_file(p), "payload")
  assert(fs.is_dir(test_tmpdir()), "tmpdir is a directory")
}
