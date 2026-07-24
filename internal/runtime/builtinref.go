package runtime

import "fmt"

// Builtin funcref wrappers. Each __wisp_builtin_<name> is a thin eta-expansion
// that adapts an underlying prelude helper to the funcref calling convention
// (result in $__ret, data via "$@"). This file is the SINGLE synthesis site: the
// spec table below drives generation of every wrapper snippet, and it is pinned
// to the checker's generatable allowlist (types.GeneratableBuiltinFuncrefs) by
// the cross-package consistency test in internal/codegen.
//
// Two shapes:
//   - total (located == false): pass all positionals straight through to the
//     helper, which writes $__ret. The wrapper has the helper's own signature.
//   - located (located == true): the helper's convention is $1=<pos> $2..=data;
//     the wrapper injects the builtin NAME as the synthetic $1 so an abort
//     reports the builtin name rather than a source-file position (position
//     degradation, spec AC4). Void builtins use this shape too (the produced
//     $__ret is simply unread by a void funcref caller).

// builtinWrapperSpec describes one funcref wrapper. Most builtins have a single
// unambiguous funcref type and mint one wrapper, "__wisp_builtin_<builtin>". An
// overloaded builtin (abs/min/max/clamp/sign) instead mints one wrapper PER ARM
// ("__wisp_builtin_<builtin>_<arm>"), set via the arm field; the wrapper id
// suffix and the underlying helper's default name both pick up the arm suffix,
// but the located name injected as $1 stays the bare builtin name (s.builtin),
// matching the existing AC4 degrade-to-builtin-name convention (an abort must
// name "abs", not "abs_int").
type builtinWrapperSpec struct {
	builtin string // wisp builtin name (wrapper id suffix + injected $1 for located)
	arm     string // non-empty for one arm of an overloaded builtin: "int"/"float"
	helper  string // underlying shell helper function name; default "__wisp_"+builtin(+"_"+arm)
	dep     string // prelude snippet id the helper is defined by (tree-shaking edge)
	located bool   // inject a name as $1 (fallible/void helpers)
}

// id returns the wrapper's mangled name / prelude snippet id: plain
// "__wisp_builtin_<builtin>", or "__wisp_builtin_<builtin>_<arm>" for an arm.
func (s builtinWrapperSpec) id() string {
	id := "__wisp_builtin_" + s.builtin
	if s.arm != "" {
		id += "_" + s.arm
	}
	return id
}

// helperName returns the underlying shell helper function name.
func (s builtinWrapperSpec) helperName() string {
	if s.helper != "" {
		return s.helper
	}
	name := "__wisp_" + s.builtin
	if s.arm != "" {
		name += "_" + s.arm
	}
	return name
}

