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

// project is a loaded ness project: the repository root and its parsed configuration.
type project struct {
	root string
	file *config.File
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
	p, err := locateProject()
	if err != nil {
		return nil, err
	}
	if cerr := config.CheckPin(p.file.Pin(), d.Version); cerr != nil {
		return nil, refuse("%v (binary: %s)", cerr, executable())
	}
	return p, nil
}

// locateProject finds the repository and parses .ness/ness.toml without checking the pin, so
// doctor can still report a binary that does not match it.
func locateProject() (*project, error) {
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
	f, err := config.Parse(data)
	if err != nil {
		return nil, refuse("%v", err)
	}
	return &project{root: root, file: f}, nil
}

func executable() string {
	p, err := os.Executable()
	if err != nil {
		return "unknown path"
	}
	return trimDeleted(p)
}

// trimDeleted drops the " (deleted)" Linux appends to /proc/self/exe once the running binary was
// replaced on disk, so an upgraded binary is found at its path (R-WRK-009).
func trimDeleted(p string) string { return strings.TrimSuffix(p, " (deleted)") }

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
