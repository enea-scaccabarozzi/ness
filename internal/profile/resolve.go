package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Part is one profile's text for a slot.
type Part struct {
	Owner string
	Text  []byte
}

// Block is a profile's managed block in AGENTS.md, named "<profile>/<block>" (R-SYNC-082).
type Block struct {
	Name  string
	Owner string
	Text  []byte
}

// OwnedFile is a file a profile renders into the repository.
type OwnedFile struct {
	Path    string
	Owner   string
	Content []byte
	Seed    bool
}

// Resolved is what sync renders: the profiles combined, with conflicts rejected.
type Resolved struct {
	Profiles []*Profile        // application order: as given, local last
	Lists    map[string][]Part // list slot -> parts in profile order
	Singles  map[string]Part   // single slot -> the winning part
	Blocks   []Block           // AGENTS.md blocks in profile order
	Files    []OwnedFile       // sorted by path
	Defaults map[string]any    // config defaults from [config] and [atoms]
	Features []string          // features the profiles enable
	Tools    map[string]string // required tool -> first profile requiring it
	Sizes    []string          // quick, full, then each profile's sizes in profile order (R-PROF-085)
	Checks   []Check           // profile order, local last, ids prefixed "<profile>/" (R-PROF-047)
}

type setter struct {
	part  Part
	local bool
}

type valueBy struct {
	owner, label string
	value        any
}

// Resolve combines profiles in order with the local profile last (R-PROF-020, R-PROF-021).
// defined reports the dotted keys .ness/ness.toml sets; they settle config and seam conflicts.
// There is no "last wins" between other profiles (R-PROF-034).
func Resolve(ps []*Profile, defined func(string) bool) (*Resolved, error) {
	ordered := localLast(ps)
	r := &Resolved{
		Profiles: ordered, Lists: map[string][]Part{}, Singles: map[string]Part{},
		Defaults: map[string]any{}, Tools: map[string]string{}, Sizes: []string{"quick", "full"},
	}
	errs := make([]error, 0, 2*len(ordered)+3)
	errs = append(errs, checkRelations(ordered))
	setters := map[string][]setter{}
	owners := map[string]string{}
	values := map[string][]valueBy{}
	for _, p := range ordered {
		errs = append(errs, r.addSlots(p, setters), r.addFiles(p, owners))
		addDefaults(p, values)
		for _, f := range p.Enable {
			if !slices.Contains(r.Features, f) {
				r.Features = append(r.Features, f)
			}
		}
		errs = append(errs, r.addSizes(p))
		for _, c := range p.Checks {
			c.ID = p.Name + "/" + c.ID
			r.Checks = append(r.Checks, c)
		}
		for _, tool := range p.RequiresTools {
			if _, ok := r.Tools[tool]; !ok {
				r.Tools[tool] = p.Name
			}
		}
	}
	errs = append(errs, r.pickSingles(setters), r.pickDefaults(values, defined), r.checkSizes())
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	slices.SortFunc(r.Files, func(a, b OwnedFile) int { return strings.Compare(a.Path, b.Path) })
	return r, nil
}

// Versions maps each applied profile to its recorded version.
func (r *Resolved) Versions() map[string]string {
	out := map[string]string{}
	for _, p := range r.Profiles {
		out[p.Name] = p.Version
	}
	return out
}

func isLocal(p *Profile) bool { return p.Local && p.Name == "local" }

func localLast(ps []*Profile) []*Profile {
	out := make([]*Profile, 0, len(ps))
	var local *Profile
	for _, p := range ps {
		if isLocal(p) {
			local = p
			continue
		}
		out = append(out, p)
	}
	if local != nil {
		out = append(out, local)
	}
	return out
}

func checkRelations(ps []*Profile) error {
	var errs []error
	names := map[string]bool{}
	for _, p := range ps {
		if names[p.Name] {
			errs = append(errs, fmt.Errorf("profile %s is applied twice", p.Name))
		}
		names[p.Name] = true
	}
	for _, p := range ps {
		for _, req := range p.Requires {
			if !names[req] {
				errs = append(errs, fmt.Errorf("profile %s requires profile %s; add it to profiles in .ness/ness.toml",
					p.Name, req))
			}
		}
		for _, c := range p.Conflicts {
			if names[c] {
				errs = append(errs, fmt.Errorf("profiles %s and %s conflict; apply only one", p.Name, c))
			}
		}
	}
	return errors.Join(errs...)
}

