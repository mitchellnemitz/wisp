package pkgtool

import "testing"

// rootDeps builds a root dependency map.
func dep(git, ref string) dependency { return dependency{Git: git, Ref: ref} }

func TestResolveDiamondRootOverride(t *testing.T) {
	// root -> a, b ; a -> shared@cS1 ; b -> shared@cS2 ; root -> shared@cS3.
	// Root override: shared resolves to the root's pin (cS3), not a conflict.
	f := &fakeFetcher{
		resolve: map[refKey]string{
			{"https://ex/ga.git", "ra"}: hex40("a"), {"https://ex/gb.git", "rb"}: hex40("b"),
			{"https://ex/gs.git", "r1"}: hex40("1"), {"https://ex/gs.git", "r2"}: hex40("2"), {"https://ex/gs.git", "r3"}: hex40("3"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp","dependencies":{"o/shared":{"git":"https://ex/gs.git","ref":"r1"}}}`},
			hex40("b"): {"wisp.json": `{"name":"b","main":"m.wisp","dependencies":{"o/shared":{"git":"https://ex/gs.git","ref":"r2"}}}`},
			hex40("1"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
			hex40("2"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
			hex40("3"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
		},
	}
	root := map[string]dependency{
		"o/a": dep("https://ex/ga.git", "ra"), "o/b": dep("https://ex/gb.git", "rb"), "o/shared": dep("https://ex/gs.git", "r3"),
	}
	got, err := resolveGraph(f, t.TempDir(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["o/shared"].commit != hex40("3") {
		t.Errorf("root override failed: shared = %s want %s", got["o/shared"].commit, hex40("3"))
	}
}

func TestResolveTransitiveConflictHardError(t *testing.T) {
	// a -> shared@c1 ; b -> shared@c2 ; root pins neither shared. Conflict.
	f := &fakeFetcher{
		resolve: map[refKey]string{
			{"https://ex/ga.git", "ra"}: hex40("a"), {"https://ex/gb.git", "rb"}: hex40("b"),
			{"https://ex/gs.git", "r1"}: hex40("1"), {"https://ex/gs.git", "r2"}: hex40("2"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp","dependencies":{"o/shared":{"git":"https://ex/gs.git","ref":"r1"}}}`},
			hex40("b"): {"wisp.json": `{"name":"b","main":"m.wisp","dependencies":{"o/shared":{"git":"https://ex/gs.git","ref":"r2"}}}`},
			hex40("1"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
			hex40("2"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
		},
	}
	root := map[string]dependency{"o/a": dep("https://ex/ga.git", "ra"), "o/b": dep("https://ex/gb.git", "rb")}
	_, err := resolveGraph(f, t.TempDir(), root, nil)
	if err == nil {
		t.Fatal("expected transitive-conflict error")
	}
	for _, sub := range []string{"o/shared", "o/a", "o/b"} {
		if !contains(err.Error(), sub) {
			t.Errorf("conflict error %q missing %q", err.Error(), sub)
		}
	}
}

func TestResolveCycleTerminates(t *testing.T) {
	// a -> b -> a. Visit-once terminates.
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://ex/ga.git", "ra"}: hex40("a"), {"https://ex/gb.git", "rb"}: hex40("b")},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp","dependencies":{"o/b":{"git":"https://ex/gb.git","ref":"rb"}}}`},
			hex40("b"): {"wisp.json": `{"name":"b","main":"m.wisp","dependencies":{"o/a":{"git":"https://ex/ga.git","ref":"ra"}}}`},
		},
	}
	root := map[string]dependency{"o/a": dep("https://ex/ga.git", "ra")}
	got, err := resolveGraph(f, t.TempDir(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("cycle did not terminate cleanly: %d modules", len(got))
	}
}

func TestResolveRootOverrideIgnoresTransitivePinAndItsDeps(t *testing.T) {
	// root -> a, shared@cS3 ; a -> shared@cS1 ; shared@cS1's tree itself declares a
	// dep o/leaf, while the root-pinned shared@cS3 declares NO deps. The root pin wins
	// regardless of visit order, AND the transitive shared@cS1 must never be fetched or
	// walked -- so o/leaf (only reachable via the transitive shared) must NOT appear.
	f := &fakeFetcher{
		resolve: map[refKey]string{
			{"https://ex/ga.git", "ra"}: hex40("a"),
			{"https://ex/gs.git", "r1"}: hex40("1"), {"https://ex/gs.git", "r3"}: hex40("3"),
			{"https://ex/gl.git", "rl"}: hex40("c"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp","dependencies":{"o/shared":{"git":"https://ex/gs.git","ref":"r1"}}}`},
			hex40("1"): {"wisp.json": `{"name":"shared","main":"m.wisp","dependencies":{"o/leaf":{"git":"https://ex/gl.git","ref":"rl"}}}`},
			hex40("3"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
			hex40("c"): {"wisp.json": `{"name":"leaf","main":"m.wisp"}`},
		},
	}
	root := map[string]dependency{"o/a": dep("https://ex/ga.git", "ra"), "o/shared": dep("https://ex/gs.git", "r3")}
	for i := 0; i < 20; i++ { // map order is random; assert invariant across orderings
		got, err := resolveGraph(f, t.TempDir(), root, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got["o/shared"].commit != hex40("3") {
			t.Errorf("root override failed: shared = %s want %s", got["o/shared"].commit, hex40("3"))
		}
		if _, ok := got["o/leaf"]; ok {
			t.Errorf("transitive shared@r1's dep o/leaf must not be walked when root pin wins")
		}
	}
}

func TestResolveOrderIndependentConflict(t *testing.T) {
	// Same conflict graph as above; run with two different root map orderings
	// (Go map order is random, so just run many times) -> always errors.
	for i := 0; i < 20; i++ {
		f := &fakeFetcher{
			resolve: map[refKey]string{
				{"https://ex/ga.git", "ra"}: hex40("a"), {"https://ex/gb.git", "rb"}: hex40("b"),
				{"https://ex/gs.git", "r1"}: hex40("1"), {"https://ex/gs.git", "r2"}: hex40("2"),
			},
			trees: map[string]map[string]string{
				hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp","dependencies":{"o/shared":{"git":"https://ex/gs.git","ref":"r1"}}}`},
				hex40("b"): {"wisp.json": `{"name":"b","main":"m.wisp","dependencies":{"o/shared":{"git":"https://ex/gs.git","ref":"r2"}}}`},
				hex40("1"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
				hex40("2"): {"wisp.json": `{"name":"shared","main":"m.wisp"}`},
			},
		}
		root := map[string]dependency{"o/a": dep("https://ex/ga.git", "ra"), "o/b": dep("https://ex/gb.git", "rb")}
		if _, err := resolveGraph(f, t.TempDir(), root, nil); err == nil {
			t.Fatal("conflict not detected on some ordering")
		}
	}
}
