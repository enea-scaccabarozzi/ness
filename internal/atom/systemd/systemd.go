// Package systemd is the supervisor atom (design §2): the worker as a systemd user unit at
// ~/.config/systemd/user/ness-<project>.service, the only file ness writes outside the
// repository (R-WRK-002, §6). launchd is a later atom behind the same shape.
package systemd

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Exec runs a program; cli.SystemExec in production, a fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Supervisor installs and removes the unit through systemctl --user.
type Supervisor struct {
	Exec Exec
	Home string
}

// UnitName is the unit's name without the suffix: ness-<project>.
func UnitName(project string) string { return "ness-" + project }

// UnitPath is where the unit lives: what ness doctor parses (R-DOC-005).
func (s Supervisor) UnitPath(project string) string {
	return filepath.Join(s.Home, ".config", "systemd", "user", UnitName(project)+".service")
}

// ErrForeign is returned when the unit at the path belongs to another project, or to nobody.
var ErrForeign = errors.New("unit is not this project's")

const markerPrefix = "# ness-unit project="

// quote makes one systemd word: double quotes, backslash and quote escaped, % as %% (specifiers);
// in ExecStart also $ as $$ so nothing expands.
func quote(s string, exec bool) string {
	pairs := []string{`\`, `\\`, `"`, `\"`, "%", "%%"}
	if exec {
		pairs = append(pairs, "$", "$$")
	}
	return `"` + strings.NewReplacer(pairs...).Replace(s) + `"`
}

// Unit renders the unit: the binary by its absolute path, never through PATH (R-WRK-003, §16 #3);
// the installer's PATH, because a user manager's own PATH lacks ~/.local/bin where herdr lives;
// restarted on every exit, so the exit 0 of a version change brings the new binary up (R-WRK-009).
// The first line marks the project root that owns the unit, so another project of the same name
// never overwrites or removes it.
func Unit(project, binary, workdir, path string) string {
	env := ""
	if path != "" {
		env = "Environment=" + quote("PATH="+path, false) + "\n"
	}
	return fmt.Sprintf(`%s%s
[Unit]
Description=ness worker for %s

[Service]
ExecStart=%s worker run
%sWorkingDirectory=%s
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, markerPrefix, workdir, project, quote(binary, true), env, strings.ReplaceAll(workdir, "%", "%%"))
}

// FirstWord is the program of an ExecStart= value, undoing exactly what Unit's quoting did; an
// unquoted word (a hand-written unit) is read up to the first blank.
func FirstWord(execStart string) string {
	v := strings.TrimSpace(execStart)
	var word string
	if rest, quoted := strings.CutPrefix(v, `"`); quoted {
		var b strings.Builder
		for i := 0; i < len(rest) && rest[i] != '"'; i++ {
			if rest[i] == '\\' && i+1 < len(rest) {
				i++
			}
			b.WriteByte(rest[i])
		}
		word = b.String()
	} else if f := strings.Fields(v); len(f) > 0 {
		word = f[0]
	}
	return strings.NewReplacer("%%", "%", "$$", "$").Replace(word)
}

// owner checks the unit file at path: its content, and nil when absent or owned by root.
func owner(path, root string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the unit of this project
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if other, ok := strings.CutPrefix(line, markerPrefix); ok {
			if other == root {
				return data, nil
			}
			return nil, fmt.Errorf("%w: %s belongs to the project at %s", ErrForeign, path, other)
		}
	}
	return nil, fmt.Errorf("%w: %s has no ness marker", ErrForeign, path)
}

// Available reports whether a systemd user session answers (R-WRK-004): running, or degraded,
// which exits 1 but still runs units.
func (s Supervisor) Available() error {
	out, err := s.Exec("", "systemctl", "--user", "is-system-running")
	state := strings.TrimSpace(string(out))
	if err != nil && state != "degraded" {
		msg := strings.Join(strings.Fields(err.Error()), " ")
		return errors.New("no systemd user session: " + msg + " (without a login, loginctl enable-linger $USER)")
	}
	return nil
}

// Install writes the unit atomically and loads it (R-WRK-002). The caller checks Available
// first. A unit of another project is refused (ErrForeign); an identical one is left alone; a
// changed one is reloaded and restarted so its new ExecStart, PATH and directory take effect.
func (s Supervisor) Install(project, binary, workdir, path string) (string, error) {
	unit := s.UnitPath(project)
	old, err := owner(unit, workdir)
	if err != nil {
		return "", err
	}
	data := []byte(Unit(project, binary, workdir, path))
	if bytes.Equal(old, data) {
		return unit, nil
	}
	if err = os.MkdirAll(filepath.Dir(unit), 0o750); err != nil { //nolint:gocritic // err is declared above
		return "", err
	}
	if err = writeAtomic(unit, data); err != nil { //nolint:gocritic // as above
		return "", err
	}
	steps := [][]string{{"daemon-reload"}, {"enable", "--now", UnitName(project)}}
	if old != nil {
		steps = append(steps, []string{"restart", UnitName(project)})
	}
	for _, args := range steps {
		if _, err = s.Exec("", "systemctl", append([]string{"--user"}, args...)...); err != nil {
			return "", err
		}
	}
	return unit, nil
}

func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".ness-unit-*")
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	err = errors.Join(werr, f.Close())
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		err = errors.Join(err, os.Remove(f.Name()))
	}
	return err
}

// Uninstall stops and disables the unit and removes its file (R-WRK-006). A unit of another
// project is refused (ErrForeign); a missing file is nothing to do; a failed disable is reported.
func (s Supervisor) Uninstall(project, root string) error {
	unit := s.UnitPath(project)
	data, err := owner(unit, root)
	if err != nil || data == nil {
		return err
	}
	if _, err = s.Exec("", "systemctl", "--user", "disable", "--now", UnitName(project)); err != nil {
		return err
	}
	if err = os.Remove(unit); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	_, err = s.Exec("", "systemctl", "--user", "daemon-reload")
	return err
}
