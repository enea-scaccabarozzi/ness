package update

import (
	"path/filepath"
	"strings"
)

// ManagedBy reports whether binary sits where mise or go install owns it (R-UPD-045), and the
// command that installs version there instead: under $MISE_DATA_DIR/installs (default
// ~/.local/share/mise/installs), $GOBIN, each $GOPATH entry's bin, or ~/go/bin. binary and home
// are resolved paths; the directories are resolved here.
func ManagedBy(binary, version string, getenv func(string) string, home string) (command string, ok bool) {
	version = strings.TrimPrefix(version, "v")
	mise := getenv("MISE_DATA_DIR")
	if mise == "" {
		mise = filepath.Join(home, ".local", "share", "mise")
	}
	if within(filepath.Join(mise, "installs"), binary) {
		return "mise use -g github:enea-scaccabarozzi/ness@" + version, true
	}
	dirs := []string{getenv("GOBIN"), filepath.Join(home, "go", "bin")}
	for _, p := range filepath.SplitList(getenv("GOPATH")) {
		dirs = append(dirs, filepath.Join(p, "bin"))
	}
	for _, d := range dirs {
		if d != "" && within(d, binary) {
			return goInstall + version, true
		}
	}
	return "", false
}

// within reports whether path lies inside dir. dir is resolved through symlinks first, as
// callers resolve path: a $GOBIN or $MISE_DATA_DIR that is a symlink still matches.
func within(dir, path string) bool {
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
