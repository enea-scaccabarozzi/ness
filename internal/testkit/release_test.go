package testkit_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wantAll(t *testing.T, path, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s lacks %q", path, n)
		}
	}
}

func TestGoreleaserBuildsStaticLinuxBinaries(t *testing.T) {
	// covers: R-QA-004
	g := read(t, ".goreleaser.yaml")
	wantAll(t, ".goreleaser.yaml", g, "version: 2", "main: ./cmd/ness", "CGO_ENABLED=0", "-X main.version={{.Version}}",
		"- linux", "mode: keep-existing")
	ci := read(t, ".github/workflows/ci.yml")
	wantAll(t, "ci.yml", ci, "goreleaser check", "goreleaser build --snapshot", "statically linked")
	wantAll(t, "release.yml", read(t, ".github/workflows/release.yml"), "tags:", "goreleaser release --clean")
}

func TestTheGateRunsEveryStep(t *testing.T) {
	// covers: R-QA-005, R-QA-041, R-QA-009, R-QA-044
	mk := read(t, "Makefile")
	wantAll(t, "Makefile", mk, "validate: build lint test audit vuln selfcheck", "golangci-lint run --allow-serial-runners ./...",
		"golangci-lint run --allow-serial-runners -c .golangci.tests.yml ./...", "python3 .ness/audit.py", "go test -race", "govulncheck",
		"sync --check")
	wantAll(t, "mise.toml", read(t, "mise.toml"), `python = "3.13`, `golangci-lint = "2.14.0"`, `goreleaser = "2.18.2"`)
	wantAll(t, "ci.yml", read(t, ".github/workflows/ci.yml"), "jdx/mise-action", "make validate", "make repeat")
	wantAll(t, "Makefile", mk, "repeat:\n\tgo test -count=4 ./...")
	if strings.Contains(mk[:strings.Index(mk, "repeat:")], "-count=4") {
		t.Error("-count=4 runs only in CI, in the repeat target")
	}
	wantAll(t, "lefthook.yml", read(t, "lefthook.yml"), "run: make build lint audit selfcheck")
}

func TestCommitsAreLinted(t *testing.T) {
	// covers: R-QA-030
	ci := read(t, ".github/workflows/ci.yml")
	wantAll(t, "ci.yml", ci, "pull_request", "git log --format=%s", "profiles/builtin/git-conventional/commit-msg")
}

func TestReleasePleaseHasRunningSessions(t *testing.T) {
	// covers: R-QA-031
	cfg := read(t, "release-please-config.json")
	wantAll(t, "release-please-config.json", cfg, `"release-type": "go"`, `"type": "sessions"`,
		`"section": "Running sessions"`)
	wantAll(t, ".release-please-manifest.json", read(t, ".release-please-manifest.json"), `"."`)
	wantAll(t, "release-please.yml", read(t, ".github/workflows/release-please.yml"),
		"googleapis/release-please-action", "workflow_dispatch:", "steps.rp.outputs.release_created",
		"go test ./internal/sync/", "goreleaser release --clean", "fetch-depth: 0", `grep -Eq '^sessions`)
	rp := read(t, ".github/workflows/release-please.yml")
	if strings.Index(rp, `grep -Eq '^sessions`) > strings.Index(rp, "googleapis/release-please-action") {
		t.Error("the sessions check must run before release-please")
	}
	// Every push to main updates the release pull request; only its merge must carry a sessions
	// commit, so the check is conditioned on the release commit.
	wantAll(t, "release-please.yml", rp, "push:\n    branches: [main]",
		"startsWith(github.event.head_commit.message, 'chore(main): release')")
}

func TestMutationTestingIsWeeklyAndReportOnly(t *testing.T) {
	// covers: R-QA-023
	m := read(t, ".github/workflows/mutation.yml")
	wantAll(t, "mutation.yml", m, "schedule:", "cron:", "gremlins", "internal/sync", "internal/store",
		"continue-on-error: true", "upload-artifact")
}

func TestWorkflowsArePinnedAndLeastPrivilege(t *testing.T) {
	sha := regexp.MustCompile(`uses: [\w./-]+@[0-9a-f]{40} # v\d+\.\d+\.\d+`)
	for _, f := range []string{"ci", "mutation", "release", "release-please"} {
		text := read(t, ".github/workflows/"+f+".yml")
		for _, l := range strings.Split(text, "\n") {
			if strings.Contains(l, "uses:") && !sha.MatchString(l) {
				t.Errorf("%s.yml: action not pinned to a SHA: %s", f, strings.TrimSpace(l))
			}
		}
	}
	for _, f := range []string{"ci", "mutation"} {
		wantAll(t, f+".yml", read(t, ".github/workflows/"+f+".yml"), "permissions:\n  contents: read")
	}
	wantAll(t, "mutation.yml", read(t, ".github/workflows/mutation.yml"), "timeout-minutes:")
	wantAll(t, "ci.yml", read(t, ".github/workflows/ci.yml"), "github.event.before", "github.sha")
}
