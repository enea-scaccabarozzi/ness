package sync

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

// Ref is a pointer to resolve, with the name of the declaration that carries it.
type Ref struct {
	Name string // how the refusal names the declaration, e.g. "check local/x"
	rule.Pointer
}

// ResolveRefs checks every ref against the skills the entries render
// (.agents/skills/<skill>/SKILL.md), falling back to the file on disk for a skill the project
// owns. It returns one error naming each ref that does not land, with the headings that exist
// (R-PROF-046).
func ResolveRefs(entries []Entry, root *os.Root, refs []Ref) error {
	skills := map[string][]string{}
	for _, e := range entries {
		if e.Kind == KindFile || e.Kind == KindSeed {
			if skill, ok := skillOf(e.Path); ok {
				skills[skill] = rule.Headings(e.Content)
			}
		}
	}
	var errs []error
	for _, r := range refs {
		if _, ok := skills[r.Skill]; !ok {
			if b, err := root.ReadFile(path.Join(skillsDir, r.Skill, "SKILL.md")); err == nil {
				skills[r.Skill] = rule.Headings(b)
			}
		}
		if err := rule.Resolve(skills, r.Pointer); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
		}
	}
	return errors.Join(errs...)
}

// skillOf returns the skill name of an .agents/skills/<skill>/SKILL.md path.
func skillOf(p string) (string, bool) {
	dir, file := path.Split(p)
	if file != "SKILL.md" || path.Dir(path.Clean(dir)) != path.Clean(skillsDir) {
		return "", false
	}
	return path.Base(path.Clean(dir)), true
}
