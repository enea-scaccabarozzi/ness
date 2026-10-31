package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/store"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// project is a loaded ness project: the repository root and its parsed configuration. head is
// the part of the file any ness reads (R-UPD-046); file is nil only on locateProject's
// newer-pin refusal.
type project struct {
	root string
	head config.Head
	file *config.File
}

// pin is the project's version pin, read from the parsed file or, without one, from the head.
func (p *project) pin() string {
	if p.file != nil {
		return p.file.Pin()
	}
	return p.head.Ness
}

// repoRoot returns the top of the git checkout containing dir (a worktree is its own top).
func repoRoot(dir string) (string, error) {
	args := []string{"-C", dir, "rev-parse", "--show-toplevel"}
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed git subcommand
	cmd.Env = store.CleanGitEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		return "", refuse("%s is not inside a git repository; run ness from a project checkout", dir)
	}
	return strings.TrimSpace(string(out)), nil
}

// loadProject finds the repository, parses .ness/ness.toml and checks the version pin. Every
// failure is a refusal.
func loadProject(d Deps) (*project, error) {
	p, err := locateProject(d)
	if err != nil {
		return nil, err
	}
	if cerr := config.CheckPin(p.file.Pin(), d.Version); cerr != nil {
		return nil, pinRefusal(d, cerr)
	}
	return p, nil
}

// pinRefusal is the refusal for a CheckPin error: the cause, the binary and the fix (R-CFG-013).
func pinRefusal(d Deps, err error) error {
	var pe *config.PinError
	if errors.As(err, &pe) {
		return refuse("%w (binary: %s); fix: %s", pe, d.Executable, pe.Fix())
	}
	return refuse("%v (binary: %s)", err, d.Executable)
}

// locateProject finds the repository and parses .ness/ness.toml. Only a binary older than the
// pin is refused here, before the strict parse: that file may carry keys only the newer ness
// knows, and its fix is self-update, not a config edit (R-UPD-020). That refusal comes with a
// project holding root and head but no file, so doctor still reports the binary and the store.
// Any other pin mismatch is left to the caller, so doctor can report it too.
func locateProject(d Deps) (*project, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := repoRoot(wd)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, config.Path)) //nolint:gosec // fixed path under the repo
	if errors.Is(err, fs.ErrNotExist) {
		return nil, refuse("no %s in %s; run ness init first", config.Path, root)
	}
	if err != nil {
		return nil, err
	}
	head, herr := config.ReadHead(data)
	if herr == nil {
		var pe *config.PinError
		if errors.As(config.CheckPin(head.Ness, d.Version), &pe) && pe.Older {
			return &project{root: root, head: head}, pinRefusal(d, pe)
		}
	}
	f, err := config.Parse(data)
	if err != nil {
		return nil, refuse("%w", err)
	}
	return &project{root: root, head: head, file: f}, nil
}

// resolvedConfig is the project's configuration with the applied profiles' defaults
// (R-PROF-008) and the features they enable (R-PROF-015), and the resolved profiles.
func resolvedConfig(d Deps, p *project) (*config.Config, *profile.Resolved, error) {
	base, err := p.file.Config(nil)
	if err != nil {
		return nil, nil, refuse("%v", err)
	}
	res, err := loadProfiles(d, p, base.Profiles)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := p.file.Config(res.Defaults)
	if err != nil {
		return nil, nil, refuse("%v", err)
	}
	cfg.Enable(res.Features...)
	return cfg, res, nil
}

// projectConfig is resolvedConfig without the profiles.
func projectConfig(d Deps, p *project) (*config.Config, error) {
	cfg, _, err := resolvedConfig(d, p)
	return cfg, err
}

// primaryRoot is the primary checkout: the parent of the git common dir, the same from a
// worktree (R-SESS-020). store.Locate returns <common>/ness/state.sqlite3.
func primaryRoot(p *project) (string, error) {
	path, err := store.Locate(p.root)
	if err != nil {
		return "", refuse("%v", err)
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(path))), nil
}
