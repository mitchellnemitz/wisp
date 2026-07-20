package runtime

// JSON core-module runtime (Unit 5). The single primitive is __wisp_json_awk, a
// NON-recursive pushdown scanner over ENVIRON["__wisp_json_in"], selected by
// ENVIRON["__wisp_json_op"]. It never recurses (busybox stack safety); nesting is
// tracked by an explicit awk-array state stack. Values flow in via ENVIRON under
// LC_ALL=C (byte-accurate, the established float/scmp/regex pattern); every op
// prints its payload followed by a one-byte sentinel \001 so the shell wrapper
// can tell success from truncation, and exits nonzero on any parse/type error.
//
// Canonical form (the stored json.Value text): whitespace minified; string and
// number token bodies emitted VERBATIM (validated but never re-escaped or
// %.17g-normalized) so canonicalization is idempotent and there is no byte
// emission on the encode/get/at path; duplicate object keys preserved.

// JSON runtime helper ids. The engine is the shared awk primitive; the thin sh
// wrappers set ENVIRON inputs, snapshot rc, strip the \001 sentinel, and route
// faults through a uniform located abort (spec 6.3).
const (
	JSONEngine       = "__wisp_json_awk"           // <op> <in> <arg>: iterative scanner; payload+\001, nonzero on error
	JSONFail         = "__wisp_json_fail"          // <pos> <rc> <reason>: rc>128 -> internal-awk abort, else reason
	JSONValidate     = "__wisp_json_validate"      // <pos> <s>: canonical text in __ret; malformed aborts located
	JSONEscape       = "__wisp_json_escape"        // <s>: JSON string literal (quoted) in __ret; total
	JSONTypeOf       = "__wisp_json_type_of"       // <canonical>: type keyword in __ret; total
	JSONGet          = "__wisp_json_get"           // <pos> <canonical> <key>: 0 | 1<slice> in __ret
	JSONAt           = "__wisp_json_at"            // <pos> <canonical> <idx>: 0 | 1<slice> in __ret
	JSONAsString     = "__wisp_json_as_string"     // <pos> <canonical>: native string in __ret; non-string aborts
	JSONAsInt        = "__wisp_json_as_int"        // <pos> <canonical>: range-checked int in __ret; non-int aborts
	JSONAsFloat      = "__wisp_json_as_float"      // <pos> <canonical>: %.17g float in __ret; non-number/overflow aborts
	JSONAsBool       = "__wisp_json_as_bool"       // <pos> <canonical>: true/false in __ret; non-bool aborts
	JSONDecodeString = "__wisp_json_decode_string" // <pos> <s>: validate then as_string
	JSONDecodeInt    = "__wisp_json_decode_int"    // <pos> <s>: validate then as_int
	JSONDecodeFloat  = "__wisp_json_decode_float"  // <pos> <s>: validate then as_float
	JSONDecodeBool   = "__wisp_json_decode_bool"   // <pos> <s>: validate then as_bool
)

