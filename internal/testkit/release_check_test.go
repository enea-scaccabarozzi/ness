package testkit_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// gitEnv is the environment for git in a scratch repo: no inherited GIT_* variables, no user or
// system config, a fixed identity.
func gitEnv(t *testing.T) []string {
	t.Helper()
	env := []string{"HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") && !strings.HasPrefix(kv, "HOME=") {
			env = append(env, kv)
		}
	}
	return env
}

func git(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
	cmd.Dir, cmd.Env = repo, gitEnv(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitInit(t *testing.T, repo string) {
	t.Helper()
	git(t, repo, "init", "-q", "-b", "main")
}

// commit changes the tree (one line per commit in log.txt) and commits it with msg.
func commit(t *testing.T, repo, msg string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(repo, "log.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(msg + "\n")
	if err = errors.Join(err, f.Close()); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", msg)
}

func gitTag(t *testing.T, repo, tag string) {
	t.Helper()
	git(t, repo, "tag", tag)
}

func mkdirAll(t *testing.T, repo, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, dir), 0o750); err != nil {
		t.Fatal(err)
	}
}

func writeChangelog(t *testing.T, repo, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "CHANGELOG.md"), []byte("# Changelog\n\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "chore: changelog")
}

// scratchRepo: feat: x, tag v0.1.0, then a sessions: commit.
func scratchRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "feat: x")
	gitTag(t, repo, "v0.1.0")
	commit(t, repo, "sessions: nothing to do")
	return repo
}

// releaseCheck runs the repository's release-check.sh with repo as its working directory.
func releaseCheck(t *testing.T, repo string, args ...string) (string, error) {
	t.Helper()
	script := filepath.Join(testkit.ModuleRoot(t), ".github/scripts/release-check.sh")
	cmd := exec.CommandContext(t.Context(), "sh", append([]string{script}, args...)...)
	cmd.Dir, cmd.Env = repo, gitEnv(t)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func readAbs(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReleaseCheckStable(t *testing.T) {
	// covers: R-UPD-092
	repo := scratchRepo(t)
	// rejects: a stable job that never asks for a release candidate
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no v0.2.0-rc.* tag: cut a release candidate") {
		t.Fatal(out)
	}
	gitTag(t, repo, "v0.2.0-rc.1")
	// rejects: a stable release with no upgrade fixture of the previous stable tag
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no upgrade fixture for v0.1.0") {
		t.Fatal(out)
	}
	mkdirAll(t, repo, "internal/cli/testdata/upgrade/v0.1.0") // untracked: the tree is unchanged
	// rejects: counting sessions: commits from the rc tag instead of the last stable tag
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err != nil {
		t.Fatal(out)
	}
}

func TestReleaseCheckStableTreeMoved(t *testing.T) {
	// covers: R-UPD-092
	repo := scratchRepo(t)
	mkdirAll(t, repo, "internal/cli/testdata/upgrade/v0.1.0")
	gitTag(t, repo, "v0.2.0-rc.1")
	commit(t, repo, "fix: y")
	// rejects: checking only that some rc tag exists, not that one has HEAD's tree
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no v0.2.0-rc.* tag has HEAD's tree: cut a new rc") {
		t.Fatal(out)
	}
	gitTag(t, repo, "v0.2.0-rc.2")
	// rejects: checking only the newest-by-name or the first rc tag instead of any
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err != nil {
		t.Fatal(out)
	}
}

func TestReleaseCheckStableOtherVersionRC(t *testing.T) {
	// covers: R-UPD-092
	repo := scratchRepo(t)
	mkdirAll(t, repo, "internal/cli/testdata/upgrade/v0.1.0")
	gitTag(t, repo, "v0.3.0-rc.1")
	// rejects: any -rc tag accepted, whatever its version
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no v0.2.0-rc.* tag: cut a release candidate") {
		t.Fatal(out)
	}
}

func TestReleaseCheckNoSessionsCommit(t *testing.T) {
	// covers: R-UPD-092
	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "feat: x")
	gitTag(t, repo, "v0.1.0")
	commit(t, repo, "feat: y")
	commit(t, repo, "docs(sessions): update-path plan")
	mkdirAll(t, repo, "internal/cli/testdata/upgrade/v0.1.0")
	gitTag(t, repo, "v0.2.0-rc.1")
	// rejects: a docs(sessions): record commit counted as a Running-sessions note
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no sessions: commit since v0.1.0") {
		t.Fatal(out)
	}
}

func TestReleaseCheckRC(t *testing.T) {
	// covers: R-UPD-091
	repo := scratchRepo(t)
	// rejects: an rc cut with no CHANGELOG at all
	if out, err := releaseCheck(t, repo, "rc", "0.2.0"); err == nil {
		t.Fatalf("no changelog section accepted: %s", out)
	}
	// rejects: any changelog section accepted, not the version's own
	writeChangelog(t, repo, "## [0.1.0] (2026-10-31)\n### Features\n* x\n")
	if out, err := releaseCheck(t, repo, "rc", "0.2.0"); err == nil {
		t.Fatalf("no 0.2.0 section accepted: %s", out)
	}
	writeChangelog(t, repo, "## [0.2.0] (2026-11-01)\n### Running sessions\n* x\n")
	// rejects: rc tags of another version counted
	gitTag(t, repo, "v0.1.0-rc.3")
	if out, err := releaseCheck(t, repo, "rc", "0.2.0"); err != nil || strings.TrimSpace(out) != "v0.2.0-rc.1" {
		t.Fatal(out, err)
	}
	gitTag(t, repo, "v0.2.0-rc.1")
	// rejects: a fixed rc.1, or N not the count of existing rc tags + 1
	if out, err := releaseCheck(t, repo, "rc", "0.2.0"); err != nil || strings.TrimSpace(out) != "v0.2.0-rc.2" {
		t.Fatal(out, err)
	}
}

func TestOneGoreleaserPerTag(t *testing.T) {
	// covers: R-UPD-090, R-QA-004
	root := testkit.ModuleRoot(t)
	// rejects: the old tag-push workflow kept beside the new ones (two goreleaser runs per tag)
	if _, err := os.Stat(filepath.Join(root, ".github/workflows/release.yml")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("release.yml still exists: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(root, ".github/workflows/*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var runners []string
	for _, f := range files {
		if strings.Contains(readAbs(t, f), "goreleaser release") {
			runners = append(runners, filepath.Base(f))
		}
	}
	if !slices.Equal(runners, []string{"rc.yml", "release-please.yml"}) {
		t.Fatalf("goreleaser runs in %v", runners)
	}
	rc := readAbs(t, filepath.Join(root, ".github/workflows/rc.yml"))
	// rejects: an rc cut on every push, pull request or schedule instead of by hand
	if !strings.Contains(rc, "workflow_dispatch") || strings.Contains(rc, "push:") || strings.Contains(rc, "pull_request") || strings.Contains(rc, "schedule:") {
		t.Fatalf("rc.yml triggers:\n%s", rc)
	}
	// rejects: the tag pushed before the notes test (R-UPD-091)
	if i, j := strings.Index(rc, "go test ./internal/sync/"), strings.Index(rc, "git push origin"); i < 0 || j < 0 || i > j {
		t.Fatalf("rc.yml must run go test ./internal/sync/ before git push origin:\n%s", rc)
	}
	// rejects: an rc published as a stable release
	if !strings.Contains(readAbs(t, filepath.Join(root, ".goreleaser.yaml")), "prerelease: auto") {
		t.Fatal(".goreleaser.yaml lacks prerelease: auto")
	}
}
