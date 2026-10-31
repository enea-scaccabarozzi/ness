// Package ladder orders the tier table into rungs, cheapest first, and clamps moves on it to the base and ceiling.
package ladder

import (
	"slices"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// Rung is one step of the ladder: a tier name and its model and effort for one client.
type Rung struct{ Name, Model, Effort string }

// Ladder is the project's rungs for one client, with the base and ceiling as indices.
type Ladder struct {
	Rungs         []Rung
	Base, Ceiling int
}

// New builds the ladder of a validated config for client (R-MOD-005..007).
func New(cfg *config.Config, client string) Ladder {
	m := cfg.Models
	l := Ladder{Base: slices.Index(m.Ladder, m.Base), Ceiling: len(m.Ladder) - 1}
	if m.Ceiling != "" {
		l.Ceiling = slices.Index(m.Ladder, m.Ceiling)
	}
	for _, name := range m.Ladder {
		spec := cfg.Tiers[name].Claude
		if client == "codex" {
			spec = cfg.Tiers[name].Codex
		}
		model, effort := config.SplitTier(spec)
		l.Rungs = append(l.Rungs, Rung{Name: name, Model: model, Effort: effort})
	}
	return l
}

// At is the rung offset steps from base, clamped to [0, Ceiling]; false when it clamped.
func (l Ladder) At(offset int) (Rung, bool) {
	n := len(l.Rungs)
	i, ok := l.clamp(l.Base + min(max(offset, -n), n)) // bounded first: base+MaxInt would wrap
	return l.Rungs[i], ok
}

// Index is the position of the rung named name.
func (l Ladder) Index(name string) (int, bool) {
	i := slices.IndexFunc(l.Rungs, func(r Rung) bool { return r.Name == name })
	return i, i >= 0
}

// Step is the rung delta steps from the one named name, clamped to [0, Ceiling]; false when it
// clamped or name is not on the ladder.
func (l Ladder) Step(name string, delta int) (Rung, bool) {
	from, found := l.Index(name)
	if !found {
		from = l.Base
	}
	i, ok := l.clamp(from + delta)
	return l.Rungs[i], ok && found
}

// Find is the position of the rung with exactly this model and effort.
func (l Ladder) Find(model, effort string) (int, bool) {
	i := slices.IndexFunc(l.Rungs, func(r Rung) bool { return r.Model == model && r.Effort == effort })
	return i, i >= 0
}

func (l Ladder) clamp(i int) (int, bool) {
	c := min(max(i, 0), l.Ceiling)
	return c, c == i
}