// jsonEngineSrc is the shared awk engine. Operands arrive via ENVIRON:
// __wisp_json_op (op), __wisp_json_in (JSON text), __wisp_json_arg (key/index).
// The awk program is a compiler constant (no data interpolated into the program
// text); it uses no gensub, no regex intervals, no bashisms, and is byte-level
// via substr under LC_ALL=C. Functions:
//
//	isdig/hexval  -- byte-class + \uXXXX helpers built on the ord/hx maps
//	read_string   -- validate a "-string (escapes, \uXXXX, surrogate pairing,
//	                 reject raw <0x20); append VERBATIM to out
//	read_number   -- validate an RFC-8259 number; append VERBATIM to out
//	scan          -- iterative validate+canonicalize of one document
//	after         -- post-value state from the container stack
const jsonEngineSrc = `__wisp_json_awk() {
	__wisp_json_op="$1" __wisp_json_in="$2" __wisp_json_arg="$3" LC_ALL=C awk '
	function isdig(c){ return (c != "" && ord[c] >= 48 && ord[c] <= 57) }
	function hexval(h,   v, i, c){
		v = 0
		for (i = 1; i <= 4; i++) { c = substr(h, i, 1); if (!(c in hx)) return -1; v = v * 16 + hx[c] }
		return v
	}
	function read_string(   c, c2, h, hi, lo, lh){
		out = out "\""; p++
		while (1) {
			if (p > n) return 0
			c = substr(s, p, 1)
			if (c == "\"") { out = out "\""; p++; return 1 }
			if (c == "\\") {
				c2 = substr(s, p+1, 1)
				if (c2 == "\"" || c2 == "\\" || c2 == "/" || c2 == "b" || c2 == "f" || c2 == "n" || c2 == "r" || c2 == "t") { out = out "\\" c2; p = p+2; continue }
				if (c2 == "u") {
					h = substr(s, p+2, 4); hi = hexval(h); if (hi < 0) return 0
					if (hi >= 55296 && hi <= 56319) {
						if (substr(s, p+6, 2) != "\\u") return 0
						lh = substr(s, p+8, 4); lo = hexval(lh); if (lo < 0) return 0
						if (lo < 56320 || lo > 57343) return 0
						out = out "\\u" h "\\u" lh; p = p+12; continue
					}
					if (hi >= 56320 && hi <= 57343) return 0
					out = out "\\u" h; p = p+6; continue
				}
				return 0
			}
			if (ord[c] < 32) return 0
			out = out c; p++
		}
	}
	function read_number(   start, c){
		start = p
		if (substr(s, p, 1) == "-") p++
		c = substr(s, p, 1)
		if (c == "0") { p++ }
		else if (isdig(c)) { while (isdig(substr(s, p, 1))) p++ }
		else return 0
		if (substr(s, p, 1) == ".") { p++; if (!isdig(substr(s, p, 1))) return 0; while (isdig(substr(s, p, 1))) p++ }
		c = substr(s, p, 1)
		if (c == "e" || c == "E") { p++; c = substr(s, p, 1); if (c == "+" || c == "-") p++; if (!isdig(substr(s, p, 1))) return 0; while (isdig(substr(s, p, 1))) p++ }
		out = out substr(s, start, p - start)
		return 1
	}
	function after(sp){ if (sp == 0) return "done"; if (stk[sp] == "A") return "acomma"; return "ocomma" }
	function is_int(x,   i, c, L){
		L = length(x); if (L == 0) return 0
		i = 1; if (substr(x, 1, 1) == "-") i = 2
		if (i > L) return 0
		c = substr(x, i, 1)
		if (c == "0") return (i == L)
		if (!isdig(c)) return 0
		for (; i <= L; i++) if (!isdig(substr(x, i, 1))) return 0
		return 1
	}
	function utf8(cp){
		if (cp < 128) return sprintf("%c", cp)
		if (cp < 2048) return sprintf("%c%c", 192 + int(cp/64), 128 + (cp%64))
		if (cp < 65536) return sprintf("%c%c%c", 224 + int(cp/4096), 128 + int(cp/64)%64, 128 + (cp%64))
		return sprintf("%c%c%c%c", 240 + int(cp/262144), 128 + int(cp/4096)%64, 128 + int(cp/64)%64, 128 + (cp%64))
	}
	function skip_string(   c){
		p++
		while (p <= n) { c = substr(s, p, 1); if (c == "\\") { p = p+2; continue } if (c == "\"") { p++; return } p++ }
	}
	function skip_value(   d, c){
		d = 0
		while (p <= n) {
			c = substr(s, p, 1)
			if (c == "\"") { skip_string(); if (d == 0) return; continue }
			if (c == "{" || c == "[") { d++; p++; continue }
			if (c == "}" || c == "]") { if (d == 0) return; d--; p++; if (d == 0) return; continue }
			if (c == "," && d == 0) return
			p++
		}
	}
	function decode_str(   r, c, c2, h, hi, lh, lo, cp){
		r = ""; p++
		while (p <= n) {
			c = substr(s, p, 1)
			if (c == "\"") { p++; return r }
			if (c == "\\") {
				c2 = substr(s, p+1, 1)
				if (c2 == "\"") { r = r "\""; p = p+2; continue }
				if (c2 == "\\") { r = r "\\"; p = p+2; continue }
				if (c2 == "/") { r = r "/"; p = p+2; continue }
				if (c2 == "b") { r = r sprintf("%c", 8); p = p+2; continue }
				if (c2 == "f") { r = r sprintf("%c", 12); p = p+2; continue }
				if (c2 == "n") { r = r sprintf("%c", 10); p = p+2; continue }
				if (c2 == "r") { r = r sprintf("%c", 13); p = p+2; continue }
				if (c2 == "t") { r = r sprintf("%c", 9); p = p+2; continue }
				if (c2 == "u") {
					h = substr(s, p+2, 4); hi = hexval(h)
					if (hi >= 55296 && hi <= 56319) { lh = substr(s, p+8, 4); lo = hexval(lh); cp = 65536 + (hi-55296)*1024 + (lo-56320); r = r utf8(cp); p = p+12; continue }
					r = r utf8(hi); p = p+6; continue
				}
			}
			r = r c; p++
		}
		return r
	}
	function scan(   c, st, sp){
		st = "val"; sp = 0
		while (1) {
			while (p <= n) { c = substr(s, p, 1); if (c == " " || c == "\t" || c == "\n" || c == "\r") p++; else break }
			if (p > n) break
			c = substr(s, p, 1)
			if (st == "val") {
				if (c == "{") { out = out "{"; p++; sp++; if (sp > 100000) return 0; stk[sp] = "O"; st = "okey"; continue }
				if (c == "[") { out = out "["; p++; sp++; if (sp > 100000) return 0; stk[sp] = "A"; st = "aval"; continue }
				if (c == "\"") { if (!read_string()) return 0; st = after(sp); continue }
				if (c == "-" || isdig(c)) { if (!read_number()) return 0; st = after(sp); continue }
				if (c == "t") { if (substr(s, p, 4) != "true") return 0; out = out "true"; p = p+4; st = after(sp); continue }
				if (c == "f") { if (substr(s, p, 5) != "false") return 0; out = out "false"; p = p+5; st = after(sp); continue }
				if (c == "n") { if (substr(s, p, 4) != "null") return 0; out = out "null"; p = p+4; st = after(sp); continue }
				return 0
			}
			if (st == "aval") { if (c == "]") { out = out "]"; p++; sp--; st = after(sp); continue } st = "val"; continue }
			if (st == "acomma") { if (c == ",") { out = out ","; p++; st = "val"; continue } if (c == "]") { out = out "]"; p++; sp--; st = after(sp); continue } return 0 }
			if (st == "okey") { if (c == "}") { out = out "}"; p++; sp--; st = after(sp); continue } if (c != "\"") return 0; if (!read_string()) return 0; st = "ocolon"; continue }
			if (st == "okeyreq") { if (c != "\"") return 0; if (!read_string()) return 0; st = "ocolon"; continue }
			if (st == "ocolon") { if (c != ":") return 0; out = out ":"; p++; st = "val"; continue }
			if (st == "ocomma") { if (c == ",") { out = out ","; p++; st = "okeyreq"; continue } if (c == "}") { out = out "}"; p++; sp--; st = after(sp); continue } return 0 }
			if (st == "done") return 0
		}
		return (st == "done")
	}
	BEGIN{
		for (i = 1; i <= 255; i++) ord[sprintf("%c", i)] = i
		hexl = "0123456789abcdef"; for (i = 1; i <= 16; i++) hx[substr(hexl, i, 1)] = i-1
		hexu = "ABCDEF"; for (i = 1; i <= 6; i++) hx[substr(hexu, i, 1)] = i+9
		s = ENVIRON["__wisp_json_in"]; n = length(s); p = 1; out = ""
		op = ENVIRON["__wisp_json_op"]
		if (op == "type") {
			while (p <= n) { c = substr(s, p, 1); if (c == " " || c == "\t" || c == "\n" || c == "\r") p++; else break }
			c = substr(s, p, 1)
			if (c == "{") t = "object"; else if (c == "[") t = "array"; else if (c == "\"") t = "string"; else if (c == "t" || c == "f") t = "bool"; else if (c == "n") t = "null"; else t = "number"
			printf "%s", t; printf "\001"; exit 0
		}
		if (op == "escape") {
			r = "\""
			for (i = 1; i <= n; i++) {
				c = substr(s, i, 1); o = ord[c]
				if (c == "\"") r = r "\\\""
				else if (c == "\\") r = r "\\\\"
				else if (o == 8) r = r "\\b"
				else if (o == 9) r = r "\\t"
				else if (o == 10) r = r "\\n"
				else if (o == 12) r = r "\\f"
				else if (o == 13) r = r "\\r"
				else if (o < 32) r = r sprintf("\\u%04x", o)
				else r = r c
			}
			r = r "\""
			printf "%s", r; printf "\001"; exit 0
		}
		if (op == "get") {
			key = ENVIRON["__wisp_json_arg"]
			if (substr(s, p, 1) != "{") { printf "0"; printf "\001"; exit 0 }
			p++
			if (substr(s, p, 1) == "}") { printf "0"; printf "\001"; exit 0 }
			while (1) {
				k = decode_str()
				p++
				vs = p; skip_value(); ve = p
				if (k == key) { printf "1%s", substr(s, vs, ve - vs); printf "\001"; exit 0 }
				c = substr(s, p, 1)
				if (c == ",") { p++; continue }
				printf "0"; printf "\001"; exit 0
			}
		}
		if (op == "at") {
			idx = ENVIRON["__wisp_json_arg"] + 0
			if (substr(s, p, 1) != "[") { printf "0"; printf "\001"; exit 0 }
			p++
			if (substr(s, p, 1) == "]") { printf "0"; printf "\001"; exit 0 }
			if (idx < 0) { printf "0"; printf "\001"; exit 0 }
			i = 0
			while (1) {
				vs = p; skip_value(); ve = p
				if (i == idx) { printf "1%s", substr(s, vs, ve - vs); printf "\001"; exit 0 }
				c = substr(s, p, 1)
				if (c == ",") { p++; i++; continue }
				printf "0"; printf "\001"; exit 0
			}
		}
		if (op == "scalar_string") {
			if (substr(s, 1, 1) != "\"") exit 4
			p = 1; printf "%s", decode_str(); printf "\001"; exit 0
		}
		if (op == "scalar_bool") {
			if (s == "true") { printf "true"; printf "\001"; exit 0 }
			if (s == "false") { printf "false"; printf "\001"; exit 0 }
			exit 4
		}
		if (op == "scalar_int") {
			if (is_int(s)) { printf "%s", s; printf "\001"; exit 0 }
			exit 4
		}
		if (op == "scalar_float") {
			c = substr(s, 1, 1)
			if (c == "-" || isdig(c)) { printf "%s", s; printf "\001"; exit 0 }
			exit 4
		}
		if (!scan()) exit 3
		printf "%s", out; printf "\001"; exit 0
	}
	' 2>/dev/null
}`

