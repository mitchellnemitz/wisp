import "env"
import "fs"
import "process"
fn main() -> int {
  let payload: string = "$(echo PWN); `id`; rm -rf x"
  fs.write_file("inj_file", payload)
  fs.append_file("inj_file", payload)
  let back: string = fs.read_file("inj_file")
  print("rf=${back}")
  print("hasenv=${to_string(env.has(payload))}")
  let out: string = process.run(["echo", payload])
  print("run=${out}")
  try {
    print(env.get(payload))
  } catch (e) {
    print("env-unset")
  }
  return 0
}
