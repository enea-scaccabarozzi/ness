package testkit_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestFakeExecTableMissingAndGit(t *testing.T) {
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	f := testkit.FakeExec{Dir: dir}
	out, err := f.Run("", "claude", "--version")
	if err != nil || strings.TrimSpace(string(out)) != "2.1.294 (Claude Code)" {
		t.Fatalf("claude --version = %q, %v", out, err)
	}
	if _, err := f.Run("", "nope", "x"); err == nil || !strings.Contains(err.Error(), "no fake output") {
		t.Fatalf("unknown command: %v", err)
	}
	if err := testkit.SetFake(dir, "missing", "herdr", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Run("", "herdr", "--version"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing herdr: %v", err)
	}
	if out, err := f.Run(t.TempDir(), "git", "--version"); err != nil || !strings.HasPrefix(string(out), "git version") {
		t.Fatalf("git passthrough = %q, %v", out, err)
	}
}

func TestFakeHerdrTabsAndAgents(t *testing.T) {
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	f := testkit.FakeExec{Dir: dir}
	out, err := f.Run("", "herdr", "tab", "create", "--cwd", "/tmp/x", "--label", "root", "--env", "NESS_SESSION=s-1", "--no-focus")
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Result struct {
			Tab struct {
				TabID string `json:"tab_id"`
			} `json:"tab"`
			RootPane struct {
				PaneID string `json:"pane_id"`
			} `json:"root_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &created); err != nil || created.Result.Tab.TabID != "wF:t1" || created.Result.RootPane.PaneID != "wF:p1" {
		t.Fatalf("tab create = %s, %v", out, err)
	}
	if _, err := f.Run("", "herdr", "agent", "start", "p-root", "--kind", "claude", "--pane", "wF:p1", "--", "--effort", "medium", "hi"); err != nil {
		t.Fatal(err)
	}
	out, _ = f.Run("", "herdr", "agent", "list")
	if !strings.Contains(string(out), `"name":"p-root"`) || !strings.Contains(string(out), `"pane_id":"wF:p1"`) {
		t.Fatalf("agent list = %s", out)
	}
	if _, err := f.Run("", "herdr", "agent", "start", "p-root", "--kind", "claude", "--pane", "wF:p1"); err == nil ||
		!strings.Contains(err.Error(), "agent_name_taken") {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := f.Run("", "herdr", "tab", "close", "wF:t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Run("", "herdr", "tab", "close", "wF:t1"); err == nil || !strings.Contains(err.Error(), "tab_not_found") {
		t.Fatalf("closing a closed tab: %v", err)
	}
	out, _ = f.Run("", "herdr", "tab", "list")
	if strings.Contains(string(out), "wF:t1") {
		t.Fatalf("closed tab listed: %s", out)
	}
}

func TestFakeExecRunsShForReal(t *testing.T) {
	// The integrate gate is `sh -c <gate>`; it runs for real and is recorded (A2, R-QA-024).
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	out, err := testkit.FakeExec{Dir: dir}.Run(t.TempDir(), "sh", "-c", "echo gated")
	if err != nil || strings.TrimSpace(string(out)) != "gated" {
		t.Fatalf("sh = %q, %v", out, err)
	}
}

func TestFakeHTTPJevRoute(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "jev", "v1"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "jev", "v1", "models"), []byte("401\n{\"detail\":\"no\"}"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := http.Client{Transport: testkit.FakeHTTP{Dir: dir}}
	for url, want := range map[string]int{"https://api.typesafe.ai/v1/models": 401, "https://api.typesafe.ai/v1/systemone": 404} {
		resp, err := c.Get(url) //nolint:noctx // fake transport
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close() //nolint:errcheck // in-memory body
		if resp.StatusCode != want {
			t.Fatalf("%s: status %d, want %d", url, resp.StatusCode, want)
		}
	}
	if _, err := c.Get("https://jev.invalid/v1/models"); err == nil { //nolint:noctx,bodyclose // fails before a body
		t.Fatal("unknown host answered")
	}
}
