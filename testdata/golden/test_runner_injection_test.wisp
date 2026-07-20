// Injection-safety (N1/AC4): a test NAME and a skip REASON containing shell
// metacharacters reach the TAP output as inert data only -- never evaluated,
// never glob-expanded -- identically on every shell.

test ("name $(echo PWNED) and *glob* and ; rm -rf and backtick id") {
  skip("reason $(echo PWNED) and *.go and ; echo X and backtick id")
}
