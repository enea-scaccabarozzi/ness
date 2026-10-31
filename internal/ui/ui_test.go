package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

func envOf(kv ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) string { return m[k] }
}

func TestResolveMode(t *testing.T) {
	// covers: R-UI-001
	cases := []struct {
		name string
		tty  bool
		flag string
		env  []string
		rich bool
	}{
		{"terminal", true, "", nil, true},
		{"pipe", false, "", nil, false},
		{"flag rich on a pipe", false, "rich", nil, true},
		{"flag plain on a terminal", true, "plain", nil, false},
		{"flag beats the variable", true, "plain", []string{"NESS_UI", "rich"}, false},
		{"flag auto defers to the variable", false, "auto", []string{"NESS_UI", "rich"}, true},
		{"variable rich on a pipe", false, "", []string{"NESS_UI", "rich"}, true},
		{"variable plain on a terminal", true, "", []string{"NESS_UI", "plain"}, false},
		{"variable auto", true, "", []string{"NESS_UI", "auto"}, true},
		{"dumb terminal", true, "", []string{"TERM", "dumb"}, false},
		{"CI", true, "", []string{"CI", "true"}, false},
		{"session identity", true, "", []string{"NESS_SESSION", "s-1"}, false},
		{"session identity forced rich", true, "rich", []string{"NESS_SESSION", "s-1"}, true},
		{"session identity beats the variable", true, "", []string{"NESS_SESSION", "s-1", "NESS_UI", "rich"}, false},
		{"adopted root", true, "", []string{"CLAUDE_CODE_SESSION_ID", "u-1"}, false},
		{"Claude Code shell", true, "", []string{"CLAUDECODE", "1", "NESS_UI", "rich"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := ui.Resolve(ui.Term{StdoutTTY: c.tty, Getenv: envOf(c.env...)}, c.flag)
			if err != nil {
				t.Fatal(err)
			}
			if s.Rich != c.rich {
				t.Fatalf("rich = %v, want %v", s.Rich, c.rich)
			}
		})
	}
}

func TestResolveRefusesUnknownMode(t *testing.T) {
	// covers: R-UI-002
	if _, err := ui.Resolve(ui.Term{Getenv: envOf("NESS_UI", "fancy", "NESS_SESSION", "s-1")}, ""); err != nil {
		t.Fatalf("an agent's bad NESS_UI is not its to fix: %v", err)
	}
	for _, in := range []struct{ flag, env string }{{"fancy", ""}, {"", "fancy"}} {
		_, err := ui.Resolve(ui.Term{Getenv: envOf("NESS_UI", in.env)}, in.flag)
		if err == nil || !strings.Contains(err.Error(), ui.Modes) {
			t.Fatalf("%+v: err = %v, want the accepted values", in, err)
		}
	}
}

func TestResolveTheme(t *testing.T) {
	// covers: R-UI-011
	queried := 0
	dark := func() bool { queried++; return false }
	s, _ := ui.Resolve(ui.Term{StdoutTTY: true, Getenv: envOf("NESS_THEME", "dark"), Dark: dark}, "")
	if !s.Dark || queried != 0 {
		t.Fatalf("NESS_THEME=dark: dark=%v queried=%d", s.Dark, queried)
	}
	s, _ = ui.Resolve(ui.Term{StdoutTTY: true, Getenv: envOf("NESS_THEME", "light"), Dark: dark}, "")
	if s.Dark || queried != 0 {
		t.Fatalf("NESS_THEME=light: dark=%v queried=%d", s.Dark, queried)
	}
	_, _ = ui.Resolve(ui.Term{StdoutTTY: false, Dark: dark}, "")
	if queried != 0 {
		t.Fatal("the plain renderer queried the terminal")
	}
	s, _ = ui.Resolve(ui.Term{StdoutTTY: true, Dark: dark}, "")
	if s.Dark || queried != 1 {
		t.Fatalf("detection: dark=%v queried=%d", s.Dark, queried)
	}
}

func TestResolveWidth(t *testing.T) {
	// covers: R-UI-012
	size := func(n int) func() int { return func() int { return n } }
	cases := []struct {
		size func() int
		env  []string
		want int
	}{
		{size(132), []string{"COLUMNS", "80"}, 132},
		{size(0), []string{"COLUMNS", "80"}, 80},
		{nil, []string{"COLUMNS", "junk"}, 100},
		{nil, nil, 100},
	}
	for _, c := range cases {
		s, _ := ui.Resolve(ui.Term{StdoutTTY: true, Getenv: envOf(c.env...), Size: c.size}, "")
		if s.Width != c.want {
			t.Fatalf("%v: width %d, want %d", c.env, s.Width, c.want)
		}
	}
}

func TestResolveAccessible(t *testing.T) {
	// covers: R-UI-013
	s, _ := ui.Resolve(ui.Term{StdoutTTY: true, Getenv: envOf("ACCESSIBLE", "1")}, "")
	if !s.Rich || !s.Accessible || s.Animate {
		t.Fatalf("ACCESSIBLE: %+v", s)
	}
	if s, _ = ui.Resolve(ui.Term{StdoutTTY: true}, ""); !s.Animate {
		t.Fatalf("a terminal does not animate: %+v", s)
	}
	if s, _ = ui.Resolve(ui.Term{}, "rich"); s.Animate {
		t.Fatalf("a pipe animates: %+v", s)
	}
}

func TestWriterDownsamples(t *testing.T) {
	// covers: R-UI-010
	var b strings.Builder
	s := ui.Settings{Rich: true, Profile: colorprofile.NoTTY}
	_, _ = s.Writer(&b).Write([]byte("\x1b[31mred\x1b[0m"))
	if b.String() != "red" {
		t.Fatalf("NoTTY kept escapes: %q", b.String())
	}
}

func TestNoColorBeatsForce(t *testing.T) {
	// covers: R-UI-010
	s, _ := ui.Resolve(ui.Term{Profile: colorprofile.TrueColor, Getenv: envOf("NO_COLOR", "1", "CLICOLOR_FORCE", "1")}, "rich")
	if s.Profile != colorprofile.ASCII {
		t.Fatalf("profile %v, want ASCII", s.Profile)
	}
}
