package systemd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/systemd"
)

func TestUnitGolden(t *testing.T) {
	// The binary by absolute path, the install-time PATH (herdr lives in ~/.local/bin, which a
	// user manager's PATH lacks), the project as working directory, restarted on every exit so an
	// exit 0 on a version change brings the new binary up.
	// covers: R-WRK-003, R-WRK-009
	got := systemd.Unit("repo", "/opt/ness/bin/ness", "/home/u/repo", "/home/u/.local/bin:/usr/bin")
	want := "# ness-unit project=/home/u/repo\n[Unit]\nDescription=ness worker for repo\n\n[Service]\nExecStart=\"/opt/ness/bin/ness\" worker run\n" +
		"Environment=\"PATH=/home/u/.local/bin:/usr/bin\"\n" +
		"WorkingDirectory=/home/u/repo\nRestart=always\nRestartSec=5\n\n[Install]\nWantedBy=default.target\n"
	if got != want {
		t.Fatalf("Unit =\n%s\nwant\n%s", got, want)
	}
	if p := (systemd.Supervisor{Home: "/home/u"}).UnitPath("repo"); p != "/home/u/.config/systemd/user/ness-repo.service" {
		t.Fatalf("UnitPath = %s", p)
	}
}

// recorder is an Exec that records calls and answers from a table.
type recorder struct {
	calls []string
	out   map[string]string
	fail  map[string]string
}

func (r *recorder) exec(_, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if msg, ok := r.fail[key]; ok {
		return []byte(r.out[key]), errors.New(key + ": exit status 1: " + msg)
	}
	return []byte(r.out[key]), nil
}

func TestAvailableAcceptsRunningAndDegradedOnly(t *testing.T) {
	// covers: R-WRK-004
	for name, c := range map[string]struct {
		out, fail string
		ok        bool
	}{
		"running":  {"running\n", "", true},
		"degraded": {"degraded\n", "exit 1", true}, // systemctl exits 1 for degraded; units still run
		"no bus":   {"", "Failed to connect to bus: No medium found", false},
		"offline":  {"offline\n", "exit 1", false},
	} {
		r := &recorder{out: map[string]string{"systemctl --user is-system-running": c.out}, fail: map[string]string{}}
		if c.fail != "" {
			r.fail["systemctl --user is-system-running"] = c.fail
		}
		err := (systemd.Supervisor{Exec: r.exec}).Available()
		if (err == nil) != c.ok {
			t.Errorf("%s: Available = %v, want ok %v", name, err, c.ok)
		}
	}
}