// builtinWrapperSpecs is the authoritative wrapper table, transcribed from the
// genBuiltinCall switch in internal/codegen/expr.go. helper is "__wisp_"+builtin
// for every entry except trunc, whose located helper is the shared __wisp_fint.
var builtinWrapperSpecs = []builtinWrapperSpec{
	// total (pass-through)
	{builtin: "lower", dep: Lower},
	{builtin: "upper", dep: Upper},
	{builtin: "trim", dep: Trim},
	{builtin: "trim_start", dep: TrimStart},
	{builtin: "trim_end", dep: TrimEnd},
	{builtin: "trim_prefix", dep: TrimPrefix},
	{builtin: "trim_suffix", dep: TrimSuffix},
	{builtin: "is_empty", dep: IsEmpty},
	{builtin: "reverse_string", dep: ReverseString},
	{builtin: "starts_with", dep: StartsWith},
	{builtin: "ends_with", dep: EndsWith},
	{builtin: "has_env", dep: HasEnv},
	{builtin: "env_or", dep: EnvOr},
	{builtin: "int_or", dep: IntOr},
	{builtin: "float_or", dep: FloatOr},
	{builtin: "pid_alive", dep: PidAlive},
	{builtin: "file_exists", dep: FileExists},
	{builtin: "is_dir", dep: IsDir},
	{builtin: "is_file", dep: IsFile},
	{builtin: "is_symlink", dep: IsSymlink},
	{builtin: "dir_name", dep: DirName},
	{builtin: "base_name", dep: BaseName},
	{builtin: "cwd", dep: Cwd},
	{builtin: "read_stdin", dep: ReadStdin},
	{builtin: "now", dep: Now},
	{builtin: "int_max", dep: IntMax},
	{builtin: "int_min", dep: IntMin},
	{builtin: "pi", dep: Pi},
	{builtin: "program_path", dep: ProgramPath},

	// located (fallible; name injected as $1)
	{builtin: "replace", dep: Replace, located: true},
	{builtin: "replace_first", dep: ReplaceFirst, located: true},
	{builtin: "matches", dep: Matches, located: true},
	{builtin: "regex_replace", dep: RegexReplace, located: true},
	{builtin: "repeat", dep: Repeat, located: true},
	{builtin: "count", dep: Count, located: true},
	{builtin: "substring", dep: Substring, located: true},
	{builtin: "char_at", dep: CharAt, located: true},
	{builtin: "pad_start", dep: PadStart, located: true},
	{builtin: "pad_end", dep: PadEnd, located: true},
	{builtin: "ord", dep: Ord, located: true},
	{builtin: "chr", dep: Chr, located: true},
	{builtin: "read_file", dep: ReadFile, located: true},
	{builtin: "file_size", dep: FileSize, located: true},
	{builtin: "sqrt", dep: Sqrt, located: true},
	{builtin: "exp", dep: Exp, located: true},
	{builtin: "ln", dep: Ln, located: true},
	{builtin: "log10", dep: Log10, located: true},
	{builtin: "log2", dep: Log2, located: true},
	{builtin: "pow", dep: Pow, located: true},
	{builtin: "floor", dep: Floor, located: true},
	{builtin: "ceil", dep: Ceil, located: true},
	{builtin: "round", dep: Round, located: true},
	{builtin: "trunc", helper: "__wisp_fint", dep: FIntT, located: true},
	{builtin: "format_float", dep: FormatFloat, located: true},
	{builtin: "gcd", dep: Gcd, located: true},
	{builtin: "lcm", dep: Lcm, located: true},
	{builtin: "random", dep: Random, located: true},
	{builtin: "temp_file", dep: TempFile, located: true},
	{builtin: "temp_dir", dep: TempDir, located: true},

	// void located (side-effecting; name injected as $1, no result read)
	{builtin: "write_file", dep: WriteFile, located: true},
	{builtin: "append_file", dep: AppendFile, located: true},
	{builtin: "set_env", dep: SetEnv, located: true},
	{builtin: "unset_env", dep: UnsetEnv, located: true},
	{builtin: "set_stdin", dep: SetStdin, located: true},
	{builtin: "chmod", dep: Chmod, located: true},
	{builtin: "symlink", dep: Symlink, located: true},
	{builtin: "symlink_force", dep: SymlinkForce, located: true},
	{builtin: "make_fifo", dep: MakeFifo, located: true},
	{builtin: "make_dir", dep: MakeDir, located: true},
	{builtin: "remove_file", dep: RemoveFile, located: true},
	{builtin: "remove_dir", dep: RemoveDir, located: true},
	{builtin: "rename", dep: Rename, located: true},
	{builtin: "sleep", dep: Sleep, located: true},
	{builtin: "change_dir", dep: ChangeDir, located: true},

	// overloaded builtins: one wrapper per int/float arm (see
	// internal/types/funcref_class.go's overloadedFuncrefArms). abs's
	// underlying helpers already exist (AbsInt/FAbs, shared with the direct-call
	// path); min/max/clamp/sign get dedicated standalone total helpers, since
	// their direct-call lowering was inline-only before universal funcrefs.
	{builtin: "abs", arm: "int", helper: "__wisp_abs_int", dep: AbsInt, located: true},
	{builtin: "abs", arm: "float", helper: "__wisp_fabs", dep: FAbs, located: true},
	{builtin: "min", arm: "int", dep: MinInt},
	{builtin: "min", arm: "float", dep: MinFloat},
	{builtin: "max", arm: "int", dep: MaxInt},
	{builtin: "max", arm: "float", dep: MaxFloat},
	{builtin: "clamp", arm: "int", dep: ClampInt},
	{builtin: "clamp", arm: "float", dep: ClampFloat},
	{builtin: "sign", arm: "int", dep: SignInt},
	{builtin: "sign", arm: "float", dep: SignFloat},

	// contains's string arm is a total pass-through over the existing shared
	// __wisp_contains helper (the same one the direct-call path uses); only its
	// array_int arm and index_of's two arms need bespoke bodies (see
	// bespokeFuncrefWrappers below), since a scalar-equality/index-search over a
	// handle-backed array is not one of the two templated shapes.
	{builtin: "contains", arm: "string", helper: Contains, dep: Contains},
}

