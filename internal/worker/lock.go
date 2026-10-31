package worker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrLocked is returned by Lock when another worker of the project holds the lock (R-WRK-001).
var ErrLocked = errors.New("another worker runs for this project")

// Lock takes <dir>/worker.lock without waiting and writes this process's pid into it; the second
// worker gets ErrLocked naming the first's pid. The lock lives as long as the returned release (or
// the process). The file is never removed: unlinking a lock file races with a second opener.
func Lock(dir string) (release func(), err error) {
	path := filepath.Join(dir, "worker.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // beside the store
	if err != nil {
		return nil, err
	}
	//nolint:gosec // an fd fits an int
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		pid, _ := io.ReadAll(f) //nolint:errcheck // the pid is a courtesy in the message
		return nil, errors.Join(fmt.Errorf("%w (pid %s)", ErrLocked, strings.TrimSpace(string(pid))), f.Close())
	}
	if err := f.Truncate(0); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	if _, err := f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return func() { _ = f.Close() }, nil //nolint:errcheck // closing drops the lock
}
