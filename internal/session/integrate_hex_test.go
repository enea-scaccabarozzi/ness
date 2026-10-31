package session

import (
	"io"
	"strings"
	"testing"
)

// Rejects a gate runner that builds its NESS_BASE line from any base: only a commit hash is
// quoted into the shell line, so a caller that passes something else cannot inject a command.
func TestRunInRefusesABaseThatIsNotAHash(t *testing.T) {
	// covers: R-SESS-156
	ran := false
	g := Git{Exec: func(string, string, ...string) ([]byte, error) { ran = true; return nil, nil }}
	err := runIn(g, t.TempDir(), "x'; touch pwned; '", "true", io.Discard, "")
	if err == nil || !strings.Contains(err.Error(), "not a commit hash") || ran {
		t.Fatalf("err = %v, ran = %v; want a refusal before anything runs", err, ran)
	}
	if err = runIn(g, t.TempDir(), strings.Repeat("a", 40), "true", io.Discard, ""); err != nil || !ran {
		t.Fatalf("a hash must run: err = %v, ran = %v", err, ran)
	}
}
