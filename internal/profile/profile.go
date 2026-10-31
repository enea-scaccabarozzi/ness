// Package profile loads profiles (the embedded core, built-ins and the project's local ones),
// validates them and resolves how they combine, refusing conflicts before anything is written.
package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// SizeRe is the syntax of a work size a profile adds.
var SizeRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// Profile is one loaded profile.
type Profile struct {
	Name          string
	Summary       string
	Requires      []string
	Conflicts     []string
	RequiresTools []string
	Slots         map[string]Slot
	Files         map[string]File
	Config        map[string]any
	Enable        []string
	Sizes         []string // work sizes the profile adds beside quick and full (R-PROF-085)
	Atoms         map[string]string
	Version       string
	Local         bool
	Dir           fs.FS
	Checks        []Check // the [[checks]] declarations, in file order, ids as declared
}

// Check is one [[checks]] declaration (R-PROF-045). Exactly one of Kind and Run is set.
type Check struct {
	// ID is unique within its profile; Resolved.Checks carries it prefixed "<profile>/".
	ID string
	// On is the event the check runs at: "open", "integrate" or "close".
	On string
	// For limits the check to "child" or "root" sessions; empty means both.
	For string
	// Skill and Section are the pointer appended to the refusal; sync resolves it.
	Skill, Section string
	// Message is the refusal text; required with Kind, optional with Run.
	Message string
	// Kind names a built-in check: section-matches, branch-forbids, match-last or
	// checkpoint-since-landing. Empty when Run is set.
	Kind string
	// Args holds the kind's string arguments by key: file, heading, path, pattern,
	// against_file, against_pattern. It excludes require. Nil for Run and for kinds without args.
	Args map[string]string
	// Require is the section-matches regexp list; nil for every other kind.
	Require []string
	// Run is the shell command of a script check. Empty when Kind is set.
	Run string
	// Timeout bounds a Run check; zero when undeclared (the runner applies its default).
	Timeout time.Duration
	// Size limits the check to roots of these work sizes; empty means every size (R-PROF-084).
	Size []string
}

// Slot is one contribution: exactly one of Append (list slot), Set (single slot) or Block (a
// block in AGENTS.md), each naming a file in the profile directory.
type Slot struct {
	Append string `toml:"append"`
	Set    string `toml:"set"`
	Block  string `toml:"block"`
}

// File is a file the profile renders: Own (kept in sync) or Seed (created once, then yours).
type File struct {
	Own  string `toml:"own"`
	Seed string `toml:"seed"`
}

type manifest struct {
	Name          string            `toml:"name"`
	Summary       string            `toml:"summary"`
	Requires      []string          `toml:"requires"`
	Conflicts     []string          `toml:"conflicts"`
	RequiresTools []string          `toml:"requires_tools"`
	Slots         map[string]Slot   `toml:"slots"`
	Files         map[string]File   `toml:"files"`
	Config        map[string]any    `toml:"config"`
	Features      features          `toml:"features"`
	Atoms         map[string]string `toml:"atoms"`
	Checks        []map[string]any  `toml:"checks"`
	Sizes         []string          `toml:"sizes"`
}

type features struct {
	Enable []string `toml:"enable"`
}

type seam struct {
	key   string
	atoms []string
}

// atomSeams maps each seam a profile may choose to its config key and atoms (R-PROF-009).
var atomSeams = map[string]seam{
	"client":       {"client", config.Clients},
	"integrator":   {"integrate.mode", config.IntegrateModes},
	"model_policy": {"models.policy", config.ModelPolicies},
}

// Parse decodes and validates the profile.toml of directory dirName (R-PROF-001, R-PROF-005).
func Parse(dirName string, data []byte, dir fs.FS) (*Profile, error) {
	where := "profile " + dirName + ": profile.toml"
	var m manifest
	md, err := toml.Decode(string(data), &m)
	if err != nil {
		var pe toml.ParseError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("%s:%d:%d: %s", where, pe.Position.Line, pe.Position.Col, pe.Message)
		}
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if und := md.Undecoded(); len(und) > 0 {
		return nil, fmt.Errorf("%s: unknown field %s", where, und[0])
	}
	p := &Profile{
		Name: m.Name, Summary: m.Summary, Requires: m.Requires, Conflicts: m.Conflicts,
		RequiresTools: m.RequiresTools, Slots: m.Slots, Files: m.Files, Config: m.Config,
		Enable: m.Features.Enable, Atoms: m.Atoms, Dir: dir, Sizes: m.Sizes,
	}
	var cerr error
	p.Checks, cerr = parseChecks(m.Checks)
	if err := errors.Join(p.validate(dirName), cerr); err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	return p, nil
}

