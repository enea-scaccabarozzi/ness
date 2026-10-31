//go:build e2e

package cli_test

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// These tests start no client and need no budget: the cleanup logic runs against a fake runner.

type fakeCall struct {
	dir     string
	args    string
	ctxDead bool
}

func fakeRunner(calls *[]fakeCall, failClose string) execFn {
	return func(ctx context.Context, dir string, _ []string, name string, args ...string) (string, error) {
		a := name + " " + strings.Join(args, " ")
		*calls = append(*calls, fakeCall{dir: dir, args: a, ctxDead: ctx.Err() != nil})
		switch {
		case ctx.Err() != nil:
			return "", ctx.Err()
		case strings.Contains(a, "status --json"):
			return `{"sessions":[{"id":"s1","scope":"x"},{"id":"s2","scope":"x"},{"id":"s3","scope":"y"}]}`, nil
		case failClose != "" && strings.Contains(a, "close "+failClose):
			return "boom", errors.New("exit 1")
		}
		return "", nil
	}
}

func TestCleanupClosesEverySessionInTheProjectDir(t *testing.T) {
	// covers: R-QA-027
	var calls []fakeCall
	var logs []string
	logf := func(f string, a ...any) { logs = append(logs, f) }
	closeScope(context.Background(), fakeRunner(&calls, "s1"), logf, "/proj", "x")
	var closed []string
	for _, c := range calls {
		if c.dir != "/proj" {
			t.Errorf("%s ran in %q, want the project dir", c.args, c.dir)
		}
		if strings.HasPrefix(c.args, "ness close") {
			closed = append(closed, c.args)
		}
	}
	if len(closed) != 2 || !strings.Contains(closed[0], "s1") || !strings.Contains(closed[1], "s2") {
		t.Fatalf("a failing close of s1 must not stop s2, and s3 (another scope) stays: %v", closed)
	}
	if len(logs) != 1 {
		t.Fatalf("the failed close must be logged, once: %v", logs)
	}
}

func TestCleanupRunsAfterTheTestContextIsCancelled(t *testing.T) {
	// covers: R-QA-027
	var calls []fakeCall
	old := cleanupExec
	cleanupExec = fakeRunner(&calls, "")
	t.Cleanup(func() { cleanupExec = old })
	t.Run("inner", func(t *testing.T) {
		t.Cleanup(func() { cleanup(t, "/proj", "x") }) // t.Context() is cancelled when this runs
	})
	if len(calls) != 3 { // status, close s1, close s2
		t.Fatalf("cleanup after the test ended made %d calls, want 3: %+v", len(calls), calls)
	}
	for _, c := range calls {
		if c.ctxDead || c.dir != "/proj" {
			t.Fatalf("cleanup call %+v ran on a dead context or outside the project", c)
		}
	}
}