// jsonWrappers are the thin sh wrappers over the engine. Each fallible wrapper
// snapshots rc immediately, strips the one trailing \001 sentinel (which also
// preserves a value that legitimately ends in a newline or a \001 byte:
// append-one/strip-one), and on rc != 0 raises a uniform located abort via
// __wisp_json_fail (rc > 128 = an awk crash, surfaced distinctly). The sentinel
// is produced identically to the engine's `printf "\001"`.
var jsonWrappers = []helper{
	{
		id: JSONFail, deps: []string{Fail}, order: 101,
		src: `__wisp_json_fail() {
	if [ "$2" -gt 128 ]; then
		__wisp_fail "$1" "json: internal awk failure (rc=$2)"
	else
		__wisp_fail "$1" "$3"
	fi
}`,
	},
	{
		id: JSONValidate, deps: []string{JSONEngine, JSONFail}, order: 102,
		src: `__wisp_json_validate() {
	__j_out="$(__wisp_json_awk validate "$2" "")"
	__j_rc=$?
	if [ "$__j_rc" -ne 0 ]; then
		__wisp_json_fail "$1" "$__j_rc" "json.decode: invalid JSON"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONEscape, deps: []string{JSONEngine}, order: 103,
		src: `__wisp_json_escape() {
	__j_out="$(__wisp_json_awk escape "$1" "")"
	__ret="${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONTypeOf, deps: []string{JSONEngine}, order: 104,
		src: `__wisp_json_type_of() {
	__j_out="$(__wisp_json_awk type "$1" "")"
	__ret="${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONGet, deps: []string{JSONEngine, JSONFail}, order: 105,
		src: `__wisp_json_get() {
	__j_out="$(__wisp_json_awk get "$2" "$3")"
	__j_rc=$?
	if [ "$__j_rc" -ne 0 ]; then
		__wisp_json_fail "$1" "$__j_rc" "json.get: internal error"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONAt, deps: []string{JSONEngine, JSONFail}, order: 106,
		src: `__wisp_json_at() {
	__j_out="$(__wisp_json_awk at "$2" "$3")"
	__j_rc=$?
	if [ "$__j_rc" -ne 0 ]; then
		__wisp_json_fail "$1" "$__j_rc" "json.at: internal error"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONAsString, deps: []string{JSONEngine, JSONFail}, order: 107,
		src: `__wisp_json_as_string() {
	__j_out="$(__wisp_json_awk scalar_string "$2" "")"
	__j_rc=$?
	if [ "$__j_rc" -ne 0 ]; then
		__wisp_json_fail "$1" "$__j_rc" "json: value is not a string"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONAsInt, deps: []string{JSONEngine, JSONFail, Int}, order: 108,
		src: `__wisp_json_as_int() {
	__j_out="$(__wisp_json_awk scalar_int "$2" "")"
	__j_rc=$?
	if [ "$__j_rc" -ne 0 ]; then
		__wisp_json_fail "$1" "$__j_rc" "json: value is not an integer"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__wisp_int "$1" "${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONAsFloat, deps: []string{JSONEngine, JSONFail, FFinite}, order: 109,
		src: `__wisp_json_as_float() {
	__j_out="$(__wisp_json_awk scalar_float "$2" "")"
	__j_rc=$?
	if [ "$__j_rc" -ne 0 ]; then
		__wisp_json_fail "$1" "$__j_rc" "json: value is not a number"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__j_tok="${__j_out%"$(printf '\001')"}"
	__j_f="$(__wisp_json_x="$__j_tok" LC_ALL=C awk 'BEGIN{ printf "%.17g", (ENVIRON["__wisp_json_x"]+0) }')"
	__wisp_ffinite "$1" "$__j_f"
}`,
	},
	{
		id: JSONAsBool, deps: []string{JSONEngine, JSONFail}, order: 110,
		src: `__wisp_json_as_bool() {
	__j_out="$(__wisp_json_awk scalar_bool "$2" "")"
	__j_rc=$?
	if [ "$__j_rc" -ne 0 ]; then
		__wisp_json_fail "$1" "$__j_rc" "json: value is not a bool"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="${__j_out%"$(printf '\001')"}"
}`,
	},
	{
		id: JSONDecodeString, deps: []string{JSONValidate, JSONAsString}, order: 111,
		src: `__wisp_json_decode_string() {
	__wisp_json_validate "$1" "$2"
	[ -n "$__wisp_err_pending" ] && return
	__wisp_json_as_string "$1" "$__ret"
}`,
	},
	{
		id: JSONDecodeInt, deps: []string{JSONValidate, JSONAsInt}, order: 112,
		src: `__wisp_json_decode_int() {
	__wisp_json_validate "$1" "$2"
	[ -n "$__wisp_err_pending" ] && return
	__wisp_json_as_int "$1" "$__ret"
}`,
	},
	{
		id: JSONDecodeFloat, deps: []string{JSONValidate, JSONAsFloat}, order: 113,
		src: `__wisp_json_decode_float() {
	__wisp_json_validate "$1" "$2"
	[ -n "$__wisp_err_pending" ] && return
	__wisp_json_as_float "$1" "$__ret"
}`,
	},
	{
		id: JSONDecodeBool, deps: []string{JSONValidate, JSONAsBool}, order: 114,
		src: `__wisp_json_decode_bool() {
	__wisp_json_validate "$1" "$2"
	[ -n "$__wisp_err_pending" ] && return
	__wisp_json_as_bool "$1" "$__ret"
}`,
	},
}

func init() {
	registry[JSONEngine] = helper{
		id:    JSONEngine,
		order: 100,
		src:   jsonEngineSrc,
	}
	for _, h := range jsonWrappers {
		registry[h.id] = h
	}
}