// builtinWrapperIDSet is the set of all wrapper snippet ids, for O(1) lookup.
var builtinWrapperIDSet = func() map[string]bool {
	m := make(map[string]bool, len(builtinWrapperSpecs)+len(bespokeFuncrefWrappers))
	for _, s := range builtinWrapperSpecs {
		m[s.id()] = true
	}
	for _, h := range bespokeFuncrefWrappers {
		m[h.id] = true
	}
	return m
}()

// IsBuiltinWrapperID reports whether id is a builtin funcref wrapper snippet id
// (equivalently, a builtin funcref mangled name). Codegen calls this at the
// funcref-value lowering site to tree-shake the wrapper in via g.use(id).
func IsBuiltinWrapperID(id string) bool { return builtinWrapperIDSet[id] }

// BuiltinWrapperIDs returns all wrapper snippet ids. Exported for the
// consistency test.
func BuiltinWrapperIDs() []string {
	out := make([]string, 0, len(builtinWrapperSpecs)+len(bespokeFuncrefWrappers))
	for _, s := range builtinWrapperSpecs {
		out = append(out, s.id())
	}
	for _, h := range bespokeFuncrefWrappers {
		out = append(out, h.id)
	}
	return out
}

// builtinWrapperSrc renders the shell source for one wrapper.
func builtinWrapperSrc(s builtinWrapperSpec) string {
	id := s.id()
	if s.located {
		return fmt.Sprintf("%s() {\n\t%s '%s' \"$@\"\n}", id, s.helperName(), s.builtin)
	}
	return fmt.Sprintf("%s() {\n\t%s \"$@\"\n}", id, s.helperName())
}

// init synthesizes every wrapper snippet into the prelude registry. Package-level
// vars (registry) are initialized before init runs, so the registry exists here.
func init() {
	for _, s := range builtinWrapperSpecs {
		id := s.id()
		if _, dup := registry[id]; dup {
			panic("builtinref: duplicate wrapper snippet id " + id)
		}
		registry[id] = helper{
			id:    id,
			deps:  []string{s.dep},
			order: 998,
			src:   builtinWrapperSrc(s),
		}
	}
	for _, h := range bespokeFuncrefWrappers {
		if _, dup := registry[h.id]; dup {
			panic("builtinref: duplicate wrapper snippet id " + h.id)
		}
		registry[h.id] = h
	}
}

