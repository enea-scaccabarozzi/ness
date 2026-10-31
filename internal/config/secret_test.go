package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

func TestResolve(t *testing.T) {
	// A literal is itself; env: and secret: resolve; a missing one names the reference, not a value.
	// covers: R-CFG-080, R-CFG-082
	path := config.SecretsPath(t.TempDir(), "proj")
	if err := config.SetSecret(path, "tg", "123:abc"); err != nil {
		t.Fatal(err)
	}
	r := &config.SecretResolver{Path: path, Getenv: func(n string) string { return map[string]string{"T": "envval"}[n] }}
	for _, c := range []struct{ in, want, err string }{
		{"42", "42", ""},
		{"env:T", "envval", ""},
		{"secret:tg", "123:abc", ""},
		{"env:NOPE", "", "NOPE is not set"},
		{"secret:nope", "", "secret nope is not set in " + path},
	} {
		got, ref, err := r.Resolve(c.in)
		if got != c.want || ref != strings.Contains(c.in, ":") || (err == nil) != (c.err == "") ||
			(err != nil && err.Error() != c.err) {
			t.Errorf("Resolve(%q) = %q, %v, %v; want %q, err %q", c.in, got, ref, err, c.want, c.err)
		}
	}
}

func TestFile(t *testing.T) {
	// The file is 0600 in a 0700 directory, a missing one is empty, a too-open one is refused.
	// covers: R-CFG-082, R-CFG-083
	path := config.SecretsPath(t.TempDir(), "proj")
	if names, err := config.SecretNames(path); err != nil || len(names) != 0 {
		t.Fatalf("missing file: %v, %v", names, err)
	}
	for _, n := range []string{"b", "a"} {
		if err := config.SetSecret(path, n, "v-"+n); err != nil {
			t.Fatal(err)
		}
	}
	if err := config.SetSecret(path, "bad name", "x"); err == nil {
		t.Fatal("a bad name was stored")
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v", st.Mode().Perm())
	}
	if st, _ := os.Stat(filepath.Dir(path)); st.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v", st.Mode().Perm())
	}
	if names, _ := config.SecretNames(path); strings.Join(names, ",") != "a,b" {
		t.Fatalf("names = %v", names)
	}
	if found, err := config.UnsetSecret(path, "a"); !found || err != nil {
		t.Fatalf("unset a = %v, %v", found, err)
	}
	if found, _ := config.UnsetSecret(path, "a"); found {
		t.Fatal("unset twice found it")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.ReadSecrets(path); err == nil || !strings.Contains(err.Error(), "chmod 600 "+path) {
		t.Fatalf("too-open file: %v", err)
	}
}
