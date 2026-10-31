package session_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

const keepalive = "core.sshCommand=ssh -o ServerAliveInterval=30 -o ServerAliveCountMax=10"

// failingPush runs real git, except that its first git push fails with fail (when set) without
// pushing. It records the argv of every push.
type failingPush struct {
	mu     sync.Mutex
	fail   string
	pushes [][]string
}

func (f *failingPush) Exec(dir, name string, args ...string) ([]byte, error) {
	if name == "git" && slices.Contains(args, "push") {
		f.mu.Lock()
		f.pushes = append(f.pushes, args)
		first := len(f.pushes) == 1
		f.mu.Unlock()
		if first && f.fail != "" {
			return nil, errors.New("git " + strings.Join(args, " ") + ": exit status 128: " + f.fail)
		}
	}
	return cli.SystemExec(dir, name, args...)
}

// pushRun integrates session/s-1 into main with Push, origin a bare clone, through f.
func pushRun(t *testing.T, f *failingPush, sshCommand string) (g session.Git, repo, remote string, log *strings.Builder, err error) {
	t.Helper()
	g, repo, common := setupRepo(t)
	remote = t.TempDir()
	steps := [][]string{{remote, "init", "-q", "--bare", "-b", "main"}, {repo, "remote", "add", "origin", remote},
		{repo, "push", "-q", "origin", "main"}}
	if sshCommand != "" {
		steps = append(steps, []string{repo, "config", "core.sshCommand", sshCommand})
	}
	for _, s := range steps {
		if _, serr := g.Out(s[0], s[1:]...); serr != nil {
			t.Fatal(serr)
		}
	}
	log = &strings.Builder{}
	_, err = session.Integrate(session.Git{Exec: f.Exec}, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1", Log: log, Push: true, Gate: "true",
		Integration: session.Integration{Source: "session/s-1", Target: "main"},
	})
	return g, repo, remote, log, err
}

// hasPair reports whether argv holds "-c" then v before "push".
func hasPair(argv []string, v string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "push" {
			return false
		}
		if argv[i] == "-c" && argv[i+1] == v {
			return true
		}
	}
	return false
}

// Rejects an implementation that gives up on a dropped connection, retries more than once, or
// pushes without the ssh keepalive.
func TestIntegrateRetriesAPushThatLostItsConnection(t *testing.T) {
	// covers: R-SESS-166
	f := &failingPush{fail: "Connection to github.com closed by remote host.\nfatal: the remote end hung up unexpectedly"}
	g, repo, remote, log, err := pushRun(t, f, "")
	if err != nil {
		t.Fatalf("push after a dropped connection: %v\n%s", err, log.String())
	}
	local, _ := g.Rev(repo, "main")
	if got, _ := g.Rev(remote, "main"); got != local {
		t.Fatalf("origin main %s, local main %s", got, local)
	}
	if len(f.pushes) != 2 {
		t.Fatalf("%d push attempts, want 2: %q", len(f.pushes), f.pushes)
	}
	for _, argv := range f.pushes {
		if !hasPair(argv, keepalive) {
			t.Fatalf("push without the keepalive -c pair: %q", argv)
		}
	}
	if want := "ness: the push lost its connection; retrying once"; !strings.Contains(log.String(), want) {
		t.Fatalf("log lacks %q:\n%s", want, log.String())
	}
}

// Rejects an implementation that retries every push failure: a rejection is not a dropped link.
func TestIntegrateDoesNotRetryARejectedPush(t *testing.T) {
	// covers: R-SESS-166
	f := &failingPush{fail: "! [rejected]        main -> main (fetch first)"}
	_, _, _, log, err := pushRun(t, f, "")
	if err == nil {
		t.Fatal("a rejected push reported success")
	}
	if len(f.pushes) != 1 {
		t.Fatalf("%d push attempts for a rejection, want 1", len(f.pushes))
	}
	if strings.Contains(log.String(), "retrying") {
		t.Fatalf("log says it retried a rejection:\n%s", log.String())
	}
}

// Rejects an implementation that overrides the project's own core.sshCommand.
func TestIntegrateKeepsTheProjectsSSHCommand(t *testing.T) {
	// covers: R-SESS-166
	f := &failingPush{}
	_, _, _, log, err := pushRun(t, f, "mine")
	if err != nil {
		t.Fatalf("push: %v\n%s", err, log.String())
	}
	if len(f.pushes) != 1 {
		t.Fatalf("%d push attempts, want 1", len(f.pushes))
	}
	for _, a := range f.pushes[0] {
		if strings.HasPrefix(a, "core.sshCommand=") {
			t.Fatalf("push overrides the project's core.sshCommand: %q", f.pushes[0])
		}
	}
}
