---
title: Standard library
---


Every function here is a builtin provided by the compiler. A small set of
always-in-scope builtins (`print`, `length`, the `to_<type>` conversions,
`assert`, and similar) need no import. Most builtins, though, live in a
module and are reachable only as `<module>.<member>` after an `import
"<module>"` (`fs`, `string`, `array`, `dict`, `math`, `env`, `process`,
`regex`, `json`, ...); there is no bare/flat spelling for these. The
always-in-scope builtins (see
[Referenceable builtins](/guide/language/#referenceable-builtins) for the list)
are reserved — you cannot declare a function, parameter, or variable with one
of those names, or with a reserved constant or type name (`Optional`,
`Result`, `RunResult`, `Process`). A module member's bare name is not reserved
and may be used as an identifier. Builtins are emitted into the output only
when used, so unused ones cost nothing.

String arguments and results are inert data. A value that contains shell-active
text flows through these functions unchanged and is never executed. Operations
on a separator or search string match literally, not as a glob or regex.

## Naming convention

Every builtin is named in `snake_case`: all lowercase, words joined by
underscores (`env.has`, `fs.read_file`, `string.starts_with`,
`string.index_of`). This is the one and only casing for the standard library,
and it does not change. New builtins are always `snake_case`; `camelCase` and
`PascalCase` are never used. A multi-word name keeps snake_case here, so it is
`fs.read_file`, never `fs.readFile` or `fs.ReadFile`.

## Output

### print

```
print(msg: string, to: int = stdout) -> void
```

Writes `msg` followed by a newline. `to` selects the stream and must be exactly
`stdout` or `stderr`, resolved at compile time.

```wisp
print("hello")
print("oops", stderr)
```

## Conversions

These names are prefixed with `to_` and take the value to convert.

### to_string

```
to_string(x: int | float | bool | string) -> string
```

Returns the canonical text form. For int, bool, and string it is the value
itself. For float it is the full-precision decimal produced by `%.17g`, so a
value that cannot be represented exactly shows all of its digits.

```wisp
to_string(42)     // "42"
to_string(true)   // "true"
to_string(2.0)    // "2"
to_string(3.14)   // "3.1400000000000001"
```

### to_int

```
to_int(s: string) -> int
to_int(f: float) -> int
```

From a string, parses an optional sign and digits with no surrounding
whitespace, and aborts on bad or out-of-range input. From a float, truncates
toward zero. There is no `to_int(bool)`.

```wisp
to_int("7")     // 7
to_int(3.9)     // 3
```

### to_float

```
to_float(x: int | string) -> float
```

From a string, accepts an optional sign, digits, and an optional fractional
part, with no exponent form, and aborts on bad input. There is no
`to_float(bool)`.

```wisp
to_float(2)       // 2.0
to_float("3.5")   // 3.5
```

### to_bool

```
to_bool(x: int | float | string) -> bool
```

From an int, `0` is false and nonzero is true. From a float, numeric zero is
false. From a string, only `"true"` and `"false"` convert; anything else
aborts. See the [language reference](/guide/language/#booleans) for the full table.

```wisp
to_bool(0)        // false
to_bool("true")   // true
```

### error

```
error(msg: string) -> error
```

Constructs an error handle whose `message` field is `msg` and `code` field is `0`.
See [error handling](/guide/language/#error-handling).

### error_with

```
error_with(code: int, msg: string) -> error
```

Constructs an error handle with both fields set. `.code` is an arbitrary integer
(caller-defined, no reserved range) and `.message` is the human-readable description.
Both fields survive `throw`/`catch`.

```wisp
try {
  throw error_with(42, "not found")
} catch (e) {
  print(e.message)      // "not found"
  print(to_string(e.code)) // "42"
}
```

Errors produced by built-in faults (e.g. failed `to_int()` conversion) always have
`code = 0` when caught.

### wrap

```
wrap(err: error, msg: string) -> error
```

Attaches a higher-level message to an existing error. Returns a NEW error handle
whose `.message` is `msg`, whose `.code` is `0`, and which carries `err` as its
cause. `err` is unchanged. `wrap` does NOT prefix or concatenate `msg` with the
inner message -- the inner message is reached via `cause`, not folded into the
outer one.

The cause survives the VALUE paths: `throw`/`catch`, `unwrap_err`, and
`map_err`/`or_else` all preserve it. It does NOT survive the fault path:
`unwrap(result)` goes through the runtime fault machinery, which discards the
cause (the resulting caught error has `cause == None`). To recover an error with
its cause intact, use `unwrap_err` (the value path) instead.

`wrap` is arbitrarily nestable. Walk the chain with `match`:

```wisp
let cur: error = top
match (cause(cur)) {
    case Some(inner) {
        // cur = inner  -- continue walking
    }
    case None {
        // root reached
    }
}
```

**Non-goals:** there is no `e.cause` field (use the `cause` builtin); no
automatic cause-chain printing on an uncaught abort (only the top-level message
is printed); no `root_cause` or `errors.Is`-style helpers yet.

### cause

```
cause(err: error) -> Optional[error]
```

Returns `Some(inner)` if `err` was produced by `wrap`, otherwise `None`.
Errors from `error(...)`, `error_with(...)`, and runtime faults carry no cause.

```wisp
fn main() -> int {
    let inner: error = error("file not found")
    let outer: error = wrap(inner, "read config failed")
    print(outer.message)
    print(to_string(outer.code))
    let c: Optional[error] = cause(outer)
    match (c) {
        case Some(e) {
            print(e.message)
        }
        case None {
            print("no cause")
        }
    }
    let c2: Optional[error] = cause(inner)
    match (c2) {
        case Some(e) {
            print("unexpected: " + e.message)
        }
        case None {
            print("root has no cause")
        }
    }
    return 0
}
```

### debug

```
debug(x) -> string      // x is a value of ANY type
```

Returns a structural representation of any value as a string. This is a
pure renderer -- it returns the string and does not print anything. `debug`
accepts a value of every type, including aggregates and opaque handles that
`to_string()` rejects (there is no user-facing `any` type; `debug` accepts the
broad input as a built-in special case). Use it to inspect values during development; for normal string
conversion of scalars, use `to_string()`.

**Format contract by type:**

| type | format | example |
|---|---|---|
| `int`, `bool` | as-is | `42`, `true` |
| `float` | canonical (same as `to_string()`) | `1.5` |
| `string` | double-quoted | `"hello"` |
| `T[]` | comma-separated elements, bracketed | `[1, 2, 3]`, `[]` |
| `Optional[T]` | `Some(v)` or `None` | `Some(42)`, `None` |
| `Result[T]` | `Ok(v)` or `Err(error("msg", code))` | `Ok(10)`, `Err(error("fail", 0))` |
| `{string: V}` | insertion order, string keys double-quoted | `{"a": 1, "b": 2}`, `{}` |
| `{int: V}` | insertion order, int keys bare | `{1: "a", 2: "b"}` |
| `struct Name` | `Name { field: value, ... }` | `Point { x: 3, y: 4 }` |
| `(T1, T2, ...)` | parenthesized, comma-separated | `(5, "hi")` |
| `error` | `error("msg", code)` | `error("oops", 0)` |
| `fn(...)-> T` | `<fn(params)->ret>` | `<fn(int,int)->int>` |
| `RunResult` | `RunResult { stdout: "...", stderr: "...", code: N }` | see below |

```wisp
print(debug(42))          // 42
print(debug(1.5))         // 1.5
print(debug("hello"))     // "hello"
print(debug([1, 2, 3]))   // [1, 2, 3]
let s: Optional[int] = Some(42)
print(debug(s))           // Some(42)
let ok: Result[int] = Ok(10)
print(debug(ok))          // Ok(10)
```

**Logging idiom:** to log a value to stderr without changing the program's
stdout, pass the result of `debug` to `print` with the `stderr` stream:

```wisp
print(debug(xs), stderr)
```

**Limitation:** string values are rendered double-quoted, but embedded
double-quote characters and newlines inside the string are NOT escaped. This
is a best-effort debug representation, not a serialization format.

**Note:** `debug` is a reserved builtin name and cannot be used as a user
identifier -- function, `let` binding, parameter, `struct`, or `const`.

## Strings

### length

```
length(s: string) -> int
length(xs: T[]) -> int
```

The byte length of a string, or the element count of an array. The form is
chosen by the argument type. The string form counts bytes, not characters, and
is the same under every shell: a UTF-8 multibyte string reports its byte length
on dash, busybox ash, bash, and zsh alike.

```wisp
length("hello")   // 5
```

### string.lower, string.upper

```
string.lower(s: string) -> string
string.upper(s: string) -> string
```

ASCII case folding. Bytes outside A-Z and a-z, including a trailing newline, are
left unchanged.

```wisp
string.lower("MixedCase")   // "mixedcase"
```

### string.trim

```
string.trim(s: string) -> string
```

Strips leading and trailing spaces, tabs, newlines, and carriage returns.
Interior whitespace is untouched.

```wisp
string.trim("  hi  ")   // "hi"
```

### string.replace

```
string.replace(s: string, search: string, replacement: string) -> string
```

Replaces every non-overlapping occurrence of `search`, left to right. The
replacement may be empty. An empty `search` aborts. Matching is byte-based and
produces the same result under every shell, including for UTF-8 multibyte input.

```wisp
string.replace("a-b-c", "-", "_")   // "a_b_c"
```

### string.split, string.join

```
string.split(s: string, sep: string) -> string[]
string.join(parts: string[], sep: string) -> string
```

`split` divides on each non-overlapping occurrence of `sep`. An empty `sep`
aborts. Splitting `""` yields one empty element. A trailing separator yields a
trailing empty element. `join` concatenates with `sep` between elements;
joining an empty array yields `""` and `string.join(parts, "")` is plain concatenation.
(The empty array must carry an element type, e.g. `let e: string[] = []; string.join(e, ",")`;
a bare `[]` argument does not compile because its element type cannot be inferred.)

```wisp
string.split("one,two,three", ",")   // ["one", "two", "three"]
string.join(["a", "b", "c"], ", ")   // "a, b, c"
```

### string.contains, string.starts_with, string.ends_with

```
string.contains(s: string, sub: string) -> bool
string.starts_with(s: string, pre: string) -> bool
string.ends_with(s: string, suf: string) -> bool
```

Literal substring, prefix, and suffix tests. An empty needle returns true.
`contains`, `starts_with`, and `ends_with` are byte-based and produce the same
result under every shell, including for UTF-8 multibyte input. `contains` is also
defined for arrays; see below.

```wisp
string.contains("hello", "ell")    // true
string.starts_with("hello", "he")  // true
```

### string.index_of

```
string.index_of(s: string, sub: string) -> Optional[int]
string.index_of(xs: T[], x: T) -> Optional[int]   // T is int, bool, string, or an enum type
```

The form is chosen by the type of the first argument.

String form: `Some(i)` with the byte index of the first occurrence of `sub`, or
`None` if absent. An empty `sub` returns `Some(0)`. The index is a byte offset
and is the same under every shell, including for UTF-8 multibyte input. See
[Optional](/guide/language/#optional) for the access builtins
(`is_some`/`unwrap`/`unwrap_or`).

Array form: `Some(i)` with the index of the first element equal to `x`, or
`None` when `x` is not in `xs`. Defined only for arrays of int, bool, string,
or an enum type (same comparable-element restriction as `contains`). The
source is unchanged.

```wisp
unwrap_or(string.index_of("hello", "l"), -1)        // 2
unwrap_or(string.index_of("hello", "z"), -1)        // -1
unwrap_or(string.index_of([10, 20, 30], 20), -1)    // 1
is_none(string.index_of([10, 20, 30], 99))          // true
```

### string.repeat

```
string.repeat(s: string, n: int) -> string
```

`s` repeated `n` times. `n` of 0 is the empty string. A negative `n` aborts.

```wisp
string.repeat("ab", 3)   // "ababab"
```

### string.substring, string.char_at

```
string.substring(s: string, start: int, end: int) -> string
string.char_at(s: string, i: int) -> string
```

`substring` returns the bytes in the half-open range `[start, end)`; it requires
`0 <= start <= end <= length(s)` and aborts (catchable) otherwise. `char_at`
returns the one-byte string at byte index `i` and aborts on an out-of-range index.
Indices are byte offsets, not codepoint positions. The implementation uses
`LC_ALL=C awk`, so results are byte-identical across all supported shells
including dash, busybox ash, bash, and zsh.

```wisp
string.substring("hello", 1, 4)   // "ell"
string.char_at("hello", 0)        // "h"
```

### string.last_index_of, string.count

```
string.last_index_of(s: string, sub: string) -> Optional[int]
string.count(s: string, sub: string) -> int
```

`last_index_of` returns `Some(i)` with the byte index of the rightmost
occurrence of `sub`, or `None`; an empty `sub` returns `Some(length(s))`.
`count` returns the number of non-overlapping occurrences; an empty `sub`
aborts. The index and the count are byte-based and the same under every shell,
including for UTF-8 multibyte input.

```wisp
unwrap_or(string.last_index_of("abcabc", "bc"), -1)   // 4
string.count("a.b.c", ".")                            // 2
```

### string.replace_first

```
string.replace_first(s: string, search: string, with: string) -> string
```

Replaces only the first occurrence of `search`. An empty `search` aborts; a string
with no match is returned unchanged. Matching is byte-based and produces the same
result under every shell, including for UTF-8 multibyte input.

### string.trim_start, string.trim_end, string.trim_prefix, string.trim_suffix

```
string.trim_start(s: string) -> string
string.trim_end(s: string) -> string
string.trim_prefix(s: string, pre: string) -> string
string.trim_suffix(s: string, suf: string) -> string
```

`trim_start`/`trim_end` strip whitespace (space, tab, newline, carriage return)
from one side. `trim_prefix`/`trim_suffix` remove the affix only when present
(otherwise the string is unchanged); an empty affix is a no-op.

```wisp
string.trim_prefix("unhappy", "un")   // "happy"
```

### string.pad_start, string.pad_end

```
string.pad_start(s: string, width: int, fill: string) -> string
string.pad_end(s: string, width: int, fill: string) -> string
```

Pads `s` with copies of `fill` (final copy truncated) until it is exactly `width`
bytes, on the start or end. Width, the gap, and the final-copy truncation are
all byte-based and the same under every shell. If `length(s) >= width` (or
`width <= 0`) the string is returned unchanged and `fill` is not consulted;
otherwise an empty `fill` aborts. Because the target width is in bytes, a
multibyte (UTF-8) `fill` whose last copy does not land on a character boundary
is truncated mid-character, yielding the same partial bytes on every shell.

```wisp
string.pad_start("42", 5, "0")   // "00042"
string.pad_end("hi", 5, ".")     // "hi..."
```

### string.lines, string.is_empty

```
string.lines(s: string) -> string[]
string.is_empty(s: string) -> bool
```

`lines` splits `s` into its newline-separated lines: the empty string yields an
empty array, and a single trailing newline does not add an empty final line
(`"a\nb\n"` -> `["a", "b"]`). `is_empty` is `length(s) == 0`.

### string.reverse

```
string.reverse(s: string) -> string
```

Returns a fresh string with the bytes of `s` in reverse order. An empty input
returns an empty string. Total; never aborts. Byte-oriented: multibyte sequences
are reversed byte-for-byte, not rune-for-rune (consistent with the rest of the
byte model).

```wisp
string.reverse("hello")  // "olleh"
string.reverse("")        // ""
```

### string.ord

```
string.ord(s: string) -> int
```

Returns the numeric byte value (1-255) of the first byte of `s`. Aborts located
(catchable in `try`) if `s` is empty. If `s` has more than one byte, only the
first byte is examined.

```wisp
string.ord("A")      // 65
string.ord("hello")  // 104  (first byte of 'h')
```

### string.chr

```
string.chr(code: int) -> string
```

Returns a single-byte string whose byte value is `code`. `code` must be in the
range 1-255 inclusive; aborts located for 0 or out-of-range values. `string.chr(0)` is
rejected because the null byte is not representable in POSIX shell variables.

```wisp
string.chr(65)                       // "A"
string.chr(string.ord(string.char_at("hello", 0))) // "h"  (round-trip)
```

Doctest -- round-trip through all three new builtins:

```wisp
import "string"

fn main() -> int {
    let s: string = "hello"
    let rev: string = string.reverse(s)
    print(rev)                              // olleh
    print(string.chr(string.ord(string.char_at(s, 0))))          // h
    print(string.chr(string.ord(string.char_at(s, 4))))          // o
    return 0
}
// stdout: olleh\nh\no
```

## Numbers

Integer arithmetic uses the shell's native signed integers (`math.int_max()` /
`math.int_min()`). A `+`, `-`, or `*` whose true result falls outside that range
wraps, like C two's-complement arithmetic, and the wrapped value is the same on
every supported shell. Two integer operations instead abort (catchable, located,
identical on every shell) because their result is not representable at all:
division by zero, and the single overflowing division `math.int_min() / -1` (the
quotient `2^63` exceeds `math.int_max()`); `math.int_min() % -1` is `0` and does not abort.
The magnitude builtins below (`abs`, `gcd`, `lcm`) abort on `math.int_min()` for the
same reason.

### math.abs

```
math.abs(x: int) -> int
math.abs(x: float) -> float
```

Absolute value. The result type matches the argument type. `math.abs(math.int_min())`
aborts (catchable): the most-negative integer has no representable positive
counterpart, so taking its magnitude would overflow. This is the same located
abort, on every shell, as a division overflow.

```wisp
math.abs(-5)     // 5
math.abs(-2.5)   // 2.5
```

### math.min, math.max

```
math.min(a, b)   // both the same ordered scalar type
math.max(a, b)
```

The smaller or larger of two values. Both arguments must share one ordered scalar
type: int, float, bool, string, or a value enum (bool orders `false < true`, an
enum by its backing value). Mixing distinct types (int and float, or two different
enums) is a compile error. The result has the operand type -- for min/max there is
no coercion. Note the funcref *value* forms of `min`/`max` (passing `min` itself as
a function) stay int/float only, like `contains`/`index_of`.

```wisp
math.min(3, 7)        // 3
math.max(3.5, 7.5)    // 7.5
```

### math.clamp, math.sign

```
math.clamp(x, lo, hi)   // all int, or all float
math.sign(x) -> int     // x is int or float
```

`clamp` returns `x` confined to `[lo, hi]` (`math.max(lo, math.min(x, hi))`; all three the
same numeric type). It requires `lo <= hi`; this bound is a precondition, not a
checked error. If `lo > hi` the result is `lo` (the outer `max` wins), which is
defined but rarely what you want. `sign` returns `-1`, `0`, or `1` for a
negative, zero, or positive value (int or float; always returns int).

```wisp
math.clamp(15, 0, 10)   // 10
math.sign(-2.5)         // -1
```

### math.floor, math.ceil, math.round, math.trunc

```
math.floor(x: float) -> int
math.ceil(x: float) -> int
math.round(x: float) -> int
math.trunc(x: float) -> int
```

Round a float to an int: `floor` toward negative infinity, `ceil` toward positive
infinity, `round` to nearest with ties going up (`math.floor(x + 0.5)`), `trunc` toward
zero (the same as `to_int(float)`).

```wisp
math.floor(-1.5)   // -2
math.round(2.5)    // 3
```

### math.sqrt

```
math.sqrt(x: float) -> float
```

The non-negative square root. `sqrt` of a negative aborts (catchable). It is
computed with Newton's method in plain shell arithmetic, not awk's `sqrt()`,
because a minimal busybox awk is commonly built without math support. The result
is exact on perfect squares and within about one unit in the last place
otherwise. A result whose magnitude `%.17g` would render in exponent form (very
large or very small) is out of the wisp float domain and aborts, like any other
float op.

```wisp
math.sqrt(2.25)       // 1.5
math.sqrt(144.0)      // 12.0
```

### string.format_float

```
string.format_float(x: float, decimals: int) -> string
```

Renders `x` with exactly `decimals` digits after the decimal point, using awk's
`printf` f-conversion at the given precision. `decimals` must be `>= 0`; a
negative value is a located abort (catchable).

Unlike `to_string(float)`, which uses full `%.17g` precision and may produce
many digits or exponent notation, `format_float` produces a fixed-point
string with a predictable number of decimal places.

Rounding follows the platform `printf` (round-half-to-even on most libc
implementations). The result is NOT guaranteed half-up: an exact-halfway last
digit may differ across awk implementations.

Float-to-string formatting (here and in `to_string`, `sqrt`, `pow`, and the
other float builtins) is computed with `LC_ALL=C awk`, so the decimal
separator is always a period regardless of the caller's environment locale.

```wisp
string.format_float(3.14159, 2)   // "3.14"
string.format_float(1.0, 4)       // "1.0000"
string.format_float(2.5, 0)       // "2" or "3" (platform-dependent rounding)
```

```wisp
import "string"

fn main() -> int {
  print(string.format_float(3.14159, 2))
  return 0
}
// stdout: 3.14
```

### math.pow

```
math.pow(base: float, exp: float) -> float
```

Raises `base` to `exp`. The implementation uses only `+`, `-`, `*`, `/` in awk
(no `^`, no math library) so it is portable across minimal busybox builds.

- Integer exponent (including negative): exact repeated-squaring path.
- Fractional exponent on a positive base: `math.exp(e * math.ln(b))` via atanh-series `ln`
  and Taylor-series `exp`, both using only basic arithmetic.
- Negative base with a non-integer exponent: catchable abort.
- `0 ^ negative` and any result that would be non-finite: catchable abort.

```wisp
math.pow(2.0, 10.0)   // 1024.0
math.pow(4.0, 0.5)    // 1.9999999999999998  (computed via exp/ln; ~1 ULP off the exact 2.0 that math.sqrt(4.0) returns)
math.pow(-2.0, 3.0)   // -8.0
```

### math.exp

```
math.exp(x: float) -> float
```

Computes e^x (the natural exponential). The implementation uses only `+`, `-`,
`*`, `/` in awk (the same Taylor series that `pow` uses internally) so it is
portable across minimal busybox builds.

**Domain and overflow:** wisp floats are always finite; a result that would render
in exponent notation aborts located (catchable), exactly like any other out-of-range
float operation. For example, `math.exp(100.0)` (approximately 2.7e43) aborts because
the magnitude is outside the representable range. Values in roughly the range
`[-36, 36]` are safe; outside that range the result may abort.

**Euler's number:** there is no `e` constant builtin -- `e` is the conventional
error-binding name in `catch (e)` and is not reserved. Use `math.exp(1.0)` to obtain
Euler's number (approximately 2.718281828459045).

**Trigonometry:** `sin`, `cos`, `tan`, and inverse trig are not yet available
(deferred; awk trig is absent on no-libm busybox builds, so each needs a
pure-arithmetic implementation). Angle units are N/A until trig lands.

```wisp
math.exp(0.0)    // 1.0
math.exp(1.0)    // ~2.718281828459045  (Euler's number)
math.exp(-5.0)   // ~0.0067379469990855
```

### math.ln

```
math.ln(x: float) -> float
```

The natural logarithm. The implementation is pure arithmetic (no awk `log()`)
so it runs on busybox builds without math support.

**Domain:** `x` must be greater than 0. `math.ln(0.0)`, `math.ln(-1.0)`, and any
non-positive argument abort located (catchable) -- the same behavior as
`math.sqrt(-1.0)`. The domain guard fires immediately; there is no loop that can
hang on a zero or negative input.

**Trigonometry:** `sin`, `cos`, `tan`, and inverse trig are not yet available.
Angle units are N/A until trig lands.

```wisp
math.ln(1.0)            // 0.0
math.ln(math.exp(1.0))       // ~1.0  (round-trip through exp)
math.ln(math.exp(5.0))       // ~5.0
```

### math.log10

```
math.log10(x: float) -> float
```

Base-10 logarithm, computed as `math.ln(x) / math.ln(10)`. The same domain rule applies:
`x` must be positive; a non-positive argument aborts located (catchable).

```wisp
math.log10(1.0)      // 0.0
math.log10(10.0)     // ~1.0
math.log10(1000.0)   // ~3.0
```

### math.log2

```
math.log2(x: float) -> float
```

Base-2 logarithm, computed as `math.ln(x) / math.ln(2)`. The same domain rule applies:
`x` must be positive; a non-positive argument aborts located (catchable).

```wisp
math.log2(1.0)   // 0.0
math.log2(2.0)   // ~1.0
math.log2(8.0)   // ~3.0
```

### math.pi

```
math.pi() -> float
```

Returns the mathematical constant pi as the float literal
`3.141592653589793`. This is a compile-time constant with no awk invocation
at runtime. Note that `to_string(math.pi())` renders the full 17-significant-digit
`%.17g` form `3.1415926535897931` (the closest double to pi), not the shorter
source literal.

**Trigonometry:** `sin`, `cos`, `tan`, and inverse trig are not yet available
(deferred; see `exp` above). The primary use for `math.pi()` today is with `pow`
for geometry computations. Angle units (radians vs degrees) are N/A until
trig lands.

```wisp
math.pi()                  // 3.141592653589793 (the source literal)
to_string(math.pi())          // "3.1415926535897931" (full %.17g rendering)
math.pow(math.pi(), 2.0)        // ~9.869604401  (pi squared)
```

Runnable example using all five transcendentals:

```wisp
import "math"

fn main() -> int {
    let e_approx: float = math.exp(1.0)
    print("e ~ " + to_string(e_approx))
    let ln_e: float = math.ln(e_approx)
    print("math.ln(e) ~ " + to_string(ln_e))
    let l10: float = math.log10(1000.0)
    print("math.log10(1000) ~ " + to_string(l10))
    let l2: float = math.log2(8.0)
    print("math.log2(8) ~ " + to_string(l2))
    let circle_area: float = math.pi()
    print("pi ~ " + to_string(circle_area))
    return 0
}
```

### math.int_max, math.int_min

```
math.int_max() -> int
math.int_min() -> int
```

The largest and smallest signed integers the current shell can represent. Because
POSIX does not mandate a fixed integer width (32-bit and 64-bit shells are both
common), these are computed at runtime by doubling a value until it overflows.
As a mathematical identity, `math.int_min()` equals `-math.int_max() - 1`; however, the
expression `-math.int_max() - 1` in wisp requires shell `$(( ))` arithmetic at the
`2^63` boundary, which diverges on zsh (cannot represent `2^63`) and is
off-by-one on dash when the intermediate value is stored in a variable. Use
`math.int_min()` directly rather than computing it via arithmetic.

```wisp
math.int_max() > 1000000000   // true
math.int_min() < -1000000000  // true
```

### string.int_or, string.float_or

```
string.int_or(s: string, fallback: int) -> int
string.float_or(s: string, fallback: float) -> float
```

Parse `s` as an int (resp. float) like `int`/`float`, but return `fallback`
instead of aborting on any input the conversion would reject (invalid syntax or
out of range). The safe, non-aborting counterpart to the conversions.

```wisp
string.int_or("42", 0)     // 42
string.int_or("oops", -1)  // -1
```

### math.gcd, math.lcm

```
math.gcd(a: int, b: int) -> int
math.lcm(a: int, b: int) -> int
```

The non-negative greatest common divisor and least common multiple
(`math.gcd(0, 0)` is `0`; `lcm` is `0` when either argument is `0`). An operand equal
to `math.int_min()` aborts (catchable): its magnitude `2^63` is not representable, so
the result would overflow. The abort is the same on every shell. A very large
`lcm` that stays within range but exceeds `math.int_max()` follows the shell's integer
wraparound, like `*`.

```wisp
math.gcd(24, 36)   // 12
math.lcm(4, 6)     // 12
```

## Arrays

The only array builtins that mutate their argument in place are `push`, `pop`,
`remove_at`, and `insert_at`. Every other array builtin leaves its input
unchanged, returning either a fresh handle or a scalar value (for example
`contains -> bool`, `length -> int`).

### array.push

```
array.push(xs: T[], v: T) -> void
```

Appends `v` to `xs` in place and grows its length.

### array.contains, string.contains

```wisp
array.contains(xs: T[], x: T) -> bool
string.contains(xs: T[], x: T) -> bool
```

Membership by value. Defined only for arrays of int, bool, string, float, or
an enum type; a non-comparable element type, such as struct or function
reference, is a compile error. A float element compares by numeric identity
(`1.0` matches `1.00`; `-0.0` matches `0.0`).

```wisp
string.contains([1, 2, 3], 2)   // true
```

### array.index_of

```wisp
array.index_of(xs: T[], x: T) -> Optional[int]
```

Same as `string.index_of`'s array form (see the [Strings](#strings) section)
— returns the index of the first occurrence of `x` in `xs`, or `None` if not
found. `T` must be `int`, `bool`, `string`, `float`, or an enum type; a
`float` element compares by numeric identity.

### array.reverse

```
array.reverse(xs: T[]) -> T[]
```

Returns a new array with the elements reversed. The source is unchanged, and
any element type is allowed.

```wisp
array.reverse([1, 2, 3])   // [3, 2, 1]
```

### array.sort, array.sort_by

```
array.sort(xs: T[]) -> T[]              // T is any ordered scalar type
array.sort_by(xs: T[], less: fn(T, T) -> bool) -> T[]
```

`sort` returns a new array in ascending order: numeric for int and float, byte
order for string, `false` before `true` for bool, and backing order for a value
enum. The element type must be an ordered scalar (int, float, bool, string, or a
value enum); anything else is a compile error. `sort_by` returns a new array ordered by `less` (true means
the first argument sorts first) and works for any element type. Both are stable
and leave the source unchanged.

```wisp
array.sort([3, 1, 2])   // [1, 2, 3]
```

### array.find, array.any, array.all

```
array.find(xs: T[], f: fn(T) -> bool) -> Optional[int]
array.any(xs: T[], f: fn(T) -> bool) -> bool
array.all(xs: T[], f: fn(T) -> bool) -> bool
```

`find` returns `Some(i)` with the index of the first element where `f` is true,
or `None` when no element matches (see [Optional](/guide/language/#optional)). `any`
is true when `f` holds for at least one element (false for an empty array); `all`
is true when it holds for every element (true for an empty array). All three stop
at the first decisive element.

```wisp
unwrap_or(array.find([1, 3, 4], isEven), -1)   // 2
```

`isEven` here is a user-defined predicate (`fn isEven(n: int) -> bool`), not a builtin.

### array.slice, array.concat

```
array.slice(xs: T[], start: int, end: int) -> T[]
array.concat(a: T[], b: T[]) -> T[]
```

`slice` returns the elements in the half-open range `[start, end)`; an
out-of-range or inverted range aborts (catchable). `concat` returns a new array of
`a`'s elements followed by `b`'s. Neither changes its source.

```wisp
array.slice([10, 20, 30, 40], 1, 3)   // [20, 30]
```

### array.sum, array.range

```
array.sum(xs: int[]) -> int
array.sum(xs: float[]) -> float
array.range(n: int) -> int[]
```

`sum` adds the elements (an empty array is `0` / `0.0`). `array.range(n)` returns
`[0, 1, ..., n-1]`; `n` of zero or less is an empty array.

```wisp
array.sum([1, 2, 3])   // 6
array.range(4)         // [0, 1, 2, 3]
```

### array.first, array.last

```
array.first(xs: T[]) -> T
array.last(xs: T[]) -> T
```

The first or last element. An empty array aborts (catchable).

### array.count_where

```
array.count_where(xs: T[], f: fn(T) -> bool) -> int
```

Returns the number of elements for which `f` returns true. No short-circuit:
`f` is called for every element. An empty array returns `0`. A faulting
callback propagates the fault (catchable via `try`).

```wisp
array.count_where([1, 2, 3, 4], isEven)   // 2
```

`isEven` here is a user-defined predicate (`fn isEven(n: int) -> bool`), not a builtin.

### array.flatten

```
array.flatten(xs: T[][]) -> T[]
```

Concatenates all inner arrays in order into a single new array. The argument
must be an array of arrays; a non-array-of-arrays argument is a compile error.
Empty inner arrays contribute nothing. (When an inner array is an empty literal
`[]`, the argument must carry an element type so the empty literal can be
inferred, e.g. `let xs: int[][] = [[1, 2], [], [3]]`.)

```wisp
let xs: int[][] = [[1, 2], [], [3]]
array.flatten(xs)                  // [1, 2, 3]
```

### array.unique

```
array.unique(xs: T[]) -> T[]
```

Returns a new array containing each distinct element of `xs` at its first
occurrence, in order. Defined only for int, bool, string, float, or enum
elements; float elements are deduplicated by numeric identity. The source is
unchanged.

```wisp
array.unique([3, 1, 2, 1, 3])   // [3, 1, 2]
```

### array.take, array.drop

```
array.take(xs: T[], n: int) -> T[]
array.drop(xs: T[], n: int) -> T[]
```

`take` returns the first `n` elements; `drop` returns all but the first `n`.
Both clamp `n` to `[0, length(xs)]` and never abort. The source is unchanged.

```wisp
array.take([1, 2, 3, 4, 5], 3)   // [1, 2, 3]
array.drop([1, 2, 3, 4, 5], 2)   // [3, 4, 5]
```

### array.pop

```
array.pop(xs: T[]) -> T
```

Removes and returns the last element of `xs` in place, shrinking its length by
one. Aborts (catchable) when the array is empty.

```wisp
let xs: int[] = [1, 2, 3]
array.pop(xs)   // 3; xs is now [1, 2]
```

### array.remove_at

```
array.remove_at(xs: T[], i: int) -> void
```

Removes the element at index `i` from `xs` in place, left-shifting the tail to
close the gap and decrementing the length. Aborts (catchable) when `i` is
negative or `>= length(xs)`.

```wisp
let xs: int[] = [10, 20, 30]
array.remove_at(xs, 1)   // xs is now [10, 30]
```

### array.insert_at

```
array.insert_at(xs: T[], i: int, v: T) -> void
```

Inserts `v` at index `i` in `xs` in place, right-shifting the tail to open a
slot and incrementing the length. `i == length(xs)` is a valid append. Aborts
(catchable) when `i` is negative or `> length(xs)`.

```wisp
let xs: int[] = [10, 20, 30]
array.insert_at(xs, 1, 15)   // xs is now [10, 15, 20, 30]
```

## Dicts

`remove` and `clear` are the only dict builtins that mutate in place; the rest
return fresh handles or scalar values and leave the dict unchanged.

### dict.has

```
dict.has(d: {K: V}, k: K) -> bool
```

Tests whether a key is present. Use it before indexing, since reading a missing
key aborts.

### dict.keys

```
dict.keys(d: {K: V}) -> K[]
```

Returns a new array of keys in insertion order. The dict is unchanged.

```wisp
let m: {string: int} = { "a": 1, "b": 2 }
dict.keys(m)   // ["a", "b"]
```

### dict.values

```
dict.values(d: {K: V}) -> V[]
```

Returns a new array of values in insertion order, parallel to `keys` (the i-th
value goes with the i-th key). The dict is unchanged.

### dict.get_or

```
dict.get_or(d: {K: V}, k: K, fallback: V) -> V
```

Returns `d[k]` if `k` is present, otherwise `fallback`. Unlike indexing, it never
aborts on a missing key.

```wisp
let m: {string: int} = { "a": 1 }
dict.get_or(m, "a", 0)   // 1
dict.get_or(m, "z", 0)   // 0
```

### dict.get

```
dict.get(d: {K: V}, k: K) -> Optional[V]
```

Returns `Some(d[k])` if `k` is present, otherwise `None` (see
[Optional](/guide/language/#optional)). The `Optional`-returning counterpart to
`get_or`; use it when absence needs to be distinguished from a value rather than
folded into a fallback.

```wisp
let m: {string: int} = { "a": 1 }
unwrap_or(dict.get(m, "a"), 0)   // 1
is_none(dict.get(m, "z"))        // true
```

### dict.remove

```
dict.remove(d: {K: V}, k: K) -> void
```

Removes `k` from `d` in place; removing an absent key does nothing.

### dict.merge

```
dict.merge(a: {K: V}, b: {K: V}) -> {K: V}
```

Returns a new dict with `a`'s entries followed by `b`'s; on a shared key `b`'s
value wins but `a`'s position is kept. Both sources are unchanged.

```wisp
dict.merge({ "a": 1 }, { "a": 2, "b": 3 })   // { "a": 2, "b": 3 }
```

### dict.size

```
dict.size(d: {K: V}) -> int
```

Returns the number of keys in `d`. An empty dict returns `0`. Does not mutate.

```wisp
dict.size({ "a": 1, "b": 2 })   // 2
```

### dict.clear

```
dict.clear(d: {K: V}) -> void
```

Removes every entry from `d` in place, resetting its key list to empty. After
`clear`, `dict.size(d)` is `0` and `dict.keys(d)` is `[]`. A no-op on an already-empty
dict. Aliased handles observe the mutation.

```wisp
let d: {string: int} = { "a": 1, "b": 2 }
dict.clear(d)   // d is now {}
```

## Optional

`Optional[T]` is a value that is either `Some(x)` or `None`. The access builtins
are the only way to read the contained value; see the
[Optional language guide](/guide/language/#optional) for the full type, the `None`
rules, and the opacity (`to_string()`, interpolation, and use as a `switch` subject
are rejected). `==`/`!=` are the one exception: they are supported when the inner
type is comparable (`int`, `bool`, `string`, or a nested comparable `Optional`),
comparing structurally (equal iff both are `None`, or both are `Some` with equal
inner values). `Optional[float]` and `Optional` of non-comparable types (arrays,
dicts, structs, `error`, function references) stay non-comparable, so `==`/`!=` on
them remain compile errors.

### is_some, is_none

```
is_some(o: Optional[T]) -> bool
is_none(o: Optional[T]) -> bool
```

Total presence tests. `is_none` is the inverse of `is_some`.

### unwrap

```
unwrap(o: Optional[T]) -> T
unwrap(r: Result[T]) -> T
```

Returns the contained value if `o` is `Some` (or `r` is `Ok`), otherwise a
catchable located abort: `unwrap of None` for an empty `Optional`, or the carried
`error.message` for an `Err`. Overloaded across `Optional` and `Result`,
dispatched on the static argument type.

### unwrap_or

```
unwrap_or(o: Optional[T], fallback: T) -> T
unwrap_or(r: Result[T], fallback: T) -> T
```

Total: the value if `Some`/`Ok`, otherwise `fallback`. The `fallback` is evaluated
eagerly (wisp has no short-circuit in this position); it runs even when the
Optional/Result holds a value, so an effectful or faulting fallback expression
(e.g. `unwrap_or(o, xs[0])`) will execute -- and may abort -- regardless of the
carrier's state. For a lazily-computed fallback, use `or_else`, whose
`fn() -> Optional[T]` / `fn(error) -> Result[T]` callback runs only on the
empty/error branch. Overloaded across `Optional` and `Result`.

```wisp
let found: Optional[int] = array.find(xs, isEven)
if (is_some(found)) {
    print("at index " + to_string(unwrap(found)))
} else {
    print("not found")
}
let idx: int = unwrap_or(array.find(xs, isEven), -1)
```

Here `xs` is an `int[]` and `isEven` is a user-defined predicate (`fn isEven(n: int) -> bool`), not a builtin.

## Result

`Result[T]` is a value that is either `Ok(x)` or `Err(e)` (where `e` is the
built-in `error` handle). The access builtins are the only way to read the
contents; see the [Result language guide](/guide/language/#result) for the full type,
the `Ok`/`Err` rules, and the opacity.

### is_ok, is_err

```
is_ok(r: Result[T]) -> bool
is_err(r: Result[T]) -> bool
```

Total variant tests. `is_err` is the inverse of `is_ok`.

### unwrap_err

```
unwrap_err(r: Result[T]) -> error
```

Returns the carried `error` handle if `r` is `Err`, otherwise a located abort
`unwrap_err of Ok` (catchable via `try`/`catch`). `unwrap` (above) is the `Ok`-side
counterpart.

```wisp
let r: Result[int] = parse(line)
match (r) {
    case Ok(n) { print("parsed " + to_string(n)) }
    case Err(e) { print("error: " + e.message) }
}
```

## Combinators

Combinators let Optional and Result values flow through a pipeline without
unwrapping at each step. Each combinator invokes its function lazily: the
function is called only on the branch that needs it, so side effects in the
function fire only when the combinator activates.

`map` and `filter` are overloaded on the static type of the first argument.
`map` accepts arrays (existing behavior) or `Optional`/`Result`; `filter` accepts
arrays or `Optional` (it is not defined over `Result` -- see the matrix below).

| combinator | Optional | Result | callback shape | note |
|---|---|---|---|---|
| `map` | yes | yes | `fn(T)->U` (U != void) | uniform success-side transform |
| `and_then` | yes | yes | `fn(T)->Optional[U]` / `fn(T)->Result[U]` | uniform flat-map |
| `filter` | yes | no | `fn(T)->bool` | Optional-only: there is no Result "drop" without inventing an error; use `and_then` on Result |
| `or_else` | yes | yes | `fn()->Optional[T]` / `fn(error)->Result[T]` | arity differs: None carries no value; Err carries the error handle, passed to the callback |
| `map_err` | no | yes | `fn(error)->error` | Result-only: Optional has no error arm to transform |

### map (also array.map)

Callable flat as `map(o, f)` / `map(r, f)` when arg-0 is `Optional`/`Result`, or
namespaced as `array.map(...)` (both spellings accepted, including the array
form).

```
array.map(o: Optional[T], f: fn(T)->U) -> Optional[U]   U != void
array.map(r: Result[T],   f: fn(T)->U) -> Result[U]      U != void
array.map(xs: T[],        f: fn(T)->U) -> U[]             (existing array form)
```

`Some(x)` -> `Some(f(x))`; `None` -> `None` (f not called).
`Ok(x)` -> `Ok(f(x))`; `Err(e)` -> `Err(e)` unchanged (f not called).

### and_then

```
and_then(o: Optional[T], f: fn(T)->Optional[U]) -> Optional[U]
and_then(r: Result[T],   f: fn(T)->Result[U])   -> Result[U]
```

Flat-map (bind). `Some(x)` -> `f(x)` (f returns the monad); `None` -> `None`.
`Ok(x)` -> `f(x)`; `Err(e)` -> `Err(e)` unchanged.

### filter (also array.filter)

Callable flat as `filter(o, p)` when arg-0 is `Optional`, or namespaced as
`array.filter(...)` (both spellings accepted, including the array form).
filter over Result remains a compile error directing to `and_then`.

```
array.filter(o: Optional[T], f: fn(T)->bool) -> Optional[T]
array.filter(xs: T[],        f: fn(T)->bool) -> T[]          (existing array form)
```

`Some(x)` -> `Some(x)` if `f(x)` is true, else `None`; `None` -> `None`.
`filter` is not defined over `Result` (use `and_then` instead).

### or_else

```
or_else(o: Optional[T], f: fn()->Optional[T])    -> Optional[T]
or_else(r: Result[T],   f: fn(error)->Result[T]) -> Result[T]
```

`Some(x)` -> the same Optional (f not called); `None` -> `f()`.
`Ok(x)` -> the same Result (f not called); `Err(e)` -> `f(e)` (f receives the
error handle, so `.message` resolves on the bound parameter).

### map_err

```
map_err(r: Result[T], f: fn(error)->error) -> Result[T]
```

`Ok(x)` -> the same Result (f not called); `Err(e)` -> `Err(f(e))` with the
transformed error. `map_err` is not defined over `Optional`.

## Higher-order functions

Array combinators. Most (`map`, `filter`, `each`, ...) take a function reference
and apply it across an array; `zip` instead combines two arrays element-wise.

### array.zip

```
array.zip(a: T[], b: U[]) -> (T, U)[]
```

Pairs two arrays element-wise, up to the shorter length. Returns an array of 2-tuples. The extra tail of the longer array is dropped.

```wisp
let a: int[] = [1, 2, 3]
let b: string[] = ["a", "b"]
let z: (int, string)[] = array.zip(a, b)
print(to_string(z[0][0]))
print(z[0][1])
```

### array.map, array.filter, array.each

```
array.map(xs: T[], f: fn(T) -> U) -> U[]
array.filter(xs: T[], f: fn(T) -> bool) -> T[]
array.each(xs: T[], f: fn(T) -> void) -> void
```

`map` builds a new array by applying `f` to each element. `filter` keeps the
elements for which `f` returns true. `each` calls `f` for its side effect on
each element.

```wisp
fn dbl(n: int) -> int { return n * 2 }
array.map([1, 2, 3], dbl)   // [2, 4, 6]
```

### array.reduce

```
array.reduce(xs: T[], init: U, f: fn(U, T) -> U) -> U
```

Left fold. Starts from `init` and replaces the accumulator with `f(acc, elem)`
for each element in order. An empty array returns `init`.

```wisp
fn add(a: int, b: int) -> int { return a + b }
array.reduce([1, 2, 3, 4], 0, add)   // 10
```

## I/O

These builtins reach outside the program: the environment, the filesystem, other
commands, and the process exit status. I/O is fallible, so each fallible builtin
is a located abort that is catchable inside a `try` (like division by zero), and
the message names the subject that failed (the variable, the path, or `argv[0]`).
Every argument is inert data: a path, content, name, or argv element containing
`$(...)`, backticks, or globs is passed as data and never executed.

### env.get

```
env.get(name: string) -> string
```

Returns the value of the environment variable `name`. An unset variable is a
located abort naming the variable; a set-but-empty variable returns `""`. The
value is read through a command substitution, so trailing newlines are stripped
(the same as `run`); interior bytes are preserved. For exact trailing bytes, read
a file with `read_file` instead.

```wisp
env.get("HOME")   // "/home/me"
```

### env.has

```
env.has(name: string) -> bool
```

Reports whether `name` is set, including when it is set to the empty string. This
is the safe test to run before `env`, mirroring `has` for dicts.

```wisp
env.has("PATH")   // true
```

### fs.read_file

```
fs.read_file(path: string) -> string
```

Returns the entire contents of the file at `path`, **exactly**, including every
trailing newline. Unlike `run` and `env`, no trailing bytes are stripped. A
missing or unreadable file is a located abort naming the path. A file containing
a NUL byte is unsupported (wisp strings cannot hold NUL) and aborts located.

```wisp
fs.read_file("config.txt")
```

### fs.write_file

```
fs.write_file(path: string, content: string) -> void
```

Writes `content` to `path`, truncating an existing file or creating a new one.
`content` is written exactly, with no added newline. A write failure is a located
abort naming the path. Writes are **not atomic**: they use ordinary shell
redirection, so a write that fails partway (for example, a full disk) may leave a
truncated file.

```wisp
fs.write_file("out.txt", "data")
```

### fs.append_file

```
fs.append_file(path: string, content: string) -> void
```

Like `write_file` but appends to the file rather than truncating, creating it if
absent. Also non-atomic.

```wisp
fs.append_file("log.txt", "another line\n")
```

### read_line

```
read_line() -> Optional[string]
```

Reads one line from standard input and strips the trailing newline. Returns
`Some(line)` when a line is available, including an empty line (`"\n"` yields
`Some("")`). Returns `None` only at EOF with no data to return. A partial line
with no trailing newline -- for example the last line when stdin ends without `\n`
-- is returned as `Some` of the content read, not `None`. Calling `read_line` after
EOF continues to return `None`.

```wisp
let line: Optional[string] = read_line()
while (is_some(line)) {
  print(unwrap(line))
  line = read_line()
}
```

### read_stdin

```
read_stdin() -> string
```

Reads all of standard input and returns it as a single string. Every byte is
preserved exactly, including any trailing newlines (no stripping, unlike `run`).
An empty stdin returns `""`. This buffers the entire input in memory; for
line-at-a-time processing prefer `read_line`.

```wisp
let all: string = read_stdin()
```

### set_stdin

```
set_stdin(content: string) -> void
```

Replaces standard input with `content`, byte-for-byte, discarding whatever was
there before. No newline is added: `set_stdin("yes\n")` feeds a line for
`read_line`, `set_stdin("yes")` feeds the same text with no trailing newline
(`read_line` still returns `Some("yes")` at EOF). Subsequent `read_line`,
`read_stdin`, or anything else that reads fd 0 sees exactly `content`. A later
`set_stdin` call fully replaces an earlier one. A fault creating the internal
buffer (e.g. `mktemp` failing) is a located abort.

`set_stdin` is scoped to the current shell or subshell: the test runner
executes each `test(...)` body in its own subshell, so `set_stdin` in one test
does not affect the next. This makes it the way to exercise a confirmation
prompt's "yes" branch under `wisp test`, where a function like `confirm` below
reads the answer via `read_line`:

```wisp
fn confirm(prompt: string) -> bool {
  print(prompt)
  let answer: Optional[string] = read_line()
  return is_some(answer) && unwrap(answer) == "yes"
}

test ("declines when the answer is not yes") {
  set_stdin("no\n")
  assert_eq(confirm("Proceed?"), false)
}

test ("accepts yes") {
  set_stdin("yes\n")
  assert_eq(confirm("Proceed?"), true)
}
```

```wisp
set_stdin("hello\n")
print(unwrap(read_line())) // hello
```

### read_secret

```
read_secret(prompt: string) -> Optional[string]
```

Prints `prompt` to stderr, reads one line from standard input with terminal echo
suppressed (`stty -echo`), restores echo, then returns `Some(line)` with the
trailing newline stripped, or `None` at EOF. The prompt appears before the read,
so the user sees it even though their typing is invisible.

Echo suppression is best-effort: when stdin is not a terminal (a pipe, a file, or
a CI environment) the `stty` call is a silent no-op and the line is still read
normally. The function is safe in pipelines and automated scripts; it never aborts
because `stty` failed.

**Caveats:** a SIGINT (Ctrl-C) received during the read may leave the terminal
with echo off. To recover, type `stty echo` blind and press Enter. It reads a
whole line with echo suppressed; single-keypress raw-mode reads are not
supported.

Contrast with `read_line`: `read_line` reads without a prompt and with echo on.

```wisp
match (read_secret("Password: ")) {
  case Some(pw) { print("got " + to_string(length(pw)) + " chars") }
  case None { print("no input") }
}
```

### process.run

```
process.run(argv: string[]) -> string
```

Runs a command and returns its standard output. `argv[0]` is the program; the
remaining elements are its arguments, each passed as a separate word with no
shell interpretation. The program is found via the normal `PATH` search. Note
that the shell's command resolution finds its own builtins (such as `echo`)
before searching `PATH`, so a builtin may shadow a `PATH` binary of the same
name.

An empty `argv` is a located abort. A command that cannot be executed (surfacing
as status 127) or that exits with any nonzero status is a located abort whose
message includes `argv[0]` and the status; wrap the call in a `try` to handle a
command that may fail. Only standard output is captured; standard error passes
through to the script's own standard error. As with shell command substitution,
trailing newlines are stripped from the captured output and interior newlines are
preserved (this differs from `read_file`, which preserves exact bytes).

```wisp
process.run(["echo", "hello"])   // "hello"
```

### exit

```
exit(code: int) -> void
```

Terminates the program immediately with `code` as the process exit status. It is
**not** a fault: it is not catchable and it does **not** run any pending
`finally`. For that reason, calling `exit` inside a `try`, `catch`, or `finally`
body is a compile error; a program that needs cleanup to run should return from
`main` instead. The restriction is lexical, so `exit` reached through a function
that is itself called from a try body still terminates at once without running
`finally`.

```wisp
exit(1)
```

## Filesystem and process

These build on the whole-file `read_file`/`write_file` and the aborting `run`
with the directory, predicate, and process operations real scripts need. Every
path or name is passed to its command as a single quoted argument and is never
re-evaluated as shell, so a path containing spaces, glob characters, a leading
`-`, or `$(...)`/backticks is treated as literal data.

### fs.file_exists, fs.is_dir

```
fs.file_exists(path: string) -> bool
fs.is_dir(path: string) -> bool
```

`file_exists` is true if `path` exists as any file type (a regular file, a
directory, or a symlink to an existing target). `is_dir` is true only if `path`
exists and is a directory (following a symlink). Both are total: a non-existent
path yields `false`, never a fault.

```wisp
fs.file_exists("config.toml")   // true or false
fs.is_dir("src")                // true or false
```

### fs.make_dir

```
fs.make_dir(path: string) -> void
```

Creates `path`, including any missing parent directories (`mkdir -p`). Creating
a directory that already exists is a success. A genuine failure -- a path
component is a non-directory, or permission is denied -- is a located abort.
`make_dir` is **not** transactional: a failure may leave some parent
directories already created.

```wisp
fs.make_dir("build/out/cache")
```

### fs.remove_file

```
fs.remove_file(path: string) -> void
```

Removes the file at `path`. Removing a path that does not exist is a success
(there is nothing to do). A real failure -- the path is a directory, or its
parent is unwritable -- is a located abort; the convenience of the no-op-on-
missing case never masks a genuine error.

```wisp
fs.remove_file("tmp.lock")
```

### fs.remove_dir

```
fs.remove_dir(path: string) -> void
```

Removes the **empty** directory at `path` (`rmdir`). A missing path, a
non-directory, or a non-empty directory is a located abort. There is
intentionally no recursive remove: a recursive delete is a footgun that, if
ever added, must read dangerously at the call site.

```wisp
fs.remove_dir("build/empty")
```

### fs.rename

```
fs.rename(from: string, to: string) -> void
```

Renames or moves `from` to `to` (`mv`). A failure -- the source is missing, a
cross-device move is unsupported, or permission is denied -- is a located abort.

```wisp
fs.rename("out.tmp", "out.txt")
```

### fs.cwd

```
fs.cwd() -> string
```

Returns the process's current working directory as an absolute path. Takes no
argument.

```wisp
let here: string = fs.cwd()
```

### fs.change_dir

```
fs.change_dir(path: string) -> void
```

Changes the process's current working directory to `path`. A nonexistent path, a
non-directory (including a regular file), or a path the process lacks permission
to enter is a located abort naming `change_dir`; wrap in a `try` to handle failure.

**Footgun:** `change_dir` mutates global process state. It affects every subsequent
relative path in the entire program, including paths used by `run_full`, `run`,
`read_file`, and all other builtins. There is no scoped restore -- the directory
stays changed for the process lifetime unless `change_dir` is called again.

```wisp
fs.change_dir("build")
try {
  fs.change_dir("/no/such/path")
} catch (e) {
  print("caught")
}
```

### fs.program_path

```
fs.program_path() -> string
```

Returns the program's invocation path: the value of the shell `$0` as the
script was invoked. It is captured once at the script's top level before
`main` runs, so it is the same value whether called from `main` or any nested
function, and identical on dash, busybox ash, bash, and zsh.

Semantics by entry point:

- A built artifact run directly (`./prog.sh`, `sh prog.sh`, or `prog.sh` on
  `PATH`): `fs.program_path()` is `$0` as invoked -- the relative or absolute path
  the shell received.
- Under `wisp run prog.wisp`: the program runs as a temporary script, so
  `fs.program_path()` is that temporary path. It is the honest invocation path;
  `fs.program_path()` is most useful for built artifacts.
- Sourcing a wisp artifact is not a supported entry point. Cross-shell `$0`
  semantics when sourcing differ and are unspecified.

The `$0` capture line is emitted only when `program_path` is reachable, so a
program that never calls it carries no capture and no added overhead.

```wisp
let dir: string = fs.dir_name(fs.program_path())
```

### fs.dir_name

```
fs.dir_name(path: string) -> string
```

Returns the directory portion of `path`, with POSIX `dirname`-style semantics,
as a pure string function (no external `dirname` process, no I/O).

| input        | result |
|--------------|--------|
| `/a/b/c`     | `/a/b` |
| `/a/b/c/`    | `/a/b` |
| `a/b`        | `a`    |
| `b` (no `/`) | `.`    |
| `/`          | `/`    |
| `//`         | `/`    |
| `` (empty)   | `.`    |
| `/a`         | `/`    |

```wisp
fs.dir_name("/usr/local/bin/script.sh")   // "/usr/local/bin"
fs.dir_name("script.sh")                  // "."
```

### fs.base_name

```
fs.base_name(path: string) -> string
```

Returns the final component of `path`, with POSIX `basename`-style semantics,
as a pure string function (no external `basename` process, no I/O). Trailing
slashes are stripped before the component is extracted.

| input        | result |
|--------------|--------|
| `/a/b/c`     | `c`    |
| `/a/b/c/`    | `c`    |
| `a/b`        | `b`    |
| `b` (no `/`) | `b`    |
| `/`          | `/`    |
| `//`         | `/`    |
| `` (empty)   | ``     |
| `/a`         | `a`    |

```wisp
fs.base_name("/usr/local/bin/script.sh")   // "script.sh"
fs.base_name("script.sh")                  // "script.sh"
```

### fs.which

```
fs.which(name: string) -> Optional[string]
```

Resolves a command name to its path, returning `Some(path)` when found and
`None` when not. It never aborts -- absence is `None`, not a fault. Note that
for a shell builtin or keyword, the underlying `command -v` may return the name
itself rather than a filesystem path, which `which` reports as `Some` of that
text.

```wisp
fs.which("sh")                          // Some("/bin/sh") on a typical system
unwrap_or(fs.which("sh"), "/bin/sh")    // the resolved path, or the fallback
fs.which("definitely_missing")          // None
```

### env.get_or

```
env.get_or(name: string, fallback: string) -> string
```

The non-aborting twin of `env`: returns the value of environment variable
`name` if it is **set** (including when set to the empty string), otherwise
`fallback`. Only an unset variable yields the fallback. The name flows into the
environment lookup as data and is never re-evaluated.

```wisp
env.get_or("EDITOR", "vi")
```

### env.set

```
env.set(name: string, value: string) -> void
```

Exports `name=value` into the wisp process's environment. Every subsequent
child process -- `run`, `spawn`, `exec_command`, `run_env*`, and any other
command launched after this call -- inherits the assignment.

`name` must match the POSIX env-name grammar `[A-Za-z_][A-Za-z0-9_]*`. An
empty name, a name starting with a digit, or a name containing `=` or any
other non-conforming byte is a located abort naming `set_env`. `value` is
arbitrary inert data and is never re-evaluated.

**Footgun:** `set_env` mutates global process state. It affects every
subsequent child in the entire program. There is no scoped restore -- the
assignment persists for the process lifetime unless changed again with
`set_env` or removed with `unset_env`. Prefer `run_env` when the override
should apply to one child only and must not touch the wisp process environment.

```wisp
import "env"

fn main() -> int {
  env.set("DEPLOY_ENV", "staging")
  print(env.get_or("DEPLOY_ENV", "unset"))
  env.unset("DEPLOY_ENV")
  print(env.get_or("DEPLOY_ENV", "unset"))
  return 0
}
// stdout: staging
// stdout: unset
```

### env.unset

```
env.unset(name: string) -> void
```

Removes `name` from the wisp process's environment. If `name` is not
currently set the call succeeds silently (no-op). Every subsequent child
process no longer inherits the variable.

`name` must match `[A-Za-z_][A-Za-z0-9_]*`. An invalid name is a located
abort naming `unset_env`. Same name validation as `set_env`.

**Footgun:** same global-state caveat as `set_env` -- the removal is permanent
for the process lifetime.

```wisp
env.unset("DEPLOY_ENV")
```

### fs.list_dir

```
fs.list_dir(path: string) -> string[]
```

Returns the entries of a directory as the **base names** of its contents,
including hidden (dotfile) entries and excluding `.` and `..`. The order is
unspecified (it follows the shell's glob order); call `array.sort()` on the result
when a deterministic order is needed. A missing path or a non-directory is a
located abort; an empty directory returns `[]`. Names with spaces, glob
metacharacters, or a broken symlink are handled correctly.

```wisp
array.sort(fs.list_dir("src"))        // ["a.wisp", "b.wisp", ...]
```

### fs.is_file, fs.is_symlink

```
fs.is_file(path: string) -> bool
fs.is_symlink(path: string) -> bool
```

`is_file` is true if `path` is a regular file (or a symlink to one -- it
follows symlinks, identical to `test -f`). `is_symlink` is true if `path`
itself is a symbolic link, regardless of what the link points to. Both are
total: a missing or inaccessible path returns `false`, never a fault.

```wisp
fs.is_file("config.toml")      // true if it exists as a regular file or a symlink to one
fs.is_symlink("config.toml")   // true only if config.toml itself is a symlink
```

### fs.file_size

```
fs.file_size(path: string) -> int
```

Returns the byte size of the file at `path`. Implemented via `wc -c <
"$path"` (the only portable cross-platform size method; `stat` syntax
differs across GNU, BSD, and busybox). Leading whitespace from `wc`'s output
is stripped before the value is used. A missing path, a directory, or an
unreadable file is a located abort naming `file_size`. The path flows as data
through shell redirection, so a leading `-` in the path is safe.

```wisp
let n: int = fs.file_size("archive.tar.gz")
```

### fs.read_link

```
fs.read_link(path: string) -> Optional[string]
```

Returns the immediate target of the symlink at `path` as `Some(target)`, or
`None` if `path` is not a symlink or does not exist. Implemented via
`readlink -- "$path"` (the `--` guards a leading-dash path). Single-level
only: the result is the stored target string, not canonicalized. Any failure,
including a permission error or a missing path, maps to `None` rather than a
fault. A trailing newline on the target is stripped by shell `$(...)` capture
(filenames do not end in newlines in practice); a non-trailing newline within
a target is preserved verbatim.

`readlink` is not strict POSIX but is present on every target wisp supports
(GNU coreutils, busybox, and macOS BSD).

```wisp
match (fs.read_link("version.ln")) {
  case Some(t) { print(t) }
  case None { print("not a symlink") }
}
```

### fs.chmod

```
fs.chmod(path: string, mode: string) -> void
```

Changes the permissions of `path` to `mode`. The mode is an octal or
symbolic string (`"755"`, `"u+x"`, `"go-w"`) passed directly to `chmod`.
Implemented as `chmod -- "$mode" "$path"` (a single `--` ends option parsing
so a leading-dash `path` is safe). A missing path or an invalid mode is a
located abort. `chmod` is statement-only (Void) and cannot appear in a `let`
binding.

The exact resulting permission bits are not verified by cross-shell tests
because CI typically runs as root, which bypasses mode-bit enforcement;
the testable contract is that a valid chmod completes without error and an
invalid one aborts located.

```wisp
fs.chmod("script.sh", "755")
fs.chmod("data.txt", "u+r,go-rwx")
```

### fs.symlink

```
fs.symlink(target: string, link_path: string) -> void
```

Creates `link_path` as a symbolic link pointing to `target`. Implemented as
`ln -s -- "$target" "$link_path"` (a single `--` guards both operands). The
`target` is stored verbatim; it does not need to exist (dangling links are
permitted, as with `ln -s`). If `link_path` already exists, the call is a
located abort. `symlink` is statement-only (Void).

```wisp
fs.symlink("../shared/lib.so", "lib.so")
```

### fs.symlink_force

```
fs.symlink_force(target: string, link_path: string) -> void
```

Creates or replaces `link_path` as a symbolic link pointing to `target`.
If `link_path` already exists as a symlink (including one that points at a
directory) or a regular file, it is removed and re-created. If `link_path`
is a real directory (not a symlink), the call is a located abort -- it will
not clobber a directory. `target` is stored verbatim; a dangling target is
allowed (like `symlink`).

NOT atomic and DESTRUCTIVE-ON-FAILURE: `symlink_force` removes the old
`link_path` first, then calls `ln -s`. If the relink fails after the
removal, `link_path` is gone -- identical to the behavior of `ln -sf`.
When replacing a regular file, the file is lost if the relink fails.
Use `symlink` when `link_path` must never already exist.

```wisp
import "fs"

fn main() -> int {
  fs.symlink_force("/opt/app/current", "applink")
  print(unwrap_or(fs.read_link("applink"), "none"))
  return 0
}
// stdout: /opt/app/current
```

### fs.glob

```
fs.glob(pattern: string) -> string[]
```

Expands `pattern` using the shell's pathname expansion and returns the
matching paths as an array. If nothing matches, returns an empty array (not
the literal pattern string). Total: no match is success, not a fault.

Matched filenames are stored as inert data -- names containing `$(...)`,
backticks, semicolons, spaces, or embedded newlines round-trip verbatim
without being re-evaluated.

The pattern undergoes word-splitting before globbing, so a pattern with an
embedded space is treated as two separate globs. This matches shell semantics
and is the intended contract. The ordering of results follows the shell's
pathname expansion order; call `array.sort()` on the result if a deterministic
order is needed.

Cross-shell parity on zsh depends on the emulate-sh banner wisp emits at the
top of every compiled script (`ZSH_VERSION -> emulate sh / setopt
shwordsplit`), which disables zsh's `NOMATCH` error and restores word-
splitting. Glob patterns that match only on some shells are not a wisp
concern.

```wisp
let txt: string[] = fs.glob("src/*.txt")
print(to_string(length(txt)))
```

### fs.temp_file, fs.temp_dir

```
fs.temp_file() -> string
fs.temp_dir() -> string
```

`temp_file` creates a new empty temporary file via `mktemp` and returns its
path. `temp_dir` creates a new temporary directory via `mktemp -d` and
returns its path. Both are located: a failure from `mktemp` is a located
abort. Caller owns cleanup -- remove the file with `remove_file` and the
directory with `remove_dir` when done.

`mktemp` is not strict POSIX but is present on every target wisp supports.
No template or suffix control (uses the system default `$TMPDIR`/`/tmp`
prefix).

`mtime` (modification time as an epoch integer) is a documented non-goal: no
portable single source exists across GNU (`stat -c%Y`), BSD (`stat -f%m`),
and busybox; use `process.run(["stat", ...])` platform-specifically if needed.

```wisp
let f: string = fs.temp_file()
fs.write_file(f, "scratch data")
// ... use f ...
fs.remove_file(f)

let d: string = fs.temp_dir()
fs.write_file(d + "/out.txt", "result")
// ... use d ...
fs.remove_file(d + "/out.txt")
fs.remove_dir(d)
```

### Filesystem metadata example

A complete program using the nine new builtins. All printed values are
deterministic booleans; the temp paths and file sizes are not printed
directly.

```wisp
import "fs"

fn main() -> int {
  let d: string = fs.temp_dir()
  let f: string = d + "/hello.txt"
  fs.write_file(f, "abc")
  print(to_string(fs.file_size(f) == 3))
  let hits: string[] = fs.glob(d + "/*.txt")
  print(to_string(length(hits) == 1))
  print(to_string(fs.is_file(f)))
  print(to_string(fs.is_symlink(f)))
  let lnk: string = d + "/link.txt"
  fs.symlink(f, lnk)
  print(to_string(fs.is_symlink(lnk)))
  fs.chmod(f, "644")
  print(to_string(fs.is_file(f)))
  fs.remove_file(lnk)
  fs.remove_file(f)
  fs.remove_dir(d)
  return 0
}
```

### process.run_status

```
process.run_status(argv: string[]) -> int
```

Like `run`, but returns the command's exit code as an `int` and does **not**
abort on a nonzero exit. Unlike `run`, it does not capture output: the child's
standard output and standard error pass through to the script's own. The child
also inherits the script's standard input (unlike `run_full`, which connects the
child's stdin to `/dev/null`). An empty `argv` is a located abort (the only abort
it performs); a nonzero child status is returned, not aborted.

```wisp
let code: int = process.run_status(["grep", "-q", "needle", "file"])
```

### process.run_full

```
process.run_full(argv: string[]) -> RunResult
```

Runs a command and captures its standard output, standard error, and exit code.
Unlike `run` and `run_status`, the child's stdin is connected to `/dev/null` so the
child cannot read from the script's stdin. `argv[0]` is the program; the remaining
elements are its arguments, passed without shell interpretation. An empty `argv` is
a located abort; a nonzero child exit code is **not** an abort -- it is accessible
on the result.

The return type `RunResult` is an opaque handle with three fields:
- `.stdout`: captured standard output as a string; trailing newlines are **not**
  stripped (unlike `run`)
- `.stderr`: captured standard error as a string; also not stripped
- `.code`: the exit code as an `int`

`RunResult` cannot be compared with `==` or `!=`, converted with `to_string()`, used
as a `switch` subject, or passed to builtins that require a comparable value. Access
it only through its fields. To render a `RunResult` for debugging, use `debug(r)`.

```wisp
let r: RunResult = process.run_full(["grep", "-r", "TODO", "."])
if (r.code == 0) {
  process.run_status(["printf", "%s", r.stdout])
} else {
  print("no TODOs found")
}
```

### process.run_input

```
process.run_input(argv: string[], stdin: string) -> string
```

Runs a command with `stdin` fed to its standard input and returns its standard
output. `argv[0]` is the program; the remaining elements are its arguments,
passed without shell interpretation. The input is fed via `printf %s "$stdin"`,
so the bytes of `stdin` are delivered exactly with no added trailing newline.

An empty `argv` is a located abort. A command that exits with any nonzero
status is a located abort naming `argv[0]` and the status; wrap the call in a
`try` to handle a command that may fail. Only standard output is captured;
trailing newlines are stripped, as with `run`.

`run_input` is the single-filter ergonomic form: feed one string to one command
and get its stdout. For a multi-stage pipeline such as feeding a string through
several commands in sequence, use `process.pipe([["printf", "%s", s], argv, ...])`.

```wisp
import "process"

fn main() -> int {
  print(process.run_input(["cat"], "hello"))
  return 0
}
// stdout: hello
```

### process.run_input_full

```
process.run_input_full(argv: string[], stdin: string) -> RunResult
```

Like `run_input`, but returns a `RunResult{stdout, stderr, code}` without
aborting on a nonzero child exit. The same stdin feed applies: `stdin` is
delivered via `printf %s` as exact bytes with no added trailing newline.

An empty `argv` is a located abort. A nonzero child exit code is **not** an
abort -- it is accessible on `result.code`.

The `RunResult` fields are the same as for `run_full`:
- `.stdout`: captured standard output; trailing newlines are **not** stripped
- `.stderr`: captured standard error; also not stripped
- `.code`: the exit code as an `int`

```wisp
let r: RunResult = process.run_input_full(["cat"], "hello")
print(to_string(r.code == 0))
print(r.stdout)
```

### process.run_env

```
process.run_env(argv: string[], env: {string: string}) -> string
```

Like `run`, but runs the command with a custom (augmented) child environment.
The `env` dict's entries are added to (or override) the child's environment;
the child still inherits every variable from the parent that is not named in
the dict. For a supported `argv[0]`, `process.run_env(argv, {})` behaves identically
to `process.run(argv)`.

An empty `argv` is a located abort. A command that exits with any nonzero
status is a located abort naming `argv[0]` and the status. Only standard
output is captured; trailing newlines are stripped as with `run`.

**Environment name validation.** Each key in `env` must satisfy the POSIX
env-name grammar `[A-Za-z_][A-Za-z0-9_]*`. A key that is empty, starts with a
digit, or contains whitespace, `=`, a newline, or any other non-conforming byte
is a located abort (`run_env: invalid environment variable name: <name>`).
Values are arbitrary (any byte sequence is accepted).

**argv[0] limitation.** A command name starting with `-` or containing `=` is
not supported: without a `--` terminator, `env` parses a leading-dash name as
an option and an `=`-containing name as an assignment. Use a path-qualified
form (`./cmd` or an absolute path). This is a documented limitation, not a
checked error; the effect surfaces at runtime through the normal exit-code path.

**Non-goals.** No clean-slate environment (`env -i`); no env-var unset.

**Dependency.** Uses the `env` utility (POSIX; present on every target).

```wisp
import "process"

fn main() -> int {
  let result: string = process.run_env(["sh", "-c", "printf %s \"$GREETING\""], {"GREETING": "hi"})
  print(to_string(result == "hi"))
  return 0
}
```

### process.run_env_status

```
process.run_env_status(argv: string[], env: {string: string}) -> int
```

Like `run_status`, but runs the command with a custom (augmented) child
environment. Returns the child's exit code without aborting on a nonzero
result. Name validation and the empty-argv check are the same as `run_env`;
those abort located before the command runs.

```wisp
let code: int = process.run_env_status(["sh", "-c", "exit \"$RC\""], {"RC": "3"})
```

### process.run_env_full

```
process.run_env_full(argv: string[], env: {string: string}) -> RunResult
```

Like `run_full`, but runs the command with a custom (augmented) child
environment. Returns a `RunResult{stdout, stderr, code}` without aborting on
a nonzero exit. Name validation and the empty-argv check are the same as
`run_env`; those abort located before the command runs.

```wisp
let r: RunResult = process.run_env_full(["sh", "-c", "printf %s \"$MSG\""], {"MSG": "ok"})
print(to_string(r.code == 0))
```

### process.exec_command

```
process.exec_command(argv: string[]) -> void
```

Replaces the current shell process with `argv` via the POSIX `exec` builtin.
The spawned program becomes the current process with the same PID; the shell is
gone. It never returns on success: any code after `process.exec_command(...)` is
unreachable at runtime. This is a runtime fact, not a compile-time error -- the
checker does not flag the following statement as unreachable.

Unlike `run` and `spawn`, `exec_command` performs no redirection and no output
capture. The exec'd program inherits the script's stdin, stdout, stderr, all
other open file descriptors, and signal state unchanged. Its exit status becomes
the script's exit status.

An empty `argv` (`process.exec_command([])`) is a located abort. On exec failure (command
not found or not executable) the shell exits with its native diagnostic and status
127 or 126 -- this is NOT a wisp located abort, since exec does not return.

The primary use case is the container-entrypoint handoff: run setup steps, then
hand off to the real long-running process as PID 1. Illustrative fragment (the
call after setup; nothing after it runs):

```wisp
// ... setup steps ...
process.exec_command(["the-server", "--port", "8080"])
// never reached: the shell process is now the-server
```

Runnable smoke example -- exec `true` as the final statement (exits 0):

```wisp
import "process"

fn main() -> int {
  print("before exec")
  process.exec_command(["true"])
  return 0
}
// stdout: before exec
```

## Signals and traps

Register a wisp function to run when the process receives a named signal or
exits. These are exit and signal HOOKS for cleanup and graceful shutdown -- not
full process supervision. Forwarding signals to or reaping a long-running child
needs background processes and `wait` (a future feature); wisp delivers the hooks.

Both builtins lower to the shell `trap` builtin. The handler is a user-declared
`fn() -> void`; its funcref lowers to its compiler-assigned mangled shell-function
name (`__wisp_f_m0_<name>`), an inert `[A-Za-z0-9_]`-only word. No user data
enters any trap action -- the action is injection-safe by construction.

**Replace semantics.** POSIX `trap` allows one action per signal or EXIT. A
later `on_signal(sig, h2)` for the same `sig` REPLACES the earlier handler; a
later `on_exit(h2)` REPLACES the earlier EXIT handler. There is no accumulation.
To run multiple cleanup steps, compose one handler `fn` that calls them in order.

**Re-entrancy caveat.** A signal may arrive mid-statement (for example while an
error is propagating). The handler runs against possibly-partial program state.
Keep signal handlers self-contained.

### on_exit

```
on_exit(handler: fn() -> void) -> void
```

Registers `handler` to run when the process exits: on normal completion, an
explicit `exit(n)`, or a located abort. The handler is called with the original
exit code already preserved; the process exits with that same code after the
handler returns.

`on_exit` does **not** fire when the process is killed by an uncaught terminating
signal (for example an unhandled `TERM`). To run cleanup on a signal, register
`on_signal(sig, h)` and call `exit(n)` from `h` -- the `exit` fires the EXIT
trap and thus the `on_exit` handler.

**Exit-code preservation.** The emitted trap action captures `$?` before calling
the handler and restores it afterward (`__wisp_ec=$?; <handler>; exit "$__wisp_ec"`).
The handler's last command does not affect the final exit code. An on_exit handler
should perform side-effect cleanup, not set the exit code. To set a specific exit
code, call `exit(n)` in `main`; calling `exit` or triggering an abort inside the
handler has shell-dependent behavior across dash/busybox ash/bash/zsh.

**The handler must be a user-declared `fn() -> void`.** No builtin has a
`fn() -> void` signature (referenceable builtins take exactly one scalar argument),
so passing a builtin as the handler is always a compile error. A handler with
parameters or a non-void return type is a compile error.

```wisp
on_exit(cleanup)
```

Runnable example -- register a cleanup that prints a message, do some work, exit 0:

```wisp
fn cleanup() -> void {
  print("cleanup done")
}
fn main() -> int {
  on_exit(cleanup)
  print("working")
  return 0
}
```

### on_signal

```
on_signal(sig: string, handler: fn() -> void) -> void
```

Registers `handler` to run when the process receives the signal named `sig`.
After the handler returns, execution **resumes** where it was interrupted -- a
caught signal does not terminate the process. To terminate on a signal (the
graceful-shutdown pattern), the handler must call `exit(n)` explicitly.

`sig` must be a **string literal** from the supported set:
`"INT"`, `"TERM"`, `"HUP"`, `"QUIT"`, `"USR1"`, `"USR2"`.
A non-literal or computed `sig` is a compile error. An unsupported literal
(`"EXIT"`, `"KILL"`, `"STOP"`, `"BOGUS"`, `""`, and so on) is a compile error.
`EXIT` is not accepted here -- use `on_exit`. `KILL` and `STOP` are untrappable
by POSIX. Signal names are case-sensitive and must be given without the `SIG`
prefix.

**Composition.** Calling `exit(n)` inside an `on_signal` handler triggers any
registered `on_exit` handler. The canonical shutdown pattern:

```wisp
fn cleanup() -> void {
  print("cleanup done")
}
fn shutdown() -> void {
  print("shutting down")
  exit(0)
}
on_exit(cleanup)
on_signal("TERM", shutdown)
```

When `TERM` is received, `shutdown` runs, its `exit(0)` fires the EXIT trap, and
`cleanup` runs before the process exits 0.

**The handler must be a user-declared `fn() -> void`.** Same constraint as
`on_exit`.

```wisp
on_signal("TERM", shutdown)
on_signal("INT", shutdown)
on_signal("USR1", handler)
```

## Background processes

Launch a command in the background, then poll or block on it. This is explicit
async: there are no promises, no event loop, and no callbacks -- you `spawn` a
command, get an opaque `Process` handle back, and later `wait` for it (or poll
with `is_done` / `wait_any`). The model is the same boundary as `run`: wisp
manages the command it launched, not any grandchildren that command forks.

`Process` is an opaque handle. The only readable field is `.pid: int` (the
spawned command's process id). A `Process` cannot be compared, interpolated into
a string, or used in arithmetic -- those are compile errors. It is constructed
only by `spawn` and `wait_any`.

**The capture model.** `spawn` runs the command in a wrapper subshell that
backgrounds it, redirecting its stdout and stderr to private temp files and its
stdin to `/dev/null`. The wrapper publishes the command's pid and, when the
command finishes, records its exit (or death) code to a done-marker. `wait`
reads the marker and the two temp files into a `RunResult` with `.stdout`,
`.stderr`, and `.code`, exactly like `run_full`. Output completeness has the
same caveat as `run_full`: if the command forks a descendant that inherits the
captured file descriptors and outlives the command, late writes from that
descendant may not be captured.

### process.spawn

```
process.spawn(argv: string[]) -> Process
```

Launches `argv` in the background and returns a `Process` immediately, without
waiting. `argv` is rebuilt and executed exactly as `run` does -- each element is
a separate, quoted word, so shell metacharacters (`$(...)`, backticks, `;`, `*`,
spaces) in any element are inert data, never interpreted. An empty `argv` aborts
with a located error.

### process.wait

```
process.wait(p: Process) -> RunResult
```

Blocks until `p` finishes, then returns its `RunResult`. `wait` is
**idempotent**: the first call blocks, builds the `RunResult`, caches it on the
handle, and reclaims the command's temp files; every later `process.wait(p)` returns the
same cached `RunResult` without a second shell `wait` and without touching the
already-deleted temps.

**Reaping contract.** You must `wait` every process you `spawn` -- that is what
reaps the finished child and reclaims its temp files. A `Process` you never
`wait` leaks its temp files and a zombie until the shell exits. `wait_any`
returns only one process; `wait` the rest too.

### process.is_done

```
process.is_done(p: Process) -> bool
```

A non-blocking, non-reaping liveness check. Returns `true` once the command has
finished (whether it exited normally or was killed by a signal), `false` while it
is still running. It never blocks and never reaps, so a later `process.wait(p)` still
returns the real result.

### process.pid_alive

```
process.pid_alive(pid: int) -> bool
```

A total liveness probe for an arbitrary PID. Returns `true` if a process with
`pid` exists and is signalable by the caller (POSIX `kill -0 $pid`). Returns
`false` in two cases: no process with that PID exists, and a process exists but
the caller lacks permission to signal it (EPERM). It never aborts.

**Use case: lock-file-owner pattern.** Read a PID from a lock file, then call
`pid_alive` to check whether the holder is still running before deciding to
steal the lock.

**PID-reuse caveat.** `true` means "a process with this PID exists NOW", not
necessarily your process -- the OS may have recycled the PID. For a process
wisp itself `spawn`ed, use `is_done` (PID-reuse-resistant).

```wisp
import "process"

fn main() -> int {
  print(to_string(process.pid_alive(2147483647)))
  return 0
}
// stdout: false
```

### process.signal

```
process.signal(p: Process, sig: string) -> void
```

Sends `sig` to the spawned command, best-effort. `sig` must be a **string
literal** from the SEND set:
`"INT"`, `"TERM"`, `"HUP"`, `"QUIT"`, `"USR1"`, `"USR2"`, `"KILL"`, `"STOP"`,
`"CONT"`. Unlike `on_signal`'s trap set, this set includes `KILL`/`STOP`/`CONT`:
they cannot be trapped, but they can be sent. A non-literal or unsupported `sig`
is a compile error.

`signal` is **total** -- it never aborts. It is a no-op once the process has
finished, which makes it **PID-reuse-resistant**: it will not signal an unrelated
process that happens to reuse the pid after the command finished. A documented
microsecond residual race remains (the gap between the kernel reaping the pid and
the finished-check), unclosable without process groups, which are deliberately
out of scope. Note also that `signal` observes the command's **current** signal
disposition: if you `spawn` then immediately `signal` before the command installs
its own handler, the default action applies, so an exact trapped exit code is not
guaranteed from a spawn-then-signal sequence.

### process.wait_any

```
process.wait_any(ps: Process[], poll_secs: int) -> Process
```

Polls the `ps` array in **list order** and returns the first process observed
done. Each round scans the whole list before sleeping, so an already-finished
process is returned on the first scan even when `poll_secs > 0`. `poll_secs` is
in **whole seconds** (sub-second sleep is not portable) and must be `>= 0`; `0`
is a tight poll. An empty list or a negative `poll_secs` aborts with a located
error attributed to `wait_any`. `wait_any` reads markers only -- it does not
reap -- so it composes with a later `wait` on the returned process and on the
others.

### Running a command under a timeout (composition)

There is no `run_timeout` builtin. A wall-clock timeout composes directly from the
background-process builtins: `spawn` the command, poll `is_done` while sleeping in
whole-second steps up to the budget, and if the deadline is exceeded `signal` it and
`wait` to reap. (`timeout(1)` is deliberately not used -- it is absent on macOS and
not POSIX.) Whole-second granularity is a consequence of `sleep` being whole-second
only (sub-second sleep is not portable), so the effective deadline is within one
second of `secs`.

```wisp
// Run argv, terminating it if it runs longer than `secs` whole seconds.
// Returns the RunResult; on timeout, .code reflects the signal-kill.
fn run_with_timeout(argv: string[], secs: int) -> RunResult {
  let p: Process = process.spawn(argv)
  let elapsed: int = 0
  while (elapsed < secs) {
    if (process.is_done(p)) {
      return process.wait(p)
    }
    sleep(1)
    elapsed = elapsed + 1
  }
  // Deadline exceeded: ask it to terminate, then reap.
  if (!process.is_done(p)) {
    process.signal(p, "TERM")
  }
  return process.wait(p)
}
```

A command that ignores `SIGTERM` would keep `wait` blocked; for such commands add an
escalation -- after a short grace poll, `process.signal(p, "KILL")` (which cannot be ignored)
before the final `wait`. Distinguish "timed out" from "exited on its own" by tracking
whether the poll loop fell through to the `signal` path (e.g. set a `bool` flag), since
`wait`'s `RunResult.code` after a `TERM` is the signal-kill code, not a dedicated
timeout sentinel.

### process.make_fifo

```
process.make_fifo(path: string) -> void
```

Creates a named pipe (FIFO) at `path` via `mkfifo`, for inter-process
communication between spawned commands. `path` flows as inert data (a leading
dash is guarded with `--`), so metacharacters in the path are never interpreted.
It aborts with a located error if the path already exists or cannot be created.

### Example

```wisp
import "process"

fn main() -> int {
  let p: Process = process.spawn(["echo", "hello from a child"])
  let r: RunResult = process.wait(p)
  print(r.stdout)
  print("child pid was ${p.pid}, exit ${r.code}")
  return 0
}
```

## Pipelines

Run a sequence of commands connected by shell pipes, capturing the last stage's
stdout, stderr, and exit code. wisp compiles to POSIX shell; `pipe` compiles to a
POSIX shell pipeline (`stage1 | stage2 | ... | stageN`) via a safe helper that
builds the pipeline string from compiler-controlled tokens only -- no argv data
ever enters the evaluated string.

### process.pipe

```
process.pipe(stages: string[][]) -> RunResult
```

Runs the stages as a pipeline and returns a `RunResult` with the last stage's
`.stdout`, `.stderr`, and `.code`. Each inner array is one stage's argv: `stages[0]`
is the first command, `stages[N-1]` is the last.

**Capture model.** Only the last stage's stdout and stderr are captured; earlier
stages' stderr inherits to the script's own stderr. The first stage's stdin is
`/dev/null` (matching `run_full`). Trailing bytes are preserved exactly (no
stripping), like `run_full`.

**Status semantics.** The result `.code` is the last stage's exit code only --
there is no `PIPESTATUS` and no `set -o pipefail`. An earlier stage's failure is
invisible: if stage 0 exits 3 and stage 1 exits 0, `.code` is 0.

**Empty-stage handling.** An empty outer array (`[]`) aborts located. An empty
inner argv (`[]` as one stage) causes that stage to exit 127 (command-not-found
semantics); because it is not the last stage, the exit code is invisible, but
stdout will be empty. If the empty-argv stage is the last, `.code` is 127.

**Injection safety.** argv elements flow through the run-family mechanism: each
element is a separate quoted word and is never shell-interpolated. Shell
metacharacters in any element (`$(...)`, backticks, `;`, `*`, spaces) are inert
data.

```wisp
let r: RunResult = process.pipe([["echo", "hello"], ["tr", "a-z", "A-Z"]])
print(r.stdout)
print("exit ${r.code}")
```

Runnable example -- sort three numbers through a two-stage pipeline:

```wisp
import "process"

fn main() -> int {
  let r: RunResult = process.pipe([["printf", "3\n1\n2\n"], ["sort"]])
  print(r.stdout)
  print("exit ${r.code}")
  return 0
}
```

## Time and randomness

`now` and `random` are nondeterministic: their output varies across runs and
cannot be pinned to an exact value. Examples and tests assert properties
computed inside the program, not exact values (e.g. `t > 1000000000` rather
than `t == <literal>`).

### now

```
now() -> int
```

Returns the current Unix epoch in whole seconds, via `date +%s`. `date +%s`
is technically a GNU/BSD/busybox extension to POSIX, but it is present on
every dash/busybox ash/bash/zsh target wisp supports. No arguments; no
injection surface.

**Portability:** the only external dependency is `date`, a base utility
present on every target.

**Not monotonic:** `now()` reads the wall clock. It can move backward under
NTP adjustments or manual time changes; it does not guarantee
non-decreasing values across calls.

```wisp
let t: int = now()
print(to_string(t > 1000000000))   // true (past 2001-09-09)
```

### sleep

```
sleep(secs: int) -> void
```

Pauses the program for `secs` whole seconds, via the external `sleep`
command. `secs` must be `>= 0`; a negative argument is a located abort
(`sleep: negative duration`), not a silent malformed call to the shell.
`secs` flows as inert data and is never shell-interpolated.

`sleep` is a statement-only builtin: it has no usable return value (`void`)
and cannot appear in an expression or a `let` binding.

**Portability:** fractional/sub-second sleep is not supported. POSIX `sleep`
accepts only non-negative integers; fractional `sleep 0.5` is a GNU/busybox
extension that fails on some targets. The portable, always-correct form is
whole seconds only.

```wisp
sleep(0)
sleep(2)
```

### math.random

```
math.random(max: int) -> int
```

Returns a non-cryptographic random integer in `[0, max)`. `max` must be
`> 0`; `max <= 0` is a located abort (`random: max must be positive`). The
result is in the range `[0, max)`: `math.random(1)` always returns `0`; smaller
values of `max` make collisions between successive calls common by design.

**Implementation:** each call runs a fresh `awk` process and seeds `srand`
from a combination of the wall-clock time (`srand()`-twice idiom), the
process id (`$$`), and a monotonic per-process counter. This varies the
seed per call so the same-second-same-seed forced-collision cannot occur.
`max` flows via awk `-v` and is never interpolated into program text.
`to_int(rand() * max)` is used rather than `%`-based modulo to avoid bias.
awk `srand`/`rand` works on all four supported shells (including dash, which
has no `$RANDOM`).

**Non-cryptographic:** `random` is suitable for jitter, shuffle, and
pick-a-value uses. Do not use it for security tokens, cryptographic keys,
or sampling that requires uniformity guarantees.

**Reproducibility note:** two runs that start in the same second with the
same process id will reproduce the same sequence. This is a documented
non-crypto limitation, not a bug.

```wisp
let r: int = math.random(100)
print(to_string(r >= 0 && r < 100))   // true
```

Runnable example using all three builtins:

```wisp
import "math"

fn main() -> int {
    let t: int = now()
    print(to_string(t > 1000000000))
    let r: int = math.random(100)
    print(to_string(r >= 0 && r < 100))
    sleep(0)
    print("done")
    return 0
}
```

## Argument parsing

### parse_args

```
parse_args(args: string[], value_flags: string[]) -> ({string: string}, string[], string[])
```

Parses a raw argument list into three parts: valued flags, boolean switches, and
positional arguments. The return is a 3-tuple and must be consumed with annotated
tuple destructuring:

```wisp
let (vals: {string: string}, sw: string[], files: string[]) = parse_args(args, ["--name"])
```

`value_flags` is the complete list of flag names that consume the next token as a
value (for example `["--name", "-o"]`). This is the only source of truth for which
flags take a value; every other flag-shaped token is treated as a boolean switch.

**Return slots:**
- Slot 0 `{string: string}` - valued flags that appeared, keyed by flag name. A
  flag in `value_flags` whose value is not present (end of args) is omitted from
  the dict; use `dict.get(vals, "--name")` which returns `Optional[string]`.
- Slot 1 `string[]` - flag tokens not in `value_flags`, in first-seen order,
  deduped by exact string (boolean switches).
- Slot 2 `string[]` - non-flag tokens, in order (positionals).

**Value forms.** A flag `F` in `value_flags` accepts two forms:
- `F VALUE` (space form): the next token is the value, consumed unconditionally
  even if it looks like a flag. At end of args, `F` is omitted from the dict.
- `F=VALUE` (equals form): the value is the text after the first `=`. An empty
  value (`F=`) yields a present key with the empty string, so
  `dict.get(vals, "F")` returns `Some("")`.

Last occurrence wins: `["--n", "a", "--n", "b"]` -> `{"--n": "b"}`.

**`--` terminator.** A bare `--` token ends option parsing; it is dropped and
every following token becomes a positional. Exception: if a value-flag in space
form is immediately followed by `--`, that `--` is consumed as the flag's literal
value and does not act as a terminator (`{"-o": "--"}` with the rest parsed
normally).

**Boolean switches.** A flag token not in `value_flags` goes into the switches
array. The bare and `=` forms are distinct strings and are not deduped against
each other (`["--v", "--v=1"]` -> `["--v", "--v=1"]`); only exact-string
repeats are deduped (`["--v", "--v"]` -> `["--v"]`).

**Positionals.** Any token not starting with `-`, or the lone `-` (the
conventional stdin indicator), is a positional.

**Usage example:**

```wisp
import "dict"
import "string"

fn main(args: string[]) -> int {
    let (vals: {string: string}, sw: string[], files: string[]) = parse_args(args, ["--name"])
    let name: string = unwrap_or(dict.get(vals, "--name"), "world")
    let verbose: bool = string.contains(sw, "--verbose")
    print("hello, ${name}")
    print("verbose: ${to_string(verbose)}")
    print("files: ${string.join(files, ", ")}")
    return 0
}
```

**Non-goals.** No short-flag bundling (`-abc`), no attached short value
(`-ovalue`); use `-o value` or `-o=value`. No alias resolution (`-v` vs
`--verbose`); OR the alternatives yourself. No type coercion (values are
strings; convert with `to_int(...)` etc.). No required-flag checks or
help generation.

`parse_args` is pure: it performs no I/O and never aborts. A value containing
shell metacharacters (`$(...)`, backticks, `;`, `*`, spaces) is preserved
literally and never executed.

## Regular expressions

wisp regular expressions are **POSIX ERE** (Extended Regular Expressions) -- the
flavor `awk`, `grep -E`, and `egrep` use. They are **not** PCRE and **not** BRE.
Matching is **byte-based** under `LC_ALL=C`, consistent with the rest of the
string standard library, so `.` and the character classes count bytes, not
codepoints.

What ERE gives you: anchors `^` `$`, character classes `[...]` and POSIX classes
`[[:digit:]]` / `[[:alnum:]]` / `[[:space:]]`, alternation `|`, grouping `(...)`
(for precedence and alternation only -- see whole-match-only below), the
quantifiers `*` `+` `?` `{n,m}`, and `.`. Write the POSIX classes, **not** the
PCRE shorthands: there is no `\d`, `\w`, `\b`, or lookaround. For
case-insensitivity, write the class yourself (`[Hh]ello`).

**Whole-match only.** These builtins report the WHOLE match; there is no
capture-group API (no `regex_groups`, no named groups). Group parentheses still
work for precedence and alternation, but you cannot extract a submatch. This is a
hard limit of the portable `awk` engine (busybox `awk` and `mawk` lack the
group-capturing `match(s, re, arr)`), not a temporary gap.

**Malformed patterns.** A syntactically invalid pattern (an unbalanced `[`, a bad
class) is a catchable located abort: it is fatal at the top level and catchable in
`try`/`catch`, exactly like `unwrap` of `None`. The abort is guaranteed for
patterns the underlying `awk` rejects; busybox `awk` is lenient and accepts some
malformed patterns (notably an unbalanced `(`), returning an unspecified result
instead of aborting. A valid pattern never aborts, whether or not it matches.

**Bytes and limits.** A NUL byte in the subject, pattern, or replacement is
unsupported (the values reach `awk` as C strings and truncate at NUL), matching
`read_file` and the byte-string model. Because the subject, pattern, and
replacement are transported to `awk` through the environment, a very large
subject, pattern, or replacement may hit the operating system's
environment-size limit before `awk` runs; this is an inherent bound of the
transport, acceptable for typical sizes.

### regex.matches

```
regex.matches(s: string, pattern: string) -> bool
```

True when `pattern` matches anywhere in `s`, false otherwise. A malformed
pattern aborts (it never silently reads as `false`).

```wisp
regex.matches("hello42", "[[:digit:]]+")   // true
regex.matches("hello", "[0-9]+")           // false
regex.matches("hat", "^h")                 // true
regex.matches("cat", "c|d")                // true
```

### regex.find

```
regex.find(s: string, pattern: string) -> Optional[string]
```

The first whole match in `s` as `Some(match)`, or `None` when there is no match.
A match that ends in a newline is preserved exactly.

```wisp
regex.find("a1b22c", "[0-9]+")              // Some("1")  -- the first match
regex.find("abc", "[0-9]+")                 // None
unwrap_or(regex.find("x", "[0-9]+"), "-")   // "-"
```

### regex.find_all

```
regex.find_all(s: string, pattern: string) -> string[]
```

Every **non-empty** whole match, left to right, as a `string[]`. An empty input
or no match yields `[]`. A zero-width-capable pattern (such as `x*`) terminates
and contributes only its non-empty matches. Anchors apply **per remaining
window** as the scan advances, not only to the original string ends: scanning
`"aaa"` with `"^a"` yields `["a", "a", "a"]`.

```wisp
regex.find_all("a1b22c333", "[0-9]+")   // ["1", "22", "333"]
regex.find_all("abc", "[0-9]+")         // []
regex.find_all("aaa", "^a")             // ["a", "a", "a"]  -- anchor per window
```

### regex.replace

```
regex.replace(s: string, pattern: string, replacement: string) -> string
```

Replaces **all** matches of `pattern` in `s` with `replacement`. In the
replacement, `&` is the whole match and `\&` is a literal `&`. Group
backreferences (`\1`, `\2`) are **not supported** (the portable `awk` engine has
none); a `\1` is left as `awk` renders it rather than expanding a group. A bare
`\\` (literal backslash) in the replacement is engine-dependent across `awk`
implementations, so avoid relying on it. A trailing newline in the subject, and
a match that ends in a newline, are preserved exactly.

```wisp
regex.replace("a1b2", "[0-9]+", "#")      // "a#b#"   -- all matches
regex.replace("ab", "(a)(b)", "<&>")      // "<ab>"   -- & is the whole match
regex.replace("a&b", "&", "\\&")          // "a&b"    -- \& is a literal &
```

### The `regex` module

The four regex operations are a **core module**: a reserved namespace you opt
into with `import "regex"`. There is no flat/bare spelling; each operation is
reachable only as `regex.<member>`. You may alias it (`import "regex" as re`)
and then write `re.find(...)`. No filesystem module is involved -- `regex` is
resolved internally.

```wisp
import "regex"

fn main() -> int {
    if (regex.matches("a1b2", "[0-9]+")) {
        print(regex.replace("a1b2", "[0-9]+", "#"))   // a#b#
    }
    return 0
}
```

| member            | signature                              |
| ----------------- | --------------------------------------- |
| `regex.matches`   | `(string, string) -> bool`             |
| `regex.find`      | `(string, string) -> Optional[string]` |
| `regex.find_all`  | `(string, string) -> string[]`         |
| `regex.replace`   | `(string, string, string) -> string`   |

Semantics, POSIX ERE dialect, and byte-based `LC_ALL=C` matching are
documented in full above.

## JSON (`import "json"`)

JSON support is the first **core module**: a reserved, always-available namespace
you opt into with an import, not a bare builtin.

```wisp
import "json"

fn main() -> int {
    let doc: json.Value = json.decode("{ \"name\": \"wisp\", \"tags\": [1, 2] }")
    print(json.encode(doc))                            // {"name":"wisp","tags":[1,2]}
    print(json.as_string(unwrap(json.get(doc, "name")))) // wisp
    return 0
}
```

The import binds the `json` namespace; every member is spelled `json.<member>`.
You may alias it (`import "json" as j`) and then write `j.decode(...)`. No
filesystem module is involved -- `json` is resolved internally.

### The `json.Value` type

`json.Value` is an **opaque** handle to a parsed JSON value (any of null, bool,
number, string, array, object). Opaque means the same rules as arrays and structs:
no arithmetic, no comparison with `==`, no `to_string(v)` conversion, no
interpolation into a `"${...}"` string. To get a native wisp value out, use an
accessor (`as_string`, `as_int`, ...) or `decode[T]`; to get its text, use
`encode`. `debug(v)` renders its canonical JSON text.

Internally a `json.Value` stores **canonical** JSON text: whitespace is removed
(minified), but string and number tokens are preserved **byte for byte**. So
`json.encode(json.decode(s))` is `s` re-minified, and numbers never lose
precision -- `9007199254740993` and `0.1` and `1e400` round-trip exactly.
Duplicate object keys are preserved; `json.get` returns the first.

### Building values

| member | signature | result |
| --- | --- | --- |
| `json.from_int(i)` | `(int) -> json.Value` | a JSON number |
| `json.from_float(f)` | `(float) -> json.Value` | a JSON number |
| `json.from_bool(b)` | `(bool) -> json.Value` | `true` / `false` |
| `json.from_string(s)` | `(string) -> json.Value` | a JSON string (escaping applied) |
| `json.null()` | `() -> json.Value` | `null` |
| `json.array(elems)` | `(json.Value[]) -> json.Value` | a JSON array |
| `json.object(entries)` | `(dict[string, json.Value]) -> json.Value` | a JSON object, in insertion order |

The scalar constructors are named `from_int` / `from_string` / `from_float` /
`from_bool` because the bare type names (`int`, `string`, ...) are keywords the
parser will not accept after a `.`.

```wisp
let arr: json.Value = json.array([json.from_int(1), json.from_string("x")])
let obj: json.Value = json.object({"a": arr, "n": json.null()})
print(json.encode(obj))   // {"a":[1,"x"],"n":null}
```

### Inspecting values

| member | signature | result |
| --- | --- | --- |
| `json.type_of(v)` | `(json.Value) -> string` | one of `null bool number string array object` |
| `json.get(v, key)` | `(json.Value, string) -> Optional[json.Value]` | object member, or `None` (not an object / absent) |
| `json.at(v, i)` | `(json.Value, int) -> Optional[json.Value]` | array element, or `None` (not an array / out of range) |

`get` and `at` never abort: a missing key, a wrong container type, or an
out-of-range index is `None`.

### Extracting native values

| member | signature | on type mismatch |
| --- | --- | --- |
| `json.as_string(v)` | `(json.Value) -> string` | located abort |
| `json.as_int(v)` | `(json.Value) -> int` | located abort (also on out-of-range) |
| `json.as_float(v)` | `(json.Value) -> float` | located abort (also on overflow) |
| `json.as_bool(v)` | `(json.Value) -> bool` | located abort |

An accessor called on the wrong JSON type is a **located abort** (catchable in
`try`/`catch`, exactly like `unwrap` of `None`). `as_string` fully unescapes the
JSON string, decoding `\uXXXX` (including surrogate pairs) to UTF-8.

### encode and decode

```
json.encode(v: json.Value) -> string
json.decode[T](s: string) -> T          // T defaults to json.Value
```

`json.encode` returns the canonical text of a value. `json.decode` parses and
validates untrusted text; **malformed JSON is a located abort**. The optional
type argument projects the result:

- `json.decode(s)` or `json.decode[json.Value](s)` -- validate and box.
- `json.decode[string](s)` / `[int]` / `[float]` / `[bool]` -- require the
  top-level value to be that scalar and return it natively (a wrong type aborts).

```wisp
let n: int = json.decode[int]("42")
let s: string = json.decode[string]("\"hi\"")
let v: json.Value = json.decode("[1, 2, 3]")
```

Recursive projection into structs/arrays is out of scope; decode a `json.Value`
and walk it with `get` / `at` / the accessors.

### Notes and limits

- **Numbers** are stored verbatim, so a `json.Value` never loses precision.
  Precision loss happens only when you convert to a native `float`
  (`as_float` / `decode[float]`), which is inherent to float parsing, and an
  overflow to a non-finite value aborts.
- **Safety.** JSON data is never re-parsed by the shell: values flow through
  quoted expansions and the environment, so `$(...)`, backticks, and `${...}` in
  JSON data are inert.
- **NUL bytes** are unsupported (the wisp byte-string model is NUL-free).

## Testing

These builtins are available inside `test` blocks in `*_test.wisp` files. On failure
they immediately fail the current test with a located message (file, line, column) and
render values through `debug()` where applicable. See the [testing guide](/guide/testing/)
for full usage.

### assert

```
assert(cond: bool, msg: string = "") -> void
```

Fails if `cond` is false. The optional `msg` is appended to the failure message.

```wisp
test ("assert examples") {
    assert(true)
    assert(1 < 2, "ordering")
}
```

### assert_eq

```
assert_eq[T: comparable](got: T, want: T) -> void
```

Fails if `got != want`. Renders both values via `debug()` in the failure message.
`T` admits int, bool, string, float, an enum type, or a nested comparable `Optional`.

```wisp
enum Color: int { Red, Green, Blue }

test ("assert_eq example") {
    assert_eq(1 + 1, 2)
    assert_eq("hello", "hello")
    assert_eq(Color.Red, Color.Red)
}
```

### assert_ne

```
assert_ne[T: comparable](got: T, want: T) -> void
```

Fails if `got == want`. Renders both values via `debug()` in the failure message.
`T` admits int, bool, string, float, an enum type, or a nested comparable `Optional`.

```wisp
test ("assert_ne example") {
    assert_ne(1, 2)
    assert_ne(Color.Red, Color.Blue)
}
```

### assert_some

```
assert_some(o: Optional[T]) -> void
```

Fails if `o` is `None`.

```wisp
test ("assert_some example") {
    assert_some(Some(42))
}
```

### assert_none

```
assert_none(o: Optional[T]) -> void
```

Fails if `o` is `Some(x)`.

```wisp
test ("assert_none example") {
    let missing: Optional[int] = None
    assert_none(missing)
}
```

### assert_ok

```
assert_ok(r: Result[T]) -> void
```

Fails if `r` is `Err`.

```wisp
test ("assert_ok example") {
    let r: Result[int] = Ok(1)
    assert_ok(r)
}
```

### assert_err

```
assert_err(r: Result[T]) -> void
```

Fails if `r` is `Ok`.

```wisp
test ("assert_err example") {
    let r: Result[int] = Err(error("oops"))
    assert_err(r)
}
```

### assert_contains

```
assert_contains(s: string, sub: string) -> void
assert_contains(xs: T[], x: T) -> void   // T: comparable
```

Overloaded on the first argument type. When the first argument is a string, asserts that
`sub` is a substring. When the first argument is an array, asserts that `x` is a member.
The array form's `T: comparable` admits int, bool, string, float, or an enum type.

```wisp
test ("assert_contains examples") {
    assert_contains("hello, world", "world")
    let nums: int[] = [1, 2, 3]
    assert_contains(nums, 2)
}
```

### skip

```
skip(reason: string) -> void
```

Marks the current test skipped and stops executing its body immediately. The reason is
reported in the test output. Teardown still runs after a skip.

```wisp
test ("skipped test") {
    skip("not implemented yet")
}
```

### test_tmpdir

```
test_tmpdir() -> string
```

Returns the path to a fresh temporary directory created before the current test and
removed after it (teardown still sees it). The directory is unique per test, so tests
cannot interfere through it. Only valid inside a test body or lifecycle functions.

```wisp
test ("temp directory example") {
    let dir: string = test_tmpdir()
    fs.write_file("${dir}/out.txt", "hello")
    assert_eq(fs.read_file("${dir}/out.txt"), "hello")
}
```
