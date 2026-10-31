// Package worker is the per-project worker (design §5 "Machinery", §14): the pass that wakes
// idle sessions, compacts them at their boundary, keeps limited panes silent and resumes them,
// reports stalls, stops a session whose model went away, retires zombie items and writes the
// heartbeat; the loop that runs it every 5 s and restarts on a binary change; the lock; the log.
package worker

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// LogMax is the size at which worker.log rotates (R-WRK-026).
	LogMax = 10 << 20
	// LogKeep is how many rotated files stay: worker.log.1 and .2.
	LogKeep = 2
)

// Log is worker.log (R-WRK-021): one line per action, rotated at LogMax keeping LogKeep files
// (R-WRK-026), an identical error per key written once until the key clears (R-WRK-022). A
// write failure is reported once on Err and never stops the caller.
type Log struct {
	Path string
	Max  int64 // 0 means LogMax
	Now  func() time.Time
	Err  io.Writer // nil discards

	mu     sync.Mutex // floors log from their own goroutines
	last   map[string]string
	failed bool
}

// Action writes one line: stamp, action, subject (a session or item id, "-" for the process), detail.
func (l *Log) Action(action, subject, detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write(action + " " + subject + " " + detail)
}

// Error writes the error for key unless it is the one last written for key.
func (l *Log) Error(key string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	text := err.Error()
	if l.last == nil {
		l.last = map[string]string{}
	}
	if l.last[key] == text {
		return
	}
	l.last[key] = text
	l.write("error " + key + " " + text)
}

// Clear forgets key's last error: the next identical one is written again.
func (l *Log) Clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.last, key)
}

func (l *Log) write(line string) {
	err := l.rotate()
	if err == nil {
		var f *os.File
		f, err = os.OpenFile(l.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the project's own log
		if err == nil {
			// One line per action: a multi-line exec error is folded onto its line.
			line = strings.NewReplacer("\r\n", " | ", "\n", " | ", "\r", " | ").Replace(line)
			_, err = fmt.Fprintf(f, "%s %s\n", l.Now().UTC().Format(time.RFC3339), line)
			err = errors.Join(err, f.Close())
		}
	}
	if err != nil && !l.failed && l.Err != nil {
		l.failed = true
		//nolint:errcheck // nowhere left to report
		_, _ = fmt.Fprintf(l.Err, "ness: %s: %v (further write errors are not reported)\n", l.Path, err)
	}
}

// rotate shifts worker.log to .1 and .1 to .2 when it reached Max; a missing log is nothing to rotate.
func (l *Log) rotate() error {
	limit := l.Max
	if limit == 0 {
		limit = LogMax
	}
	fi, err := os.Stat(l.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(filepath.Dir(l.Path), 0o750)
	}
	if err != nil || fi.Size() < limit {
		return err
	}
	for i := LogKeep; i > 1; i-- {
		err = os.Rename(rotated(l.Path, i-1), rotated(l.Path, i))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return os.Rename(l.Path, rotated(l.Path, 1))
}

func rotated(path string, n int) string { return fmt.Sprintf("%s.%d", path, n) }
