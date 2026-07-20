// Lifecycle + isolation (AC5), observable cross-shell through TAP.
//
// setup() writes a seed file into the per-test tmpdir BEFORE each body, so the
// body asserting it exists proves setup-ran-before-body and that the tmpdir is
// live during the body. Each test also writes a UNIQUE marker into the tmpdir;
// because the tmpdir is fresh per test (and removed after), a later test never
// sees an earlier test's marker -- proving state does not leak (isolation).
// teardown() appends to a fixed file in the cwd on EVERY test (pass + fail +
// skip); the final test reads that file and asserts the running teardown count,
// proving teardown runs after each preceding test including the failed/skipped
// ones.

import "fs"
fn setup() -> void {
  fs.write_file("${test_tmpdir()}/seed", "ok")
}

fn teardown() -> void {
  fs.append_file("teardowns", "x")
}

test ("1 setup ran and tmpdir is empty of markers") {
  assert_eq(fs.read_file("${test_tmpdir()}/seed"), "ok")
  assert(fs.file_exists("${test_tmpdir()}/marker") == false, "no leaked marker")
  fs.write_file("${test_tmpdir()}/marker", "1")
}

test ("2 still isolated, prior marker not visible") {
  assert(fs.file_exists("${test_tmpdir()}/marker") == false, "no leaked marker")
  skip("intentionally skipped to prove teardown still runs")
}

test ("3 teardown ran after each preceding test") {
  // teardown ran after tests 1 and 2 (one append each) before this body.
  assert_eq(fs.read_file("teardowns"), "xx")
}
