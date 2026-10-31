package cmdpolicy_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

func TestStderrOnlyInFailures(t *testing.T) {
	// covers: R-MOD-045
	// Rejects: stdout parsed with stderr mixed in, running without SplitStderr, and a failure
	// that drops the command's stderr.
	f := &fakeRunner{res: shell.Result{
		Output: []byte(`{"offset":1,"confidence":0.9,"reason":"ok"}`), Stderr: []byte("Installed 3 packages\n"),
	}}
	if _, err := policy(f).Choose(session.Brief{Kind: "design"}); err != nil {
		t.Fatal(err)
	}
	if !f.got[0].SplitStderr {
		t.Fatal("ran without SplitStderr")
	}
	f = &fakeRunner{res: shell.Result{Exit: 2, Stderr: []byte("no such model\n")}}
	if _, err := policy(f).Choose(session.Brief{Kind: "design"}); err == nil || !strings.Contains(err.Error(), "no such model") {
		t.Fatalf("err = %v, want the stderr", err)
	}
}