func (r *Resolved) addSlots(p *Profile, setters map[string][]setter) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(p.Slots)) {
		s := p.Slots[key]
		text, err := fs.ReadFile(p.Dir, s.Append+s.Set+s.Block)
		if err != nil {
			errs = append(errs, fmt.Errorf("profile %s: slot %s: %w", p.Name, key, err))
			continue
		}
		part := Part{Owner: p.Name, Text: text}
		switch {
		case s.Append != "":
			r.Lists[key] = append(r.Lists[key], part)
		case s.Set != "":
			setters[key] = append(setters[key], setter{part: part, local: isLocal(p)})
		default:
			_, name, _ := strings.Cut(key, "#")
			r.Blocks = append(r.Blocks, Block{Name: p.Name + "/" + name, Owner: p.Name, Text: text})
		}
	}
	return errors.Join(errs...)
}

// DefaultSize is the size a root takes when none is asked: the last size a profile added, else full.
func (r *Resolved) DefaultSize() string { return r.Sizes[len(r.Sizes)-1] }

// checkSizes refuses a check gated to a size nobody declares: it would never run (R-PROF-084).
func (r *Resolved) checkSizes() error {
	var errs []error
	for _, c := range r.Checks {
		for _, sz := range c.Size {
			if !slices.Contains(r.Sizes, sz) {
				errs = append(errs, fmt.Errorf("check %s: size %q is not one of: %s",
					c.ID, sz, strings.Join(r.Sizes, ", ")))
			}
		}
	}
	return errors.Join(errs...)
}

// addSizes adds p's sizes; a name core or an earlier profile already owns is a conflict.
func (r *Resolved) addSizes(p *Profile) error {
	var errs []error
	for _, sz := range p.Sizes {
		if slices.Contains(r.Sizes, sz) {
			errs = append(errs, fmt.Errorf("profile %s: size %s is already defined (quick and full are core's; "+
				"each profile size is owned by one profile)", p.Name, sz))
			continue
		}
		r.Sizes = append(r.Sizes, sz)
	}
	return errors.Join(errs...)
}

func (r *Resolved) addFiles(p *Profile, owners map[string]string) error {
	var errs []error
	for _, path := range slices.Sorted(maps.Keys(p.Files)) {
		if prev, ok := owners[path]; ok {
			errs = append(errs, fmt.Errorf("profiles %s and %s both own %s", prev, p.Name, path))
			continue
		}
		owners[path] = p.Name
		f := p.Files[path]
		content, err := fs.ReadFile(p.Dir, f.Own+f.Seed)
		if err != nil {
			errs = append(errs, fmt.Errorf("profile %s: file %s: %w", p.Name, path, err))
			continue
		}
		r.Files = append(r.Files, OwnedFile{Path: path, Owner: p.Name, Content: content, Seed: f.Seed != ""})
	}
	return errors.Join(errs...)
}

func addDefaults(p *Profile, values map[string][]valueBy) {
	for _, key := range slices.Sorted(maps.Keys(p.Config)) {
		values[key] = append(values[key], valueBy{owner: p.Name, label: key, value: p.Config[key]})
	}
	for _, name := range slices.Sorted(maps.Keys(p.Atoms)) {
		key := atomSeams[name].key
		values[key] = append(values[key], valueBy{owner: p.Name, label: "seam " + name, value: p.Atoms[name]})
	}
}

// pickSingles lets the local profile override a single slot; otherwise two setters conflict.
func (r *Resolved) pickSingles(setters map[string][]setter) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(setters)) {
		ss := setters[key]
		if i := slices.IndexFunc(ss, func(s setter) bool { return s.local }); i >= 0 {
			r.Singles[key] = ss[i].part
			continue
		}
		if len(ss) > 1 {
			names := make([]string, len(ss))
			for i, s := range ss {
				names[i] = s.part.Owner
			}
			errs = append(errs, fmt.Errorf("profiles %s all set single slot %s; only the local profile may override it",
				strings.Join(names, ", "), key))
			continue
		}
		r.Singles[key] = ss[0].part
	}
	return errors.Join(errs...)
}

// pickDefaults keeps one value per config key; keys the file sets need no default.
func (r *Resolved) pickDefaults(values map[string][]valueBy, defined func(string) bool) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if defined(key) {
			continue
		}
		vs := values[key]
		for _, v := range vs[1:] {
			if !reflect.DeepEqual(v.value, vs[0].value) {
				errs = append(errs, fmt.Errorf(
					"profiles %s and %s set %s differently (%v, %v); set %s in .ness/ness.toml",
					vs[0].owner, v.owner, v.label, vs[0].value, v.value, key))
			}
		}
		r.Defaults[key] = vs[0].value
	}
	return errors.Join(errs...)
}
