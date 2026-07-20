fn task_result(cmd: string) -> (int, string) {
    if (cmd == "ok") {
        return (0, "success")
    }
    return (1, "failed")
}

fn divmod(a: int, b: int) -> (int, int) {
    return (a / b, a % b)
}

fn main() -> int {
    // basic destructuring
    let (code: int, out: string) = task_result("ok")
    print("code: ${to_string(code)}")
    print("out: ${out}")
    // discard first element with bare _
    let (_, msg: string) = task_result("fail")
    print("msg: ${msg}")
    // final (immutable) destructuring
    final (q: int, r: int) = divmod(17, 5)
    print("17/5 = ${to_string(q)} rem ${to_string(r)}")
    return 0
}
