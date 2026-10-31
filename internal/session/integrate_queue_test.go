package session_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// fifo makes a named pipe in dir.
func fifo(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// logBuf is a log several goroutines write to.
type logBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// blockedOn reports whether /proc/locks shows a process waiting for a lock on the file at path.
func blockedOn(t *testing.T, path string) bool {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("no inode for the lock file")
	}
	b, err := os.ReadFile("/proc/locks")
	if err != nil {
		t.Skipf("no /proc/locks: %v", err)
	}
	for line := range strings.Lines(string(b)) {
		if strings.Contains(line, " -> ") && strings.Contains(line, ":"+strconv.FormatUint(st.Ino, 10)+" ") {
			return true
		}
	}
	return false
}

// Rejects an implementation that still refuses a held target lock (ErrBusy), one that never writes
// its Holder into the lock (the waiter then cannot name s-first), and one that logs the wait but
// proceeds: the first is released only once the kernel shows the second blocked on the target
// lock, and a second that went past it reaches the gate lock (logged) or a gate needing a.txt,
// which only the first lands.
func TestIntegrateQueuesBehindTheTargetLockAndNamesTheHolder(t *testing.T) {
	// covers: R-SESS-164
	g, repo, common := setupRepo(t)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-2"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	pipes, lockDir := t.TempDir(), t.TempDir()
	enter, leave := fifo(t, pipes, "enter"), fifo(t, pipes, "leave")
	run := func(id, holder, gate string, log *logBuf) chan error {
		done := make(chan error, 1)
		go func() {
			_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
				Repo: repo, Common: common, LockDir: lockDir, TmpName: "tmp-" + id, Log: log, Holder: holder, Gate: gate,
				Integration: session.Integration{Source: "session/" + id, Target: "main"},
			})
			done <- err
		}()
		return done
	}
	first := run("s-1", "s-first", "echo in > "+enter+" && cat "+leave+" >/dev/null", &logBuf{})
	entered := make(chan error, 1)
	go func() { _, err := os.ReadFile(enter); entered <- err }() //nolint:gosec // test fifo
	select {
	case err := <-entered:
		if err != nil {
			t.Fatal(err)
		}
	case err := <-first:
		t.Fatalf("the first integrate ended before its gate held the target: %v", err)
	}
	log := &logBuf{}
	second := run("s-2", "s-second", "test -f a.txt", log)
	for !blockedOn(t, filepath.Join(lockDir, "integrate-main.lock")) {
		select {
		case err := <-second:
			t.Fatalf("the second integrate returned instead of waiting for the holder: %v\n%s", err, log)
		default:
		}
		if strings.Contains(log.String(), "another gate is running") {
			t.Fatalf("the second integrate went past the held target lock:\n%s", log)
		}
		runtime.Gosched()
	}
	if want := "ness: waiting for s-first's integrate into main\n"; !strings.Contains(log.String(), want) {
		t.Fatalf("log lacks %q:\n%s", want, log)
	}
	if err := os.WriteFile(leave, []byte("go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-first; err != nil {
		t.Fatalf("first integrate: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("second integrate after the holder let go: %v", err)
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		if _, err := g.Out(repo, "cat-file", "-e", "main:"+f); err != nil {
			t.Fatalf("main lacks %s: %v", f, err)
		}
	}
}
