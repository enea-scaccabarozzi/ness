package probe_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
)

func example(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "example.md"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseTheDesignExample(t *testing.T) {
	// The §8 example parses; the scope is the body; every field reads back (R-PRB-001, R-PRB-002).
	// covers: R-PRB-001, R-PRB-002
	p, err := probe.Parse("probes/x.md", example(t))
	if err != nil {
		t.Fatal(err)
	}
	if p.Path != "probes/x.md" || p.Every != 30*time.Minute || p.Window != 48*time.Hour || p.Floor != "./probes/hard-wait.sh" ||
		len(p.Expect) != 1 || p.Expect[0] != (probe.Cond{Key: "n", Op: ">=", Value: 1}) || p.Confirm != 2 ||
		p.Wake != "not-green" || p.Sample != 0.1 || p.Notify != "judged" || p.Channel != "releases" || p.Once {
		t.Fatalf("Parse = %+v", p)
	}
	if !strings.HasPrefix(p.Scope, "Every hard send under the new config") || !strings.HasSuffix(p.Scope, "a GREEN is real.") {
		t.Fatalf("Scope = %q", p.Scope)
	}
	if p.Hash != probe.Hash(example(t)) || len(p.Hash) != 64 {
		t.Fatalf("Hash = %q", p.Hash)
	}
	all := "---\nevery: 1h\njitter: 5m\nonce: true\nwindow: 2d\nfloor: true\nexpect: a<1\nconfirm: 3\ntimeout: 90s\n" +
		"wake: always\nsample: 1\nnotify: auto\nchannel: c\n---\nscope\n"
	p, err = probe.Parse("a.md", []byte(all))
	if err != nil || p.Jitter != 5*time.Minute || !p.Once || p.Window != 48*time.Hour || p.Timeout != 90*time.Second ||
		p.Wake != "always" || p.Sample != 1 || p.Notify != "auto" || p.Scope != "scope" {
		t.Fatalf("every field: %+v, %v", p, err)
	}
}

func TestParseDefaults(t *testing.T) {
	// covers: R-PRB-080
	p, err := probe.Parse("a.md", []byte("---\nevery: 10m\nfloor: true\n---\n"))
	if err != nil || p.Jitter != 0 || p.Confirm != 1 || p.Timeout != 60*time.Second || p.Wake != "not-green" ||
		p.Sample != 0 || p.Notify != "none" || p.Scope != "" || p.Floorless() {
		t.Fatalf("defaults: %+v, %v", p, err)
	}
	if p, err = probe.Parse("a.md", []byte("---\nevery: 10m\n---\nlook around\n")); err != nil || !p.Floorless() {
		t.Fatalf("floorless: %+v, %v", p, err)
	}
}

func TestParseRefusals(t *testing.T) {
	// covers: R-PRB-003, R-PRB-004, R-PRB-005, R-PRB-006, R-PRB-007, R-PRB-008, R-PRB-010
	for _, tc := range []struct{ file, want string }{
		{"---\nevry: 30m\n---\n", "a.md: unknown field evry"},
		{"---\nevery: 30m\nwake: sometimes\n---\n", "wake: sometimes"},
		{"---\nevery: 30m\nnotify: maybe\n---\n", "notify: maybe"},
		{"---\nevery: 30m\nsample: 1.5\n---\n", "sample"},
		{"---\nevery: 30m\nsample: -0.1\n---\n", "sample"},
		{"---\nevery: 30m\nsample: NaN\n---\n", "sample"},
		{"---\nevery: 30m\nevery: 1h\n---\n", "line 3: duplicate field every"},
		{"---\nevery: 30m\nfloor: \"a\" b\n---\n", "line 3: text after the closing quote"},
		{"---\nevery: 30m\nconfirm: 0\n---\n", "confirm"},
		{"---\nfloor: true\n---\n", "a.md: neither every nor once"},
		{"---\nevery: 30m\nnotify: auto\n---\n", "channel"},
		{"---\nevery: 30m\nnotify: judged\n---\n", "channel"},
		{"---\nevery: 1w\n---\n", "every"},
		{"---\nevery: 30m\nexpect: n=>1\n---\n", "expect"},
		{"no frontmatter\n", "a.md: no frontmatter"},
		{"---\nevery: 30m\nnested:\n  a: b\n---\n", "line 4"},
		{"---\nevery: 30m\nonce: yes\n---\n", "once"},
	} {
		_, err := probe.Parse("a.md", []byte(tc.file))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Parse(%q) = %v; want an error containing %q", tc.file, err, tc.want)
		}
	}
	// A line past the scanner's 1 MiB cap is a read error, not "not closed".
	long := "---\nevery: 30m\nfloor: " + strings.Repeat("x", 2<<20) + "\n---\n"
	if _, err := probe.Parse("a.md", []byte(long)); err == nil || !strings.Contains(err.Error(), "too long") {
		t.Errorf("Parse(long line) = %v; want the scanner's token too long", err)
	}
	for _, ok := range []string{"never", "change", "red", "not-green", "always"} {
		if _, err := probe.Parse("a.md", []byte("---\nevery: 1m\nwake: "+ok+"\n---\n")); err != nil {
			t.Errorf("wake %s refused: %v", ok, err)
		}
	}
	for _, ok := range []string{"none", "auto", "judged"} {
		if _, err := probe.Parse("a.md", []byte("---\nevery: 1m\nnotify: "+ok+"\nchannel: c\n---\n")); err != nil {
			t.Errorf("notify %s refused: %v", ok, err)
		}
	}
}

func TestParseDuration(t *testing.T) {
	// covers: R-PRB-081
	for in, want := range map[string]time.Duration{"2d": 48 * time.Hour, "1d12h": 36 * time.Hour, "90m": 90 * time.Minute, "45s": 45 * time.Second} {
		if got, err := probe.ParseDuration(in); err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"1w", "", "d", "2dd", "-1h", "0s", "1d-1h", "1d+1h", "100001d", "99999999999999999999d", "100000d2500000h"} {
		if _, err := probe.ParseDuration(bad); err == nil {
			t.Errorf("ParseDuration(%q) accepted", bad)
		}
	}
}

func FuzzParse(f *testing.F) {
	// covers: R-PRB-009
	b, err := os.ReadFile(filepath.Join("testdata", "example.md"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(b)
	f.Add([]byte("---\n---\n"))
	f.Add([]byte("---\nevery: 1d\nonce: true\n---"))
	f.Fuzz(func(_ *testing.T, data []byte) { _, _ = probe.Parse("f.md", data) })
}

func TestQuotedValueKeepsHash(t *testing.T) {
	// P-30: a quoted value keeps its #; an unquoted " #" starts a comment.
	// covers: R-PRB-001
	p, err := probe.Parse("a.md", []byte("---\nevery: 1m # c\nfloor: \"grep -c ' #' f\" # note\n---\n"))
	if err != nil || p.Floor != "grep -c ' #' f" || p.Every != time.Minute {
		t.Fatalf("%+v, %v", p, err)
	}
}
