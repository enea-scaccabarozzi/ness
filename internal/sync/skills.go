package sync

import (
	"maps"
	"slices"
	"strings"
)

// ChangedSkills returns the sorted names of the skill directories (.agents/skills/<name>/…) whose
// entries differ between the two locks in hash, or exist in only one of them (R-UPD-070).
func ChangedSkills(from, to *Lock) []string {
	hashes := func(l *Lock) map[string]string {
		m := map[string]string{}
		for _, e := range l.Entries {
			if strings.HasPrefix(e.Path, skillsDir) {
				m[e.Path] = e.Hash
			}
		}
		return m
	}
	a, b := hashes(from), hashes(to)
	changed := map[string]bool{}
	note := func(x, y map[string]string) {
		for path, h := range x {
			if hy, ok := y[path]; !ok || hy != h {
				name, _, _ := strings.Cut(strings.TrimPrefix(path, skillsDir), "/")
				changed[name] = true
			}
		}
	}
	note(a, b)
	note(b, a)
	return slices.Sorted(maps.Keys(changed))
}
