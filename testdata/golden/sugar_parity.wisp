// SC-006 sugar-removal parity: int_or/float_or/get_or/env_or are gone, but
// unwrap_or(parse_int/parse_float/dict.get/env.get, fb) reproduces the exact
// same values across the full matrix each removed builtin used to cover.
import "dict"
import "env"
fn main() -> int {
    // int_or(s, fb) == unwrap_or(parse_int(s), fb)
    print(to_string(unwrap_or(parse_int("42"), -1)))   // 42
    print(to_string(unwrap_or(parse_int("007"), -1)))  // 7
    print(to_string(unwrap_or(parse_int("+9"), -1)))   // 9
    print(to_string(unwrap_or(parse_int("abc"), -1)))  // -1
    print(to_string(unwrap_or(parse_int(""), -1)))     // -1
    print(to_string(unwrap_or(parse_int("-9223372036854775808"), -1))) // INT_MIN
    print(to_string(unwrap_or(parse_int("9223372036854775808"), -1)))  // -1 (out of range)
    // float_or == unwrap_or(parse_float(s), fb)
    print(to_string(unwrap_or(parse_float("3.14"), -1.0)))
    print(to_string(unwrap_or(parse_float("x"), -1.0)))     // -1
    print(to_string(unwrap_or(parse_float(""), -1.0)))      // -1
    print(to_string(unwrap_or(parse_float("5."), -1.0)))    // -1 (trailing dot rejected)
    // get_or(d, k, fb) == unwrap_or(dict.get(d, k), fb)
    let m: {string: int} = {"a": 1}
    print(to_string(unwrap_or(dict.get(m, "a"), -1)))  // 1  (present)
    print(to_string(unwrap_or(dict.get(m, "z"), -1)))  // -1 (absent)
    // env_or(n, fb) == unwrap_or(env.get(n), fb)
    env.set("WISP_PARITY", "v")
    env.set("WISP_PARITY_EMPTY", "")
    print(unwrap_or(env.get("WISP_PARITY"), "FB"))         // v   (set-nonempty)
    print(unwrap_or(env.get("WISP_PARITY_EMPTY"), "FB"))   //     (set-empty -> "")
    print(unwrap_or(env.get("WISP_PARITY_UNSET_XZ"), "FB")) // FB (unset)
    return 0
}
