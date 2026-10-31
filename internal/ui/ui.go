// Package ui renders ness's human-facing output: it picks the plain or the rich renderer for a
// run and draws the rich one (styles, tables, trees, Markdown, forms, the live status view).
package ui

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
)

// Term is what the process knows about its terminal, gathered by the composition root.
type Term struct {
	// StdoutTTY reports that stdout is a terminal.
	StdoutTTY bool
	// Getenv reads the process environment; nil reads nothing.
	Getenv func(string) string
	// Profile is stdout's color profile (it already folds in NO_COLOR and CLICOLOR_FORCE).
	Profile colorprofile.Profile
	// Size returns the terminal width, 0 when unknown.
	Size func() int
	// Dark queries the terminal background; called only when nothing else decides it.
	Dark func() bool
}

// Settings is the renderer a run uses.
type Settings struct {
	Rich bool
	// Profile is the color profile rich output is downsampled to.
	Profile colorprofile.Profile
	Dark    bool
	Width   int
	// Accessible replaces animation (spinners, the full-screen view) with plain lines (R-UI-013).
	Accessible bool
	// Animate allows spinners and the full-screen view: a terminal, not accessible.
	Animate bool
}

// Modes are the accepted --ui and NESS_UI values.
const Modes = "auto, rich, plain"

// defaultWidth is the layout width when neither the terminal nor COLUMNS gives one.
const defaultWidth = 100

// Resolve picks the renderer: the flag, then NESS_UI, then the detection (R-UI-001).
func Resolve(t Term, flag string) (Settings, error) {
	env := t.Getenv
	if env == nil {
		env = func(string) string { return "" }
	}
	rich, err := richMode(t, env, flag)
	if err != nil || !rich {
		return Settings{}, err
	}
	s := Settings{Rich: true, Profile: t.Profile, Width: width(t, env), Accessible: env("ACCESSIBLE") != ""}
	s.Animate = t.StdoutTTY && !s.Accessible
	if env("NO_COLOR") != "" { // NO_COLOR wins over CLICOLOR_FORCE (R-UI-010)
		s.Profile = min(s.Profile, colorprofile.ASCII)
	}
	switch env("NESS_THEME") {
	case "dark":
		s.Dark = true
	case "light":
	default:
		s.Dark = t.Dark == nil || t.Dark()
	}
	return s, nil
}

// richMode applies the mode table: the flag; then plain for an agent, whatever NESS_UI says,
// since agents inherit the operator's environment; then NESS_UI; then a terminal CI does not run.
func richMode(t Term, env func(string) string, flag string) (bool, error) {
	mode := flag
	if mode == "" || mode == "auto" {
		if agent(env) {
			return false, nil
		}
		mode = env("NESS_UI")
	}
	switch mode {
	case "rich":
		return true, nil
	case "plain":
		return false, nil
	case "", "auto":
		return t.StdoutTTY && env("TERM") != "dumb" && env("CI") == "", nil
	}
	return false, fmt.Errorf("unknown ui mode %q; accepted: %s", mode, Modes)
}

// agent reports a session identity: a ness session (NESS_SESSION) or a Claude Code conversation
// (CLAUDE_CODE_SESSION_ID, CLAUDECODE), which is how an adopted root is known (R-SESS-114).
// Codex exports no stable identity variable; its commands run on pipes.
func agent(env func(string) string) bool {
	return env("NESS_SESSION") != "" || env("CLAUDE_CODE_SESSION_ID") != "" || env("CLAUDECODE") != ""
}

func width(t Term, env func(string) string) int {
	if t.Size != nil {
		if w := t.Size(); w > 0 {
			return w
		}
	}
	if w, err := strconv.Atoi(env("COLUMNS")); err == nil && w > 0 {
		return w
	}
	return defaultWidth
}

// Detect gathers the terminal facts of stdout and stdin for a composition root.
func Detect(stdin, stdout *os.File) Term {
	return Term{
		StdoutTTY: term.IsTerminal(stdout.Fd()),
		Getenv:    os.Getenv,
		Profile:   colorprofile.Detect(stdout, os.Environ()),
		Size: func() int {
			w, _, err := term.GetSize(stdout.Fd())
			if err != nil {
				return 0
			}
			return w
		},
		Dark: func() bool { return lipgloss.HasDarkBackground(stdin, stdout) },
	}
}

// Writer downsamples rich output to the profile; plain output passes through.
func (s Settings) Writer(w io.Writer) io.Writer {
	if !s.Rich {
		return w
	}
	return &colorprofile.Writer{Forward: w, Profile: s.Profile}
}
