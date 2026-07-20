// Package pkgtool implements the wisp package tool: the add/install/remove
// subcommands that populate <root>/.wisp/modules/ from a wisp.json dependencies
// map and a wisp.lock. All network, git, and os/exec live here; the compiler
// (internal/module, internal/driver) never imports this package.
package pkgtool

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// dependency is one wisp.json dependencies entry. Always object form.
type dependency struct {
	Git string `json:"git"`
	Ref string `json:"ref"`
}

// manifest models the fields of wisp.json the tool reads and writes. The loader's
// own manifest struct (internal/module) parses only name/main and ignores the rest;
// this struct adds dependencies. Name/Main are preserved verbatim on rewrite.
// extra holds every top-level key not explicitly modeled, so a round-trip
// through add/remove never silently drops fields like "version" or "author".
type manifest struct {
	Name         string                `json:"name"`
	Main         string                `json:"main"`
	Dependencies map[string]dependency `json:"dependencies,omitempty"`
	extra        map[string]json.RawMessage
}

func readManifest(path string) (*manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Decode the whole file into a raw map first so unknown top-level keys are
	// retained and written back unchanged (L7 fix). Then pull out the three known
	// fields, validating dependencies as object-form only (spec 4.1 / AC6).
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(b, &rawMap); err != nil {
		return nil, fmt.Errorf("malformed wisp.json: %w", err)
	}

	m := &manifest{extra: make(map[string]json.RawMessage)}

	// name
	if v, ok := rawMap["name"]; ok {
		if err := json.Unmarshal(v, &m.Name); err != nil {
			return nil, fmt.Errorf("malformed wisp.json: \"name\" must be a string: %w", err)
		}
	}
	// main
	if v, ok := rawMap["main"]; ok {
		if err := json.Unmarshal(v, &m.Main); err != nil {
			return nil, fmt.Errorf("malformed wisp.json: \"main\" must be a string: %w", err)
		}
	}
	// dependencies
	if v, ok := rawMap["dependencies"]; ok {
		var rawDeps map[string]json.RawMessage
		if err := json.Unmarshal(v, &rawDeps); err != nil {
			return nil, fmt.Errorf("malformed wisp.json: \"dependencies\" must be an object: %w", err)
		}
		m.Dependencies = make(map[string]dependency, len(rawDeps))
		for key, rm := range rawDeps {
			if _, _, err := module.SplitRepo(key); err != nil {
				return nil, fmt.Errorf("invalid dependency key %q: %v", key, err)
			}
			var d dependency
			if err := json.Unmarshal(rm, &d); err != nil {
				return nil, fmt.Errorf("dependency %q must be an object {\"git\":...,\"ref\":...}: %w", key, err)
			}
			if d.Git == "" {
				return nil, fmt.Errorf("dependency %q is missing a non-empty \"git\"", key)
			}
			if d.Ref == "" {
				return nil, fmt.Errorf("dependency %q is missing a non-empty \"ref\"", key)
			}
			// Validate the git URL against the transport allowlist and the ref syntax at
			// LOAD time, so a malformed manifest is rejected at read -- not deep in install.
			// (Same validators resolveGraph applies per dependency before any git argv use.)
			if err := validateGitURL(d.Git); err != nil {
				return nil, fmt.Errorf("dependency %q: %v", key, err)
			}
			if err := validateRef(d.Ref); err != nil {
				return nil, fmt.Errorf("dependency %q: %v", key, err)
			}
			m.Dependencies[key] = d
		}
	}

	// Stash every other top-level key verbatim.
	known := map[string]bool{"name": true, "main": true, "dependencies": true}
	for k, v := range rawMap {
		if !known[k] {
			m.extra[k] = v
		}
	}

	return m, nil
}

func writeManifest(path string, m *manifest) error {
	// Build the output map from the known fields plus any extras, so unknown
	// top-level keys survive a round-trip through add/remove.
	out := make(map[string]json.RawMessage, 3+len(m.extra))
	for k, v := range m.extra {
		out[k] = v
	}
	nameB, _ := json.Marshal(m.Name)
	out["name"] = nameB
	mainB, _ := json.Marshal(m.Main)
	out["main"] = mainB
	if len(m.Dependencies) > 0 {
		depsB, err := json.Marshal(m.Dependencies)
		if err != nil {
			return err
		}
		out["dependencies"] = depsB
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeFileAtomic(path, b)
}
