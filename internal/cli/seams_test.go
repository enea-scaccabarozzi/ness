package cli

import (
	"net/http"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

type stubTransport struct{}

func (stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrNotSupported
}

func TestShellAndTransportSeamsDefaultAndInject(t *testing.T) {
	// The channel atoms post through the injected transport and run through the injected shell;
	// the defaults are the real ones, and a request always has a timeout.
	// covers: R-NTF-029
	if r, err := (Deps{}).shell()(shell.Cmd{Command: "echo hi"}); err != nil || string(r.Output) != "hi\n" {
		t.Fatalf("default shell: %q, %v", r.Output, err)
	}
	var d Deps
	d.Shell = func(shell.Cmd) (shell.Result, error) { return shell.Result{Exit: 7}, nil }
	if r, _ := d.shell()(shell.Cmd{}); r.Exit != 7 {
		t.Fatalf("injected shell not used: %+v", r)
	}
	if (Deps{}).transport() != http.DefaultTransport {
		t.Fatal("default transport")
	}
	d.Transport = stubTransport{}
	c := d.httpClient()
	if c.Transport != d.Transport || c.Timeout != 30*time.Second {
		t.Fatalf("client = %+v", c)
	}
}
