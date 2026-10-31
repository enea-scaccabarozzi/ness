package claude_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func fakeClient(t *testing.T) claude.Client {
	t.Helper()
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	return claude.Client{Exec: testkit.FakeExec{Dir: dir}.Run}
}

func TestClientContractOverFake(t *testing.T) {
	// covers: R-SESS-111, R-QA-024
	testkit.ClientContract(t, fakeClient(t))
}

func TestArgs(t *testing.T) {
	// No prompt in the argv (the launch submits it once the client is ready), --session-id binds
	// the transcript, effort is explicit (frontier included), --fallback-model only when set,
	// --resume on a relaunch, the bare --remote-control on both without a name, followed by a flag.
	// covers: R-SESS-025, R-SESS-027, R-SESS-028, R-SESS-047, R-SESS-120, R-MOD-004, R-MOD-010, R-MOD-044
	c := fakeClient(t)
	spec := session.LaunchSpec{Dir: "/w", Model: "fable", Effort: "medium", SessionID: "u-1"}
	got := c.Args(spec)
	want := []string{"--session-id", "u-1", "--remote-control", "--model", "fable", "--effort", "medium"}
	if !slices.Equal(got, want) {
		t.Fatalf("Args = %q, want %q", got, want)
	}
	spec.FallbackModel = "sonnet"
	if got := c.Args(spec); !slices.Contains(got, "--fallback-model") || got[slices.Index(got, "--fallback-model")+1] != "sonnet" {
		t.Fatalf("fallback: %q", got)
	}
	spec.Resume = true
	if got := c.Args(spec); got[0] != "--resume" || got[1] != "u-1" || got[2] != "--remote-control" ||
		slices.Contains(got, "--session-id") {
		t.Fatalf("resume: %q", got)
	}
}

func TestArgsName(t *testing.T) {
	// The session is named <project>/<title> on a first launch and on a resume, and the argv ends with its
	// flags; Remote Control carries the same name in one element, third.
	// covers: R-SESS-120
	c := fakeClient(t)
	spec := session.LaunchSpec{Model: "sonnet", Effort: "low", SessionID: "u-1",
		Name: session.ClientName("demo-app", "backups")}
	for _, resume := range []bool{false, true} {
		spec.Resume = resume
		got := c.Args(spec)
		i := slices.Index(got, "--name")
		if i < 0 || got[i+1] != "demo-app/backups" || got[len(got)-1] != "demo-app/backups" ||
			got[2] != "--remote-control=demo-app/backups" {
			t.Fatalf("resume=%v: %q", resume, got)
		}
	}
}

func TestSessionIDsAreUUIDs(t *testing.T) {
	// covers: R-SESS-027
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	c := fakeClient(t)
	a, b := c.NewSessionID(), c.NewSessionID()
	if !re.MatchString(a) || a == b {
		t.Fatalf("ids %q %q", a, b)
	}
}

func TestModelsAndEfforts(t *testing.T) {
	// covers: R-MOD-031, R-MOD-033, R-SESS-017
	c := fakeClient(t)
	for _, ok := range []string{"opus", "fable", "sonnet", "haiku", "claude-opus-5-5", "claude-fable-5-1"} {
		if err := c.ResolveModel(ok); err != nil {
			t.Fatalf("ResolveModel(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"gpt-6-luna", "best", "", "Opus"} {
		if err := c.ResolveModel(bad); err == nil {
			t.Fatalf("ResolveModel(%q) accepted", bad)
		}
	}
	if !slices.Equal(c.Efforts(), []string{"low", "medium", "high", "xhigh", "max"}) || slices.Contains(c.Efforts(), "minimal") {
		t.Fatalf("Efforts = %v", c.Efforts())
	}
}

func TestVersionAndCurrentSession(t *testing.T) {
	// covers: R-SESS-003, R-SESS-016, R-SESS-038
	c := fakeClient(t)
	if v, err := c.Version(); err != nil || v != "2.1.294" || c.Tested() != "2.1.294" {
		t.Fatalf("Version = %q, %v; Tested %q", v, err, c.Tested())
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "abc")
	if c.CurrentSession() != "abc" {
		t.Fatalf("CurrentSession = %q", c.CurrentSession())
	}
}

func TestTranscriptContextAndActivity(t *testing.T) {
	// The transcript lives under ~/.claude/projects/<encoded dir>/<id>.jsonl; context is the last
	// assistant usage (input + cache read + cache creation); activity is the file's mtime.
	// covers: R-SESS-080
	home, dir := t.TempDir(), "/home/x/Projects/app/.worktrees/s-1"
	p := filepath.Join(home, ".claude", "projects", "-home-x-Projects-app--worktrees-s-1", "u-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":10,"cache_creation_input_tokens":200,"cache_read_input_tokens":3000,"output_tokens":5}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":2,"cache_creation_input_tokens":192,"cache_read_input_tokens":193021,"output_tokens":47}}}`,
		`not json at all`,
	}, "\n")
	if err := os.WriteFile(p, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	tr, err := fakeClient(t).Transcript(home, dir, "u-1")
	if err != nil || tr.Context != 193215 || !tr.LastActive.Equal(stamp) {
		t.Fatalf("Transcript = %+v, %v", tr, err)
	}
	if _, err := fakeClient(t).Transcript(home, dir, "u-2"); err == nil {
		t.Fatal("missing transcript accepted")
	}
}

func TestEnterpriseSkillsPath(t *testing.T) {
	// covers: R-SESS-014
	if claude.EnterpriseSkills != "/etc/claude-code/skills" {
		t.Fatalf("EnterpriseSkills = %q", claude.EnterpriseSkills)
	}
	if !slices.Contains(session.SkillRoots("/h"), claude.EnterpriseSkills) { // session cannot import claude
		t.Fatalf("session.SkillRoots lacks %s", claude.EnterpriseSkills)
	}
}

func TestProjectDirEncoding(t *testing.T) {
	// covers: R-SESS-080
	for dir, want := range map[string]string{
		"/home/x/Projects/app/.worktrees/s-1": "-home-x-Projects-app--worktrees-s-1",
		"/home/x/My Projects/app":             "-home-x-My-Projects-app",
		"/home/x/my_app":                      "-home-x-my-app",
		"/home/x/a.b":                         "-home-x-a-b",
	} {
		if got := claude.ProjectDir("/h", dir); got != filepath.Join("/h/.claude/projects", want) {
			t.Fatalf("ProjectDir(%q) = %q, want %q", dir, got, want)
		}
	}
}