// bespokeFuncrefWrappers are funcref wrappers whose body is NOT one of the two
// templated shapes (total pass-through / located name-injected): the array arm
// of contains/index_of (overloadedFuncrefArms) and every axis of the generic
// higher-order builtins (genericFuncrefAxes, internal/types/funcref_class.go).
// Each is a hand-written standalone shell function, transcribed from the
// corresponding codegen lowering (genArrayContains/genIndexOfElem/genMap/
// genFilter/genEach/genReduce/genSortBy/genFind/genAnyAll/genCountWhere/
// gen*Optional/gen*Result in internal/codegen), but written against POSITIONAL
// args ($1, $2, ...) instead of compile-time-known temps, and using `local` for
// every scratch variable so a wrapper that indirectly calls back into itself
// (a recursive callback, or a callback that itself invokes another funcref
// wrapper) cannot clobber an outer invocation's loop/accumulator state --
// unlike codegen-emitted bodies, which get a fresh compiler-chosen temp name per
// call site, ALL invocations of a given wrapper share the same shell function
// text, so its locals must be stack-scoped to be reentrancy-safe.
//
// A generic axis wrapper (map/filter/each/reduce/sort_by/find/any/all/
// count_where/and_then/or_else/map_err) serves ANY element/result type: the
// shell lowering these builtins share with the direct-call path does not vary
// with the scalar element/result type, only with the CONTAINER shape (array/
// Optional/Result), so one wrapper per (builtin, axis) is correct for every
// instantiation -- there is no per-scalar-type monomorphization step here,
// unlike contains/index_of's array arm (a scalar EQUALITY comparison, pinned to
// one concrete element type, int[], per plan.md PR B scope).
//
// Every wrapper whose direct-call counterpart opens its loop via
// beginArrayLoop (map/filter/each/reduce/find/any/all/count_where -- see
// internal/codegen/aggregate.go and aggregate_collections.go) repeats
// loopPendingBreak()'s mid-loop check (loopPendingBreakLine, prelude.go) at the
// top of its own while body: a callback invoked through the wrapper can itself
// raise a fault (set __wisp_err_pending) and return normally rather than
// exiting the process, and without this check the wrapper would keep
// iterating and re-invoking the callback afterward, diverging from the
// direct-call lowering's fail-fast semantics. sort_by_array is deliberately
// exempt: its direct-call counterpart (insertionSort) does not call
// beginArrayLoop either, so this is a pre-existing gap in the direct-call
// path, not a regression introduced by funcref wrapper synthesis, and fixing
// it is out of this PR's scope. contains_array_int/index_of_array_int are
// also exempt: they invoke no callback inside their own loop, so no fault can
// arise mid-loop for them (a fault before the wrapper is called is already
// caught by the caller's statement-level skip-guard).
var bespokeFuncrefWrappers = []helper{
	// --- contains/index_of array-int arm + index_of string arm (the string arm
	// of contains reuses the existing total __wisp_contains helper via
	// builtinWrapperSpecs above; only these three need bespoke bodies because
	// their result/comparison shape does not fit the templated total/located
	// wrapper shapes) ---

	// __wisp_builtin_contains_array_int <id> <target>: mirrors genArrayContains.
	{
		id:    "__wisp_builtin_contains_array_int",
		order: 998,
		src: `__wisp_builtin_contains_array_int() {
	local __wca_id __wca_target __wca_len __wca_i __wca_elem
	__wca_id="$1"
	__wca_target="$2"
	eval "__wca_len=\$__wisp_a_${__wca_id}_len"
	__wca_i=0
	__ret=false
	while [ "$__wca_i" -lt "$__wca_len" ]; do
		eval "__wca_elem=\$__wisp_a_${__wca_id}_${__wca_i}"
		if [ "$__wca_elem" = "$__wca_target" ]; then
			__ret=true
			break
		fi
		__wca_i=$(( __wca_i + 1 ))
	done
}`,
	},
	// __wisp_builtin_index_of_string <s> <sub>: mirrors genIntSentinelToOptional
	// over __wisp_index_of.
	{
		id:    "__wisp_builtin_index_of_string",
		deps:  []string{IndexOf, Alloc},
		order: 998,
		src: `__wisp_builtin_index_of_string() {
	local __wis_s __wis_sub __wis_r __wis_out __wis_tag
	__wis_s="$1"
	__wis_sub="$2"
	__wisp_index_of "$__wis_s" "$__wis_sub"
	__wis_r="$__ret"
	__wisp_alloc
	__wis_out="$__ret"
	if [ "$__wis_r" = -1 ]; then
		__wis_tag=none
		eval "__wisp_s_${__wis_out}_tag=\$__wis_tag"
	else
		__wis_tag=some
		eval "__wisp_s_${__wis_out}_tag=\$__wis_tag"
		eval "__wisp_s_${__wis_out}_value=\$__wis_r"
	fi
	__ret="$__wis_out"
}`,
	},
	// __wisp_builtin_index_of_array_int <id> <target>: mirrors genIndexOfElem.
	{
		id:    "__wisp_builtin_index_of_array_int",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_index_of_array_int() {
	local __wia_id __wia_target __wia_len __wia_i __wia_elem __wia_out __wia_tag
	__wia_id="$1"
	__wia_target="$2"
	__wisp_alloc
	__wia_out="$__ret"
	__wia_tag=none
	eval "__wisp_s_${__wia_out}_tag=\$__wia_tag"
	eval "__wia_len=\$__wisp_a_${__wia_id}_len"
	__wia_i=0
	while [ "$__wia_i" -lt "$__wia_len" ]; do
		eval "__wia_elem=\$__wisp_a_${__wia_id}_${__wia_i}"
		if [ "$__wia_elem" = "$__wia_target" ]; then
			__wia_tag=some
			eval "__wisp_s_${__wia_out}_tag=\$__wia_tag"
			eval "__wisp_s_${__wia_out}_value=\$__wia_i"
			break
		fi
		__wia_i=$(( __wia_i + 1 ))
	done
	__ret="$__wia_out"
}`,
	},

	// --- map (array/optional/result axes); mirrors genMap/genMapTagged ---
	{
		id:    "__wisp_builtin_map_array",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_map_array() {
	local __wma_id __wma_f __wma_out __wma_len __wma_i __wma_outlen __wma_elem
	__wma_id="$1"
	__wma_f="$2"
	__wisp_alloc
	__wma_out="$__ret"
	eval "__wma_len=\$__wisp_a_${__wma_id}_len"
	__wma_i=0
	__wma_outlen=0
	while [ "$__wma_i" -lt "$__wma_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wma_elem=\$__wisp_a_${__wma_id}_${__wma_i}"
		"$__wma_f" "$__wma_elem"
		eval "__wisp_a_${__wma_out}_${__wma_outlen}=\$__ret"
		__wma_outlen=$(( __wma_outlen + 1 ))
		__wma_i=$(( __wma_i + 1 ))
	done
	eval "__wisp_a_${__wma_out}_len=\$__wma_outlen"
	__ret="$__wma_out"
}`,
	},
	{
		id:    "__wisp_builtin_map_optional",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_map_optional() {
	local __wmo_id __wmo_f __wmo_tag __wmo_val __wmo_out __wmo_ret
	__wmo_id="$1"
	__wmo_f="$2"
	eval "__wmo_tag=\$__wisp_s_${__wmo_id}_tag"
	if [ "$__wmo_tag" = some ]; then
		eval "__wmo_val=\$__wisp_s_${__wmo_id}_value"
		"$__wmo_f" "$__wmo_val"
		__wmo_ret="$__ret"
		__wisp_alloc
		__wmo_out="$__ret"
		eval "__wisp_s_${__wmo_out}_tag=\$__wmo_tag"
		eval "__wisp_s_${__wmo_out}_value=\$__wmo_ret"
	else
		__wmo_out="$__wmo_id"
	fi
	__ret="$__wmo_out"
}`,
	},
	{
		id:    "__wisp_builtin_map_result",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_map_result() {
	local __wmr_id __wmr_f __wmr_tag __wmr_val __wmr_out __wmr_ret
	__wmr_id="$1"
	__wmr_f="$2"
	eval "__wmr_tag=\$__wisp_s_${__wmr_id}_tag"
	if [ "$__wmr_tag" = ok ]; then
		eval "__wmr_val=\$__wisp_s_${__wmr_id}_value"
		"$__wmr_f" "$__wmr_val"
		__wmr_ret="$__ret"
		__wisp_alloc
		__wmr_out="$__ret"
		eval "__wisp_s_${__wmr_out}_tag=\$__wmr_tag"
		eval "__wisp_s_${__wmr_out}_value=\$__wmr_ret"
	else
		__wmr_out="$__wmr_id"
	fi
	__ret="$__wmr_out"
}`,
	},

	// --- filter (array/optional axes); mirrors genFilter/genFilterOptional ---
	{
		id:    "__wisp_builtin_filter_array",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_filter_array() {
	local __wfa_id __wfa_f __wfa_out __wfa_len __wfa_i __wfa_outlen __wfa_elem
	__wfa_id="$1"
	__wfa_f="$2"
	__wisp_alloc
	__wfa_out="$__ret"
	eval "__wfa_len=\$__wisp_a_${__wfa_id}_len"
	__wfa_i=0
	__wfa_outlen=0
	while [ "$__wfa_i" -lt "$__wfa_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wfa_elem=\$__wisp_a_${__wfa_id}_${__wfa_i}"
		"$__wfa_f" "$__wfa_elem"
		if [ "$__ret" = true ]; then
			eval "__wisp_a_${__wfa_out}_${__wfa_outlen}=\$__wfa_elem"
			__wfa_outlen=$(( __wfa_outlen + 1 ))
		fi
		__wfa_i=$(( __wfa_i + 1 ))
	done
	eval "__wisp_a_${__wfa_out}_len=\$__wfa_outlen"
	__ret="$__wfa_out"
}`,
	},
	{
		id:    "__wisp_builtin_filter_optional",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_filter_optional() {
	local __wfo_id __wfo_f __wfo_tag __wfo_val __wfo_keep __wfo_out
	__wfo_id="$1"
	__wfo_f="$2"
	eval "__wfo_tag=\$__wisp_s_${__wfo_id}_tag"
	if [ "$__wfo_tag" = some ]; then
		eval "__wfo_val=\$__wisp_s_${__wfo_id}_value"
		"$__wfo_f" "$__wfo_val"
		__wfo_keep="$__ret"
		if [ "$__wfo_keep" = true ]; then
			__wfo_out="$__wfo_id"
		else
			__wisp_alloc
			__wfo_out="$__ret"
			__wfo_tag=none
			eval "__wisp_s_${__wfo_out}_tag=\$__wfo_tag"
		fi
	else
		__wfo_out="$__wfo_id"
	fi
	__ret="$__wfo_out"
}`,
	},

	// --- each (array axis only); mirrors genEach ---
	{
		id:    "__wisp_builtin_each_array",
		order: 998,
		src: `__wisp_builtin_each_array() {
	local __wea_id __wea_f __wea_len __wea_i __wea_elem
	__wea_id="$1"
	__wea_f="$2"
	eval "__wea_len=\$__wisp_a_${__wea_id}_len"
	__wea_i=0
	while [ "$__wea_i" -lt "$__wea_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wea_elem=\$__wisp_a_${__wea_id}_${__wea_i}"
		"$__wea_f" "$__wea_elem"
		__wea_i=$(( __wea_i + 1 ))
	done
}`,
	},

	// --- reduce (array axis only); mirrors genReduce ---
	{
		id:    "__wisp_builtin_reduce_array",
		order: 998,
		src: `__wisp_builtin_reduce_array() {
	local __wre_id __wre_acc __wre_f __wre_len __wre_i __wre_elem
	__wre_id="$1"
	__wre_acc="$2"
	__wre_f="$3"
	eval "__wre_len=\$__wisp_a_${__wre_id}_len"
	__wre_i=0
	while [ "$__wre_i" -lt "$__wre_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wre_elem=\$__wisp_a_${__wre_id}_${__wre_i}"
		"$__wre_f" "$__wre_acc" "$__wre_elem"
		__wre_acc="$__ret"
		__wre_i=$(( __wre_i + 1 ))
	done
	__ret="$__wre_acc"
}`,
	},

	// --- sort_by (array axis only): a fresh handle, insertion sort driven by the
	// comparator funcref (cmp(a,b) true means a sorts before b). ---
	{
		id:    "__wisp_builtin_sort_by_array",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_sort_by_array() {
	local __wsb_id __wsb_f __wsb_out __wsb_len __wsb_i __wsb_j __wsb_key __wsb_cur __wsb_lt
	__wsb_id="$1"
	__wsb_f="$2"
	__wisp_alloc
	__wsb_out="$__ret"
	eval "__wsb_len=\$__wisp_a_${__wsb_id}_len"
	eval "__wisp_a_${__wsb_out}_len=\$__wsb_len"
	__wsb_i=0
	while [ "$__wsb_i" -lt "$__wsb_len" ]; do
		eval "__wsb_cur=\$__wisp_a_${__wsb_id}_${__wsb_i}"
		eval "__wisp_a_${__wsb_out}_${__wsb_i}=\$__wsb_cur"
		__wsb_i=$(( __wsb_i + 1 ))
	done
	__wsb_i=1
	while [ "$__wsb_i" -lt "$__wsb_len" ]; do
		eval "__wsb_key=\$__wisp_a_${__wsb_out}_${__wsb_i}"
		__wsb_j=$(( __wsb_i - 1 ))
		while [ "$__wsb_j" -ge 0 ]; do
			eval "__wsb_cur=\$__wisp_a_${__wsb_out}_${__wsb_j}"
			"$__wsb_f" "$__wsb_key" "$__wsb_cur"
			__wsb_lt="$__ret"
			if [ "$__wsb_lt" != true ]; then
				break
			fi
			eval "__wisp_a_${__wsb_out}_$(( __wsb_j + 1 ))=\$__wsb_cur"
			__wsb_j=$(( __wsb_j - 1 ))
		done
		eval "__wisp_a_${__wsb_out}_$(( __wsb_j + 1 ))=\$__wsb_key"
		__wsb_i=$(( __wsb_i + 1 ))
	done
	__ret="$__wsb_out"
}`,
	},

	// --- find (array axis only); mirrors genFind/genIntSentinelToOptional shape ---
	{
		id:    "__wisp_builtin_find_array",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_find_array() {
	local __wfi_id __wfi_f __wfi_len __wfi_i __wfi_elem __wfi_out __wfi_tag __wfi_res
	__wfi_id="$1"
	__wfi_f="$2"
	__wisp_alloc
	__wfi_out="$__ret"
	__wfi_tag=none
	eval "__wisp_s_${__wfi_out}_tag=\$__wfi_tag"
	eval "__wfi_len=\$__wisp_a_${__wfi_id}_len"
	__wfi_i=0
	while [ "$__wfi_i" -lt "$__wfi_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wfi_elem=\$__wisp_a_${__wfi_id}_${__wfi_i}"
		"$__wfi_f" "$__wfi_elem"
		__wfi_res="$__ret"
		if [ "$__wfi_res" = true ]; then
			__wfi_tag=some
			eval "__wisp_s_${__wfi_out}_tag=\$__wfi_tag"
			eval "__wisp_s_${__wfi_out}_value=\$__wfi_i"
			break
		fi
		__wfi_i=$(( __wfi_i + 1 ))
	done
	__ret="$__wfi_out"
}`,
	},

	// --- any/all (array axis only); mirrors genAnyAll (short-circuiting) ---
	{
		id:    "__wisp_builtin_any_array",
		order: 998,
		src: `__wisp_builtin_any_array() {
	local __wan_id __wan_f __wan_len __wan_i __wan_elem
	__wan_id="$1"
	__wan_f="$2"
	eval "__wan_len=\$__wisp_a_${__wan_id}_len"
	__wan_i=0
	__ret=false
	while [ "$__wan_i" -lt "$__wan_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wan_elem=\$__wisp_a_${__wan_id}_${__wan_i}"
		"$__wan_f" "$__wan_elem"
		if [ "$__ret" = true ]; then
			return
		fi
		__wan_i=$(( __wan_i + 1 ))
	done
	__ret=false
}`,
	},
	{
		id:    "__wisp_builtin_all_array",
		order: 998,
		src: `__wisp_builtin_all_array() {
	local __wal_id __wal_f __wal_len __wal_i __wal_elem
	__wal_id="$1"
	__wal_f="$2"
	eval "__wal_len=\$__wisp_a_${__wal_id}_len"
	__wal_i=0
	__ret=true
	while [ "$__wal_i" -lt "$__wal_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wal_elem=\$__wisp_a_${__wal_id}_${__wal_i}"
		"$__wal_f" "$__wal_elem"
		if [ "$__ret" != true ]; then
			__ret=false
			return
		fi
		__wal_i=$(( __wal_i + 1 ))
	done
	__ret=true
}`,
	},

	// --- count_where (array axis only); mirrors genCountWhere ---
	{
		id:    "__wisp_builtin_count_where_array",
		order: 998,
		src: `__wisp_builtin_count_where_array() {
	local __wcw_id __wcw_f __wcw_len __wcw_i __wcw_elem __wcw_acc
	__wcw_id="$1"
	__wcw_f="$2"
	eval "__wcw_len=\$__wisp_a_${__wcw_id}_len"
	__wcw_i=0
	__wcw_acc=0
	while [ "$__wcw_i" -lt "$__wcw_len" ]; do
		[ -n "$__wisp_err_pending" ] && break
		eval "__wcw_elem=\$__wisp_a_${__wcw_id}_${__wcw_i}"
		"$__wcw_f" "$__wcw_elem"
		if [ "$__ret" = true ]; then
			__wcw_acc=$(( __wcw_acc + 1 ))
		fi
		__wcw_i=$(( __wcw_i + 1 ))
	done
	__ret="$__wcw_acc"
}`,
	},

	// --- and_then (optional/result axes); mirrors genAndThenTagged ---
	{
		id:    "__wisp_builtin_and_then_optional",
		order: 998,
		src: `__wisp_builtin_and_then_optional() {
	local __wao_id __wao_f __wao_tag __wao_val __wao_out
	__wao_id="$1"
	__wao_f="$2"
	eval "__wao_tag=\$__wisp_s_${__wao_id}_tag"
	if [ "$__wao_tag" = some ]; then
		eval "__wao_val=\$__wisp_s_${__wao_id}_value"
		"$__wao_f" "$__wao_val"
		__wao_out="$__ret"
	else
		__wao_out="$__wao_id"
	fi
	__ret="$__wao_out"
}`,
	},
	{
		id:    "__wisp_builtin_and_then_result",
		order: 998,
		src: `__wisp_builtin_and_then_result() {
	local __war_id __war_f __war_tag __war_val __war_out
	__war_id="$1"
	__war_f="$2"
	eval "__war_tag=\$__wisp_s_${__war_id}_tag"
	if [ "$__war_tag" = ok ]; then
		eval "__war_val=\$__wisp_s_${__war_id}_value"
		"$__war_f" "$__war_val"
		__war_out="$__ret"
	else
		__war_out="$__war_id"
	fi
	__ret="$__war_out"
}`,
	},

	// --- or_else (optional/result axes); mirrors genOrElseOptional/genOrElseResult ---
	{
		id:    "__wisp_builtin_or_else_optional",
		order: 998,
		src: `__wisp_builtin_or_else_optional() {
	local __woo_id __woo_f __woo_tag __woo_out
	__woo_id="$1"
	__woo_f="$2"
	eval "__woo_tag=\$__wisp_s_${__woo_id}_tag"
	if [ "$__woo_tag" = some ]; then
		__woo_out="$__woo_id"
	else
		"$__woo_f"
		__woo_out="$__ret"
	fi
	__ret="$__woo_out"
}`,
	},
	{
		id:    "__wisp_builtin_or_else_result",
		order: 998,
		src: `__wisp_builtin_or_else_result() {
	local __wor_id __wor_f __wor_tag __wor_errid __wor_out
	__wor_id="$1"
	__wor_f="$2"
	eval "__wor_tag=\$__wisp_s_${__wor_id}_tag"
	if [ "$__wor_tag" = ok ]; then
		__wor_out="$__wor_id"
	else
		eval "__wor_errid=\$__wisp_s_${__wor_id}_value"
		"$__wor_f" "$__wor_errid"
		__wor_out="$__ret"
	fi
	__ret="$__wor_out"
}`,
	},

	// --- map_err (result axis only); mirrors genMapErrResult ---
	{
		id:    "__wisp_builtin_map_err_result",
		deps:  []string{Alloc},
		order: 998,
		src: `__wisp_builtin_map_err_result() {
	local __wme_id __wme_f __wme_tag __wme_errid __wme_newerr __wme_out
	__wme_id="$1"
	__wme_f="$2"
	eval "__wme_tag=\$__wisp_s_${__wme_id}_tag"
	if [ "$__wme_tag" = ok ]; then
		__wme_out="$__wme_id"
	else
		eval "__wme_errid=\$__wisp_s_${__wme_id}_value"
		"$__wme_f" "$__wme_errid"
		__wme_newerr="$__ret"
		__wisp_alloc
		__wme_out="$__ret"
		__wme_tag=err
		eval "__wisp_s_${__wme_out}_tag=\$__wme_tag"
		eval "__wisp_s_${__wme_out}_value=\$__wme_newerr"
	fi
	__ret="$__wme_out"
}`,
	},
}
