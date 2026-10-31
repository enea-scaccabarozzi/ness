package session

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// TestedHerdr is the herdr version this binary was built against (R-SESS-016).
const TestedHerdr = "0.8.2"

// Env is what the environment checks of design §4 need. Check runs them in order and returns
// the first refusal; nothing has been written when it returns (R-SESS-010).
type Env struct {
	Runtime       Runtime
	Client        Client
	Tiers         map[string]config.Tier
	ProjectSkills string   // <root>/.agents/skills
	SkillRoots    []string // SkillRoots(home) in production; injected in tests (R-SESS-006)
}

// CheckHerdr refuses when herdr is missing or below the tested version (R-SESS-001, R-SESS-002,
// R-WRK-007).
func CheckHerdr(rt Runtime) error {
	v, err := rt.Version()
	if errors.Is(err, ErrNotInstalled) {
		return fmt.Errorf("herdr is required and is not installed; install herdr %s or newer", TestedHerdr)
	}
	if err != nil {
		return err
	}
	return CheckVersion("herdr", v, TestedHerdr)
}

// Check refuses when herdr is missing or old (R-SESS-001, R-SESS-002), the client is missing or
// old (R-SESS-003), a tier's model does not resolve (R-SESS-004) or a global skill shadows a
// project skill (R-SESS-005).
func (e Env) Check() error {
	if err := CheckHerdr(e.Runtime); err != nil {
		return err
	}
	cv, err := e.Client.Version()
	if errors.Is(err, ErrNotInstalled) {
		return fmt.Errorf("%s is not installed; install version %s", e.Client.Name(), e.Client.Tested())
	}
	if err != nil {
		return err
	}
	if verr := CheckVersion(e.Client.Name(), cv, e.Client.Tested()); verr != nil {
		return verr
	}
	for _, name := range slices.Sorted(maps.Keys(e.Tiers)) {
		model, _ := TierEffort(e.Tiers[name], e.Client.Name())
		if rerr := e.Client.ResolveModel(model); rerr != nil {
			return fmt.Errorf("tier %s (%s = %q): %w; fix [tiers.%s] in .ness/ness.toml",
				name, e.Client.Name(), model, rerr, name)
		}
	}
	return CheckShadowing(e.ProjectSkills, e.SkillRoots)
}

// CheckVersion accepts found when it has tested's major and is at least tested by (minor,
// patch). A pre-release of the tested version is below it (semver); herdr and the clients release
// often, so a newer one is not a reason to refuse.
func CheckVersion(what, found, tested string) error {
	f, err := parseVersion(found)
	if err != nil {
		return fmt.Errorf("%s version %q is not a version; tested version is %s", what, found, tested)
	}
	t, err := parseVersion(tested)
	if err != nil {
		return err
	}
	if f.n[0] != t.n[0] || slices.Compare(f.n[1:], t.n[1:]) < 0 || (f.n == t.n && f.pre && !t.pre) {
		return fmt.Errorf("%s %s is not supported: ness was tested with %s %s; "+
			"install that version or newer in the same major",
			what, found, what, tested)
	}
	return nil
}

type version struct {
	n   [3]int
	pre bool
}

func parseVersion(v string) (version, error) {
	core, pre, hasPre := strings.Cut(strings.TrimPrefix(v, "v"), "-")
	parts := strings.Split(core, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return version{}, fmt.Errorf("version %q", v)
	}
	out := version{pre: hasPre && pre != ""}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, fmt.Errorf("version %q", v)
		}
		out.n[i] = n
	}
	return out, nil
}

// SkillRoots are the global skill directories a project skill can be shadowed from (§4, D14):
// Claude Code personal and enterprise (R-SESS-014), Codex personal and system.
func SkillRoots(home string) []string {
	return []string{
		filepath.Join(home, ".claude", "skills"), "/etc/claude-code/skills",
		filepath.Join(home, ".agents", "skills"), "/etc/codex/skills",
	}
}

// ErrShadowed marks an exact shadow match, as opposed to a directory that could not be read.
var ErrShadowed = errors.New("skill shadowed")

// Shadow is one global skill directory that has the name of a project skill directory.
type Shadow struct{ Global, Project string }

// Shadowed lists every directory in any root that has exactly the name of a directory in
// projectSkills; symlinks are followed on both sides, so linked skills count (R-SESS-005,
// R-SESS-006). Plugin skills are namespaced (name:skill) and cannot shadow; near names do not
// match (R-SESS-007).
func Shadowed(projectSkills string, roots []string) ([]Shadow, error) {
	local, err := os.ReadDir(projectSkills)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil // no project skills, nothing to shadow
	}
	if err != nil {
		return nil, err
	}
	var out []Shadow
	for _, root := range roots {
		for _, d := range local {
			if strings.Contains(d.Name(), ":") {
				continue
			}
			project := filepath.Join(projectSkills, d.Name())
			if !isDir(project) {
				continue
			}
			global := filepath.Join(root, d.Name())
			if isDir(global) && !sameDir(global, project) {
				out = append(out, Shadow{Global: global, Project: project})
			}
		}
	}
	return out, nil
}

// CheckShadowing refuses at the first shadow (R-SESS-005).
func CheckShadowing(projectSkills string, roots []string) error {
	found, err := Shadowed(projectSkills, roots)
	if err != nil || len(found) == 0 {
		return err
	}
	return fmt.Errorf("%w: global skill %s shadows project skill %s; rename or remove one of them",
		ErrShadowed, found[0].Global, found[0].Project)
}

// TierEffort splits the tier row for the client.
func TierEffort(t config.Tier, client string) (model, effort string) {
	if client == "codex" {
		return config.SplitTier(t.Codex)
	}
	return config.SplitTier(t.Claude)
}

// sameDir reports whether a and b are one directory once symlinks are resolved: a skill root that
// is the project's own skills directory (a project at $HOME) does not shadow itself.
func sameDir(a, b string) bool {
	ra, aerr := filepath.EvalSymlinks(a)
	rb, berr := filepath.EvalSymlinks(b)
	return aerr == nil && berr == nil && ra == rb
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
