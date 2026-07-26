// internal/fuzz/rng.go
package fuzz

import "math/rand"

// RNG is a deterministic pseudo-random source: every value is a pure function of
// the seed; nothing reads wall-clock/env/ambient state (FR-016).
type RNG struct{ r *rand.Rand }

func NewRNG(seed int64) *RNG { return &RNG{r: rand.New(rand.NewSource(seed))} }

func (g *RNG) Intn(n int) int   { return g.r.Intn(n) }
func (g *RNG) Pick(n int) int   { return g.r.Intn(n) }
func (g *RNG) Int64() int64     { return int64(g.r.Uint64()) }
func (g *RNG) Float64() float64 { return g.r.Float64() }
func (g *RNG) Bool() bool       { return g.r.Intn(2) == 0 }
