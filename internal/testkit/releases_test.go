package testkit_test

import (
	"io"
	"net/http"
	"os"
	"syscall"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// Rejects a releases command that leaves NESS_RELEASES_URL on the refused port, misorders rc ids,
// logs nothing or keeps an old log, or ignores fail; and a lock that is not held.
func TestReleasesAndLockCommands(t *testing.T) {
	cmds := testkit.Cmds()
	cmds["get"] = getCmd
	cmds["locked"] = lockedCmd
	testscript.Run(t, testscript.Params{Dir: "testdata/cmds", Setup: testkit.Setup, Cmds: cmds})
}

// getCmd is `get PATH`: GET $NESS_RELEASES_URL/PATH, the body to stdout; a non-200 answer fails
// with the status code on stdout.
func getCmd(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: get PATH")
	}
	resp, err := http.Get(ts.Getenv("NESS_RELEASES_URL") + args[0]) //nolint:gosec,noctx // the script's fake server
	ts.Check(err)
	body, err := io.ReadAll(resp.Body)
	ts.Check(err)
	ts.Check(resp.Body.Close())
	out := string(body)
	if resp.StatusCode != http.StatusOK {
		out = resp.Status[:3] + "\n"
	}
	_, err = ts.Stdout().Write([]byte(out))
	ts.Check(err)
	if (resp.StatusCode == http.StatusOK) == neg {
		ts.Fatalf("GET %s: %s", args[0], resp.Status)
	}
}

// lockedCmd is `locked PATH`: true when a non-blocking flock on PATH fails.
func lockedCmd(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: locked PATH")
	}
	f, err := os.OpenFile(ts.MkAbs(args[0]), os.O_CREATE|os.O_RDWR, 0o600)
	ts.Check(err)
	defer func() { ts.Check(f.Close()) }()
	held := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil //nolint:gosec // an fd fits an int
	if held == neg {
		ts.Fatalf("%s locked = %v", args[0], held)
	}
}
