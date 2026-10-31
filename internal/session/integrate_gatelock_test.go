package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// signalWriter closes seen the first time a write contains want.
type signalWriter struct {
	mu   sync.Mutex
	want string
	seen chan struct{}
	done bool
}

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.done && strings.Contains(string(p), w.want) {
		w.done = true
		close(w.seen)
	}
	return len(p), nil
}

func TestIntegrateWaitsForAnotherGateInsteadOfFailing(t *testing.T) {
	// covers: R-SESS-145 — rejects an implementation that fails (or runs the gate) while the gate lock is held.
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	before, _ := g.Rev(repo, "main")
	lockDir := t.TempDir()
	f, err := os.Create(filepath.Join(lockDir, "gate.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()                                                    //nolint:errcheck // test cleanup
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil { //nolint:gosec // fd
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "ran")
	log := &signalWriter{want: "another gate is running in this project; waiting for it", seen: make(chan struct{})}
	type outcome struct {
		res session.Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
			Repo: repo, Common: common, LockDir: lockDir, TmpName: "tmp-s-1", Log: log,
			Gate: "touch " + marker, Integration: session.Integration{Source: "session/s-1", Target: "main"},
		})
		done <- outcome{res, err}
	}()
	<-log.seen
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatal("the gate ran while another gate held the lock")
	}
	if now, _ := g.Rev(repo, "main"); now != before {
		t.Fatal("main moved while waiting for the gate lock")
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	out := <-done
	if out.err != nil {
		t.Fatalf("integrate after the lock was released: %v", out.err)
	}
	if _, serr := os.Stat(marker); serr != nil {
		t.Fatal("the gate never ran after the lock was released")
	}
	if now, _ := g.Rev(repo, "main"); now == before || out.res.Ref != now {
		t.Fatalf("main %s → %s, recorded %s", before, now, out.res.Ref)
	}
}
