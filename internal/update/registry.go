package update

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// Registry is the machine registry: one absolute primary-checkout path per line of the file at
// Path, $XDG_CONFIG_HOME/ness/projects (R-UPD-031).
type Registry struct{ Path string }

var errNoPath = errors.New("no user config directory: the machine registry is off")

// Add records root once, drops listed paths that no longer hold .ness/ness.toml, and writes the
// file through a temp file renamed into place in the same directory.
// ponytail: no lock; a lost concurrent append is healed by the next sync.
func (r Registry) Add(root string) error {
	paths, err := r.List()
	if err != nil {
		return err
	}
	if !slices.Contains(paths, root) {
		paths = append(paths, root)
	}
	var b strings.Builder
	for _, p := range paths {
		b.WriteString(p + "\n")
	}
	return writeAtomic(r.Path, []byte(b.String()))
}

// List returns the listed paths whose .ness/ness.toml is not missing (an unreadable one stays),
// in file order; a missing file is an empty list. It never writes.
func (r Registry) List() ([]string, error) {
	if r.Path == "" {
		return nil, errNoPath
	}
	data, err := os.ReadFile(r.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var live []string
	for _, p := range strings.Split(string(data), "\n") {
		if p == "" || slices.Contains(live, p) {
			continue
		}
		// Only a missing file is stale: any other stat error keeps the project, so the install veto
		// fails closed on it.
		_, err := os.Stat(filepath.Join(p, config.Path)) //nolint:gosec // our own registry
		if !errors.Is(err, fs.ErrNotExist) {
			live = append(live, p)
		}
	}
	return live, nil
}
