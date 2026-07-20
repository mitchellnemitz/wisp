fn main() -> int {
  for (let i: int = 0; i < 4; i = i + 1) {
    if (i == 2) {
      print("skip${i}")
      continue
    }
    let j: int = 0
    while (j < 5) {
      if (j == 2) {
        break
      }
      print("i${i}j${j}")
      j = j + 1
    }
    print("after${i}")
  }
  print("done")
  return 0
}