func (p *Profile) validate(dirName string) error {
	var errs []error
	if p.Name != dirName {
		errs = append(errs, fmt.Errorf("name %q differs from its directory %q", p.Name, dirName))
	}
	for _, key := range slices.Sorted(maps.Keys(p.Slots)) {
		errs = append(errs, p.checkSlot(key, p.Slots[key]))
	}
	for _, path := range slices.Sorted(maps.Keys(p.Files)) {
		errs = append(errs, p.checkFile(path, p.Files[path]))
	}
	for _, f := range p.Enable {
		if !slices.Contains(config.FeatureNames, f) {
			errs = append(errs, fmt.Errorf("features.enable: %q is not a feature (%s)",
				f, strings.Join(config.FeatureNames, ", ")))
		}
	}
	for _, sz := range p.Sizes {
		if !SizeRe.MatchString(sz) {
			errs = append(errs, fmt.Errorf("sizes: %q is not a size name ([a-z][a-z0-9-]{0,31})", sz))
		}
	}
	errs = append(errs, p.checkAtoms()...)
	for _, key := range slices.Sorted(maps.Keys(p.Config)) {
		if err := config.ValidKey(key, p.Config[key]); err != nil {
			errs = append(errs, fmt.Errorf("config %q: %w", key, err))
		}
	}
	return errors.Join(errs...)
}

func (p *Profile) checkAtoms() []error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(p.Atoms)) {
		s, ok := atomSeams[name]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("atoms: unknown seam %q", name))
		case !slices.Contains(s.atoms, p.Atoms[name]):
			errs = append(errs, fmt.Errorf("atoms.%s = %q: accepted values are %s",
				name, p.Atoms[name], strings.Join(s.atoms, ", ")))
		}
	}
	return errs
}

func (p *Profile) checkSlot(key string, s Slot) error {
	target, name, ok := strings.Cut(key, "#")
	if !ok || target == "" || name == "" {
		return fmt.Errorf("slot %q: want \"<skill-or-file>#<slot>\"", key)
	}
	n, file := 0, ""
	for _, v := range []string{s.Append, s.Set, s.Block} {
		if v != "" {
			n, file = n+1, v
		}
	}
	if n != 1 {
		return fmt.Errorf("slot %q: set exactly one of append, set, block", key)
	}
	if (s.Block != "") != (target == "AGENTS.md") {
		return fmt.Errorf("slot %q: AGENTS.md slots take a block, and a block goes only in AGENTS.md", key)
	}
	return p.exists(file)
}

func (p *Profile) checkFile(path string, f File) error {
	if (f.Own == "") == (f.Seed == "") {
		return fmt.Errorf("file %q: set exactly one of own, seed", path)
	}
	if !fs.ValidPath(path) || path == "." {
		return fmt.Errorf("file %q: want a clean relative path inside the repository", path)
	}
	return p.exists(f.Own + f.Seed)
}

func (p *Profile) exists(name string) error {
	if _, err := fs.Stat(p.Dir, name); err != nil {
		return fmt.Errorf("references %s, which is missing from the profile directory", name)
	}
	return nil
}

// Source finds profiles by name: built-ins embedded in the binary and local ones under
// .ness/profiles (R-PROF-003).
type Source struct {
	Builtin fs.FS  // one directory per built-in profile
	Local   fs.FS  // the repository's .ness/profiles; nil when absent
	Version string // the binary version, recorded for built-ins
}

// Load loads one profile by name. A local profile may not reuse a built-in name (R-PROF-042).
func (s Source) Load(name string) (*Profile, error) {
	builtin := hasDir(s.Builtin, name)
	local := s.Local != nil && hasDir(s.Local, name)
	switch {
	case builtin && local:
		return nil, fmt.Errorf("local profile .ness/profiles/%s has the name of a built-in profile; rename it", name)
	case builtin:
		return load(s.Builtin, name, false, s.Version)
	case local:
		return load(s.Local, name, true, "")
	}
	return nil, fmt.Errorf("unknown profile %q (not built in, and no .ness/profiles/%s)", name, name)
}

// LoadCore loads the embedded core profile.
func LoadCore(core fs.FS, version string) (*Profile, error) {
	data, err := fs.ReadFile(core, "profile.toml")
	if err != nil {
		return nil, fmt.Errorf("core profile: %w", err)
	}
	p, err := Parse("core", data, core)
	if err != nil {
		return nil, err
	}
	p.Version = version
	return p, nil
}

func hasDir(fsys fs.FS, name string) bool {
	if fsys == nil || !fs.ValidPath(name) {
		return false
	}
	info, err := fs.Stat(fsys, name)
	return err == nil && info.IsDir()
}

func load(fsys fs.FS, name string, local bool, version string) (*Profile, error) {
	dir, err := fs.Sub(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("profile %s: %w", name, err)
	}
	data, err := fs.ReadFile(dir, "profile.toml")
	if err != nil {
		return nil, fmt.Errorf("profile %s: %w", name, err)
	}
	p, err := Parse(name, data, dir)
	if err != nil {
		return nil, err
	}
	p.Local, p.Version = local, version
	if local {
		if p.Version, err = dirHash(dir); err != nil {
			return nil, fmt.Errorf("profile %s: %w", name, err)
		}
	}
	return p, nil
}

// dirHash is a local profile's version: SHA-256 over its sorted paths and bytes (R-PROF-043).
func dirHash(dir fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(dir, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(dir, p)
		if err != nil {
			return err
		}
		_, _ = h.Write(fmt.Appendf(nil, "%s\x00%d\x00", p, len(b)))
		_, _ = h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}
