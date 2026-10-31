package cli_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestConcurrentResumeOpensOneTab(t *testing.T) {
	// A second --resume of a session being resumed is refused: it must not open a second tab.
	// covers: R-SESS-045, R-SESS-047
	tmp := t.TempDir()
	repo, fakes := filepath.Join(tmp, "repo"), filepath.Join(tmp, "fakes")
	for k, v := range map[string]string{
		"HOME": tmp, "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.invalid",
		"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.invalid",
	} {
		t.Setenv(k, v)
	}
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	t.Chdir(repo)
	if err := testkit.WriteFakeDefaults(fakes); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, string) {
		var out, errb bytes.Buffer
		d := cli.Deps{
			Stdout: &out, Stderr: &errb, Version: "0.1.0", Now: time.Now, GOOS: "linux",
			Exec: testkit.FakeExec{Dir: fakes}.Run,
		}
		return cli.Main(args, d), errb.String()
	}
	timeout := func(v string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repo, ".ness", "ness.toml"))
		if err != nil {
			t.Fatal(err)
		}
		base, _, _ := strings.Cut(string(data), "[launch]")
		err = os.WriteFile(filepath.Join(repo, ".ness", "ness.toml"),
			[]byte(base+"[launch]\nready_timeout = \""+v+"\"\n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}
	if code, msg := run("init", "--gate", "true", "--base", "main", "--yes"); code != 0 {
		t.Fatalf("init: %d %s", code, msg)
	}
	timeout("200ms")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "ness init")
	if err := testkit.SetHerdr(fakes, "ready_on_start", "false"); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(tmp, "brief.md")
	if err := os.WriteFile(brief, []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, msg := run("open", "--root", "--scope", "prog", "--tier", "standard", "--brief", brief); code != cli.ExitRefused {
		t.Fatalf("stalled open: %d %s", code, msg)
	}
	s := openStore(t, repo)
	id, agent := stalledSession(t, s, fakes)
	if err := testkit.SetHerdr(fakes, "exit", agent); err != nil {
		t.Fatal(err)
	}
	timeout("3s")
	done := make(chan int, 1)
	go func() { code, _ := run("open", "--resume", id); done <- code }()
	waitFor(t, func() bool { // the first resume restarted the agent and is waiting for the mark
		st, err := testkit.HerdrState(fakes)
		return err == nil && len(st.Agents) == 2 && sessionPhase(t, s, id) == "waiting-ready"
	})
	code, msg := run("open", "--resume", id)
	if code != cli.ExitRefused || !strings.Contains(msg, "already being resumed") {
		t.Fatalf("second resume: exit %d, stderr %q", code, msg)
	}
	if st, err := testkit.HerdrState(fakes); err != nil || len(st.Tabs) != 1 || len(st.Agents) != 2 {
		t.Fatalf("after the second resume: %d tabs, %d agents, %v", len(st.Tabs), len(st.Agents), err)
	}
	if code = <-done; code != cli.ExitRefused { // the mark never comes: the first stalls
		t.Fatalf("first resume exit %d", code)
	}
}

func stalledSession(t *testing.T, s *store.Store, fakes string) (id, agent string) {
	t.Helper()
	all, err := s.Sessions(true)
	if err != nil || len(all) != 1 {
		t.Fatalf("sessions = %v, %v", all, err)
	}
	st, err := testkit.HerdrState(fakes)
	if err != nil || len(st.Agents) != 1 {
		t.Fatalf("agents = %v, %v", st.Agents, err)
	}
	return all[0].ID, st.Agents[0].Name
}

func sessionPhase(t *testing.T, s *store.Store, id string) string {
	t.Helper()
	sess, err := s.Session(id)
	if err != nil {
		t.Fatal(err)
	}
	return sess.Phase
}

// openStore opens the project's store for the rest of the test.
func openStore(t *testing.T, repo string) *store.Store {
	t.Helper()
	path, err := store.Locate(repo)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for !ok() {
		select {
		case <-deadline:
			t.Fatal("condition not reached")
		case <-time.After(5 * time.Millisecond):
		}
	}
}
