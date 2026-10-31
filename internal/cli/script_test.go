package cli_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"ness": func() {
			// Test-only seams: scripts pick the binary version to exercise the pin (R-CFG-011)
			// and freeze the clock so ages and marks are deterministic.
			v := os.Getenv("NESS_TEST_VERSION")
			if v == "" {
				v = "0.1.0"
			}
			now := time.Now
			if s := os.Getenv("NESS_TEST_NOW"); s != "" {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					panic(err)
				}
				now = func() time.Time { return t }
			}
			d := cli.Deps{
				Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
				Version: v, Build: os.Getenv("NESS_TEST_BUILD"), Session: os.Getenv("NESS_SESSION"), Conversation: os.Getenv("CLAUDE_CODE_SESSION_ID"), Now: now, GOOS: "linux",
			}
			if p := os.Getenv("NESS_TEST_NOTES"); p != "" {
				b, err := os.ReadFile(p)
				if err != nil {
					panic(err)
				}
				d.Notes = string(b)
			}
			d.TTY = os.Getenv("NESS_TEST_TTY") == "1"
			d.Term = ui.Detect(os.Stdin, os.Stdout)
			d.Term.Dark = nil // scripts never query a terminal: dark unless NESS_THEME says light
			if dir := os.Getenv("NESS_TEST_FAKES"); dir != "" {
				d.Exec = testkit.FakeExec{Dir: dir}.Run
				d.Forge = testkit.FakeForge{Dir: dir}
				d.Transport = testkit.FakeHTTP{Dir: dir}
			}
			if v := os.Getenv("NESS_TEST_ROUTER"); v != "" {
				conf, err := strconv.ParseFloat(v, 64)
				if err != nil {
					panic(err)
				}
				off, _ := strconv.Atoi(os.Getenv("NESS_TEST_ROUTER_OFFSET")) // unset: 0
				d.Policy = testkit.FakeSelector{Offset: off, Confidence: conf}
			}
			os.Exit(cli.Main(os.Args[1:], d))
		},
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:   "testdata/script",
		Setup: testkit.Setup,
		Cmds:  testkit.Cmds(),
	})
}
