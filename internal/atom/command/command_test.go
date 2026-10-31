package command_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/command"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestCommandGetsRawMarkdownOnStdinAndTheKeyInEnv(t *testing.T) {
	// covers: R-NTF-029
	dir := t.TempDir()
	c := command.Channel{Command: "cat > out; env > env", Dir: dir, Run: shell.Run}
	md := "# T\n\n" + strings.Repeat("x", 100000) + "\n\n| a |\n|---|\n| 1 |"
	parts, err := notify.Split(c, notify.Parse(md))
	if err != nil || len(parts) != 1 {
		t.Fatalf("no limit: %d parts, %v", len(parts), err)
	}
	if string(c.Payload(parts[0], "k")) != md {
		t.Fatalf("payload differs from the input")
	}
	id, err := c.Post(parts[0], "rel-1")
	if err != nil || id != "rel-1" {
		t.Fatalf("Post = %q, %v", id, err)
	}
	out, _ := os.ReadFile(filepath.Join(dir, "out"))
	env, _ := os.ReadFile(filepath.Join(dir, "env"))
	if string(out) != md || !strings.Contains(string(env), "NESS_THREAD_KEY=rel-1\n") {
		t.Fatalf("out %q env has key: %v", out, strings.Contains(string(env), "NESS_THREAD_KEY=rel-1"))
	}
	c.Command = "echo boom >&2; exit 2"
	if _, err = c.Post(parts[0], ""); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("a failing command: %v", err)
	}
}

func TestCommandTextNeverReachesTheShellStringOrEnv(t *testing.T) {
	// Security: the message is stdin only; the command line is the configured string and the
	// only env added is NESS_THREAD_KEY.
	// covers: R-NTF-029
	var got shell.Cmd
	run := func(c shell.Cmd) (shell.Result, error) { got = c; return shell.Result{}, nil }
	c := command.Channel{Command: "notify-send", Dir: "/d", Run: run}
	const text = "$(touch /tmp/pwned); `id`"
	if _, err := c.Post(notify.Part{Blocks: []string{text}}, "k"); err != nil {
		t.Fatal(err)
	}
	if got.Command != "notify-send" || got.Stdin != text || len(got.Env) != 1 || got.Env[0] != "NESS_THREAD_KEY=k" {
		t.Fatalf("Cmd = %+v", got)
	}
}

func TestCommandErrorIsShortAndMemoryBounded(t *testing.T) {
	// A command failing with 10 MB of output gives a short error, and masks a configured secret.
	// covers: R-NTF-029
	c := command.Channel{Command: "echo sekret >&2; head -c 10000000 /dev/zero | tr '\\0' x; exit 4",
		Dir: t.TempDir(), Run: shell.Run, Secret: "sekret"}
	_, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "")
	if err == nil || len(err.Error()) > 700 || strings.Contains(err.Error(), "sekret") {
		t.Fatalf("error = %.200q (len %d)", err, len(err.Error()))
	}
}

func TestCommandAlwaysHasATimeout(t *testing.T) {
	// A hung command is killed and the error names the timeout; the default is 30 s.
	// covers: R-NTF-029
	var got shell.Cmd
	run := func(c shell.Cmd) (shell.Result, error) { got = c; return shell.Result{}, nil }
	if _, err := (command.Channel{Command: "x", Run: run}).Post(notify.Part{Blocks: []string{"a"}}, ""); err != nil ||
		got.Timeout != 30*time.Second {
		t.Fatalf("default timeout = %v, %v", got.Timeout, err)
	}
	c := command.Channel{Command: "sleep 30", Dir: t.TempDir(), Run: shell.Run, Timeout: 100 * time.Millisecond}
	if _, err := c.Post(notify.Part{Blocks: []string{"a"}}, ""); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("hung command: %v", err)
	}
}

func TestCommandContract(t *testing.T) {
	// covers: R-NTF-005, R-NTF-029, R-QA-024
	dir := t.TempDir()
	c := command.Channel{Command: "cat >> posts; echo \"$NESS_THREAD_KEY\" >> keys", Dir: dir, Run: shell.Run}
	testkit.ChannelContract(t, c, "k1", func(int) (string, string) {
		keys, err := os.ReadFile(filepath.Join(dir, "keys")) //nolint:gosec // the test's own file
		if err != nil {
			t.Fatal(err)
		}
		// The second post's key is the first post's id (k1), shaped as the GitHub url form the
		// contract recognises.
		return "/issues/" + strings.Split(string(keys), "\n")[1] + "/", ""
	})
}
