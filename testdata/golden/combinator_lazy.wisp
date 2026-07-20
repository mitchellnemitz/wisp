import "array"
fn loud_map(x: int) -> int { print("M")
    return x }
fn loud_and(x: int) -> Optional[int] { print("A")
    return Some(x) }
fn loud_filter(x: int) -> bool { print("F")
    return true }
fn loud_fb() -> Optional[int] { print("O")
    return Some(0) }
fn loud_rmap(x: int) -> int { print("RM")
    return x }
fn loud_rand(x: int) -> Result[int] { print("RA")
    return Ok(x) }
fn loud_ror(e: error) -> Result[int] { print("RO")
    return Ok(0) }
fn loud_rme(e: error) -> error { print("RE")
    return e }
fn main() -> int {
    let n: Optional[int] = None
    let a: Optional[int] = array.map(n, loud_map)
    let b: Optional[int] = and_then(n, loud_and)
    let c: Optional[int] = array.filter(n, loud_filter)
    let s: Optional[int] = Some(1)
    let d: Optional[int] = or_else(s, loud_fb)
    let er: error = error("x")
    let err: Result[int] = Err(er)
    let e2: Result[int] = array.map(err, loud_rmap)
    let f2: Result[int] = and_then(err, loud_rand)
    let ok: Result[int] = Ok(1)
    let g2: Result[int] = or_else(ok, loud_ror)
    let h2: Result[int] = map_err(ok, loud_rme)
    return 0
}
