package testkit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
)

// execState is exec.json: the table of canned outputs, what is "not installed", what fails, and
// every call made, so scripts can assert what ness ran (R-QA-024).
type execState struct {
	Missing []string          `json:"missing"`
	Outputs map[string]string `json:"outputs"`
	Fail    map[string]string `json:"fail"`
	Calls   [][]string        `json:"calls"`
}

// FakeExec stands in for every external program. git runs for real (the repositories are
// throwaway); herdr is the stateful fake in herdr.go; everything else answers from the table in
// Dir/exec.json, keyed by the command line.
type FakeExec struct{ Dir string }

// Run is assignable to cli.Deps.Exec and to each atom's Exec type.
func (f FakeExec) Run(dir, name string, args ...string) ([]byte, error) {
	st, err := loadExec(f.Dir)
	if err != nil {
		return nil, err
	}
	st.Calls = append(st.Calls, append([]string{name}, args...))
	if serr := saveJSON(filepath.Join(f.Dir, "exec.json"), st); serr != nil {
		return nil, serr
	}
	for _, m := range st.Missing {
		if m == name {
			return nil, fmt.Errorf("%s: %w", name, exec.ErrNotFound)
		}
	}
	switch name {
	case "git", "sh": // sh runs the integrate gate, in a real throwaway worktree
		return cli.SystemExec(dir, name, args...)
	case "herdr":
		return FakeHerdr{Path: filepath.Join(f.Dir, "herdr.json")}.Run(dir, args)
	}
	key := strings.Join(append([]string{name}, args...), " ")
	if msg, ok := st.Fail[key]; ok {
		return nil, fmt.Errorf("%s: exit status 1: %s", key, msg)
	}
	if out, ok := st.Outputs[key]; ok {
		return []byte(out), nil
	}
	return nil, fmt.Errorf("fake exec: no fake output for %q", key)
}

// SetFake changes exec.json: kind is missing, output or fail.
func SetFake(dir, kind, key, value string) error {
	st, err := loadExec(dir)
	if err != nil {
		return err
	}
	switch kind {
	case "missing":
		st.Missing = append(st.Missing, key)
	case "output":
		st.Outputs[key] = value
		delete(st.Fail, key)
	case "fail":
		st.Fail[key] = value
	default:
		return fmt.Errorf("fake: unknown kind %s", kind)
	}
	return saveJSON(filepath.Join(dir, "exec.json"), st)
}

// CodexModels is the default `codex debug models` answer: every default Codex tier resolves.
const CodexModels = `{"models":[{"slug":"gpt-6-luna"},{"slug":"gpt-6.1-sol"},{"slug":"astra"}]}` + "\n"

// WriteFakeDefaults creates dir with a table where the tested versions answer and herdr is
// installed with no tabs.
func WriteFakeDefaults(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	st := execState{Outputs: map[string]string{
		"claude --version":                   "2.1.294 (Claude Code)\n",
		"codex --version":                    "codex-cli 0.162.1\n",
		"codex debug models":                 CodexModels,
		"systemctl --user is-system-running": "running\n",
	}, Fail: map[string]string{}, Calls: [][]string{}}
	if err := saveJSON(filepath.Join(dir, "exec.json"), st); err != nil {
		return err
	}
	if err := saveJSON(filepath.Join(dir, "herdr.json"), newHerdrState()); err != nil {
		return err
	}
	return saveJSON(filepath.Join(dir, "forge.json"), forgeState{PRs: []fakePR{}})
}

func loadExec(dir string) (execState, error) {
	var st execState
	data, err := os.ReadFile(filepath.Join(dir, "exec.json")) //nolint:gosec // the fakes dir of a test
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(data, &st)
	return st, err
}

func saveJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// fakeCmd is the `fake` script command:
//
//	fake missing NAME...        the programs are not installed
//	fake output 'CMD ARGS' TEXT exact command line → stdout TEXT (a trailing newline is added)
//	fake fail 'CMD ARGS' TEXT   exact command line → exit 1 with TEXT on stderr
//	fake calls PREFIX N         fail unless N recorded calls start with PREFIX (negatable)
//	fake herdr SUB ...          the fake herdr (see herdrCmd)
//	fake forge SUB ...          the fake forge: prs N | merge URL | last VAR
//	fake http SUB ...           the fake network (see httpCmd)
func fakeCmd(ts *testscript.TestScript, neg bool, args []string) {
	dir := ts.Getenv("NESS_TEST_FAKES")
	if dir == "" || len(args) < 2 {
		ts.Fatalf("usage: fake missing|output|fail|calls|herdr|forge ...")
	}
	switch args[0] {
	case "missing", "output", "fail":
		fakeSet(ts, neg, dir, args)
	case "calls":
		fakeCalls(ts, neg, dir, args[1:])
	case "herdr":
		herdrCmd(ts, neg, filepath.Join(dir, "herdr.json"), args[1:])
	case "forge":
		if len(args) < 2 {
			ts.Fatalf("usage: fake forge prs N | merge URL | last VAR")
		}
		forgeCmd(ts, neg, dir, args[1:])
	case "http":
		httpCmd(ts, neg, dir, args[1:])
	default:
		ts.Fatalf("fake: unknown subcommand %s", args[0])
	}
}

func fakeSet(ts *testscript.TestScript, neg bool, dir string, args []string) {
	if args[0] == "missing" {
		for _, name := range args[1:] {
			ts.Check(SetFake(dir, "missing", name, ""))
		}
		return
	}
	if neg || len(args) != 3 {
		ts.Fatalf("usage: fake %s 'CMD ARGS' TEXT", args[0])
	}
	ts.Check(SetFake(dir, args[0], args[1], args[2]+"\n"))
}

func fakeCalls(ts *testscript.TestScript, neg bool, dir string, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: fake calls PREFIX N")
	}
	st, err := loadExec(dir)
	ts.Check(err)
	want, err := strconv.Atoi(args[1])
	ts.Check(err)
	n := 0
	for _, c := range st.Calls {
		if strings.HasPrefix(strings.Join(c, " "), args[0]) {
			n++
		}
	}
	if (n == want) == neg {
		ts.Fatalf("%d calls start with %q, want %d", n, args[0], want)
	}
}
