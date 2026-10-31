package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

// Install replaces binary with data (R-UPD-043): it writes a temp file in binary's directory with
// mode 0755, runs probe on it (its output's last field must be version want), keeps the old
// binary as <binary>.prev and renames the temp file into place. On any failure the old binary is
// untouched and the temp file is removed. It returns the .prev path.
func Install(binary string, data []byte, want string, probe func(path string) (string, error)) (string, error) {
	tmp, err := writeExec(filepath.Dir(binary), data)
	if err != nil {
		return "", err
	}
	prev := binary + ".prev"
	if err := checkVersion(probe, tmp, want); err != nil {
		return "", errors.Join(err, os.Remove(tmp))
	}
	if err := keepPrev(binary, prev); err != nil {
		return "", errors.Join(err, os.Remove(tmp))
	}
	if err := os.Rename(tmp, binary); err != nil {
		return "", errors.Join(err, os.Remove(tmp))
	}
	return prev, nil
}

// writeExec writes data to a new temp file with mode 0755 in dir and returns its path; on failure
// nothing is left.
func writeExec(dir string, data []byte) (string, error) {
	tmp, err := os.CreateTemp(dir, ".ness-new-*")
	if err != nil {
		return "", err
	}
	_, err = tmp.Write(data)
	err = errors.Join(err, tmp.Close())
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o755) //nolint:gosec // an executable
	}
	if err != nil {
		return "", errors.Join(err, os.Remove(tmp.Name()))
	}
	return tmp.Name(), nil
}

// checkVersion runs probe on path and wants the last field of its output to be version want,
// build metadata ignored.
func checkVersion(probe func(string) (string, error), path, want string) error {
	out, err := probe(path)
	if err != nil {
		return fmt.Errorf("the new binary does not run: %w", err)
	}
	f := strings.Fields(out)
	if len(f) == 0 {
		return fmt.Errorf("the new binary printed no version, want %s", want)
	}
	if c, err := nsync.Compare(f[len(f)-1], want); err != nil || c != 0 {
		return fmt.Errorf("the new binary reports %q, want %s", out, want)
	}
	return nil
}

// keepPrev makes prev a hard link to binary, or a copy where links fail, replacing any older one.
func keepPrev(binary, prev string) error {
	if err := os.Remove(prev); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if os.Link(binary, prev) == nil {
		return nil
	}
	data, err := os.ReadFile(binary) //nolint:gosec // the running binary
	if err != nil {
		return err
	}
	return os.WriteFile(prev, data, 0o755) //nolint:gosec // an executable
}