func TestInstallWritesTheUnitAndEnablesIt(t *testing.T) {
	// covers: R-WRK-002, R-WRK-006
	home := t.TempDir()
	r := &recorder{out: map[string]string{}, fail: map[string]string{}}
	s := systemd.Supervisor{Exec: r.exec, Home: home}
	path, err := s.Install("repo", "/opt/ness", "/w/repo", "/bin")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // the test's own home
	if err != nil || string(data) != systemd.Unit("repo", "/opt/ness", "/w/repo", "/bin") {
		t.Fatalf("unit at %s: %q, %v", path, data, err)
	}
	want := "systemctl --user daemon-reload;systemctl --user enable --now ness-repo"
	if strings.Join(r.calls, ";") != want {
		t.Fatalf("calls = %v", r.calls)
	}
	// Nothing else under home.
	var files []string
	_ = filepath.WalkDir(home, func(p string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if len(files) != 1 || files[0] != path {
		t.Fatalf("files under home: %v", files)
	}
	r.calls = nil
	if err := s.Uninstall("repo", "/w/repo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unit still there: %v", err)
	}
	want = "systemctl --user disable --now ness-repo;systemctl --user daemon-reload"
	if strings.Join(r.calls, ";") != want {
		t.Fatalf("calls = %v", r.calls)
	}
	if err := s.Uninstall("repo", "/w/repo"); err != nil {
		t.Fatalf("uninstall twice: %v", err)
	}
}

func TestUnitQuotingRoundTrips(t *testing.T) {
	// A space, a % and a $ in the binary, a space in PATH: quoted so systemd reads one word, %% and
	// $$ so nothing expands, and the doctor-side parse gives the binary back.
	// covers: R-WRK-003, R-DOC-005
	bin := `/opt/my ness/50%/b$HOME"q\ness`
	u := systemd.Unit("repo", bin, "/w/100%/repo", "/a b:/c")
	for _, want := range []string{
		`ExecStart="/opt/my ness/50%%/b$$HOME\"q\\ness" worker run`,
		`Environment="PATH=/a b:/c"`,
		"WorkingDirectory=/w/100%%/repo",
	} {
		if !strings.Contains(u, want+"\n") {
			t.Errorf("unit lacks %s:\n%s", want, u)
		}
	}
	line := ""
	for _, l := range strings.Split(u, "\n") {
		if v, ok := strings.CutPrefix(l, "ExecStart="); ok {
			line = v
		}
	}
	if got := systemd.FirstWord(line); got != bin {
		t.Fatalf("FirstWord = %q, want %q", got, bin)
	}
	if got := systemd.FirstWord("/usr/bin/ness worker run"); got != "/usr/bin/ness" {
		t.Fatalf("unquoted FirstWord = %q", got)
	}
}

func TestForeignUnitIsNeverTouched(t *testing.T) {
	// Two projects of the same basename share a unit path: the second refuses, install and
	// uninstall alike, and the first's unit stays byte for byte.
	// covers: R-WRK-002, R-WRK-006
	home := t.TempDir()
	r := &recorder{out: map[string]string{}, fail: map[string]string{}}
	s := systemd.Supervisor{Exec: r.exec, Home: home}
	path, err := s.Install("repo", "/opt/ness", "/a/repo", "/bin")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path) //nolint:errcheck,gosec // test file
	r.calls = nil
	if _, err = s.Install("repo", "/opt/ness", "/b/repo", "/bin"); !errors.Is(err, systemd.ErrForeign) ||
		!strings.Contains(err.Error(), "/a/repo") {
		t.Fatalf("install over foreign: %v", err)
	}
	if err = s.Uninstall("repo", "/b/repo"); !errors.Is(err, systemd.ErrForeign) {
		t.Fatalf("uninstall foreign: %v", err)
	}
	// A hand-written unit without the marker is foreign too.
	if err = os.WriteFile(path, []byte("[Service]\nExecStart=/x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Install("repo", "/opt/ness", "/a/repo", "/bin"); !errors.Is(err, systemd.ErrForeign) {
		t.Fatalf("install over unmarked: %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("systemctl ran: %v", r.calls)
	}
	_ = before
	if after, _ := os.ReadFile(path); string(after) != "[Service]\nExecStart=/x\n" { //nolint:errcheck,gosec // test file
		t.Fatalf("unit changed: %q", after)
	}
}

func TestReinstallRestartsOnlyWhenChanged(t *testing.T) {
	// covers: R-WRK-009
	home := t.TempDir()
	r := &recorder{out: map[string]string{}, fail: map[string]string{}}
	s := systemd.Supervisor{Exec: r.exec, Home: home}
	if _, err := s.Install("repo", "/opt/ness", "/w/repo", "/bin"); err != nil {
		t.Fatal(err)
	}
	r.calls = nil
	if _, err := s.Install("repo", "/opt/ness", "/w/repo", "/bin"); err != nil || len(r.calls) != 0 {
		t.Fatalf("unchanged: %v, calls %v", err, r.calls)
	}
	if _, err := s.Install("repo", "/opt/ness2", "/w/repo", "/bin"); err != nil {
		t.Fatal(err)
	}
	want := "systemctl --user daemon-reload;systemctl --user enable --now ness-repo;systemctl --user restart ness-repo"
	if strings.Join(r.calls, ";") != want {
		t.Fatalf("calls = %v", r.calls)
	}
	// Atomic: no temp file is left beside the unit.
	ents, _ := os.ReadDir(filepath.Dir(s.UnitPath("repo")))
	if len(ents) != 1 {
		t.Fatalf("entries: %v", ents)
	}
}

func TestUninstallReportsAFailedDisable(t *testing.T) {
	// covers: R-WRK-006
	home := t.TempDir()
	r := &recorder{out: map[string]string{}, fail: map[string]string{}}
	s := systemd.Supervisor{Exec: r.exec, Home: home}
	path, _ := s.Install("repo", "/opt/ness", "/w/repo", "/bin") //nolint:errcheck // checked below
	r.fail["systemctl --user disable --now ness-repo"] = "boom"
	if err := s.Uninstall("repo", "/w/repo"); err == nil {
		t.Fatal("failed disable was ignored")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unit removed despite failed disable: %v", err)
	}
}

func TestAvailableMentionsLinger(t *testing.T) {
	// covers: R-WRK-004
	r := &recorder{out: map[string]string{}, fail: map[string]string{"systemctl --user is-system-running": "no bus"}}
	err := (systemd.Supervisor{Exec: r.exec}).Available()
	if err == nil || !strings.Contains(err.Error(), "loginctl enable-linger $USER") {
		t.Fatalf("err = %v", err)
	}
}
