// Package cli implements the ness command line. Commands are thin: they parse flags, call the
// internal packages and write to the injected writers.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"time"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
	"github.com/enea-scaccabarozzi/ness/internal/update"
)

// Exit codes (R-CFG-035).
const (
	ExitOK       = 0
	ExitProblems = 1
	ExitUsage    = 2
	ExitConflict = 3
	ExitRefused  = 4
)

// Deps carries what a command needs from the process.
type Deps struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Version is the release version: the project pin, the lock and the release notes use it.
	Version string
	// Build names the commit the binary was built from ("" when unknown); --version, the worker's
	// heartbeat and its restart check show Version+Build (R-WRK-009).
	Build string
	// Notes is the release notes text; empty means the embedded CHANGELOG. Scripts load a fixture with NESS_TEST_NOTES.
	Notes string
	// Session is the calling session's id, from NESS_SESSION; empty means the operator
	// (R-SESS-037). Only the composition roots read the variable (R-CFG-002).
	Session string
	// Conversation is the client conversation id, from CLAUDE_CODE_SESSION_ID; it identifies the
	// root adopted with --here when Session is empty (R-SESS-114). Only the composition roots read it.
	Conversation string
	// Now is the clock. A command reads it once per run and passes the value down.
	Now func() time.Time
	// Exec runs external programs; nil means SystemExec. Tests inject testkit.FakeExec.
	Exec func(dir, name string, args ...string) ([]byte, error)
	// GOOS is the operating system; empty means runtime.GOOS. v1 runs on Linux only (R-SESS-015).
	GOOS string
	// Policy is the model policy; nil means the text policy. Tests inject a router.
	Policy session.Policy
	// Forge is the pull-request forge; nil means the gh CLI. Tests inject testkit.FakeForge.
	Forge session.Forge
	// Shell runs commands with stdin, a timeout and an exit code (the command channel, probe
	// floors); nil means shell.Run. Scripts keep the real one: the commands are their own.
	Shell shell.Runner
	// Transport carries every HTTP request (the channel atoms); nil means http.DefaultTransport.
	// Tests inject testkit.FakeHTTP, which records and never reaches the network.
	Transport http.RoundTripper
	// TTY reports that stdin is a terminal: init prompts for missing values (R-CFG-051).
	// cmd/ness sets it; scripts force it with NESS_TEST_TTY=1.
	TTY bool
	// Term is what the process knows about its terminal; the zero value is a pipe (plain).
	Term ui.Term
	// Registry is the machine registry sync records the project in (R-UPD-031); an empty Path
	// (no user config directory) turns it off.
	Registry update.Registry
	// CacheDir is $XDG_CACHE_HOME/ness, where the release check caches; "" is no cache directory.
	CacheDir string
	// Releases is the release source self-update, upgrade and the release check read.
	Releases update.Source
	// Executable is this binary's path, " (deleted)" trimmed; the only source of it in cli.
	Executable string
	// ui is the renderer this run resolved (R-UI-001), set before any command runs.
	ui *ui.Settings
}

// usage is the error of a bad invocation: exit 2.
func usage(format string, a ...any) error {
	return &exitError{code: ExitUsage, err: fmt.Errorf(format, a...)}
}

// exitError ends a command with a specific exit code; a nil err exits silently.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit %d", e.code)
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error { return e.err }

// refuse is the error of an unmet precondition: exit 4, nothing written.
func refuse(format string, a ...any) error {
	return &exitError{code: ExitRefused, err: fmt.Errorf(format, a...)}
}

// exitCode ends the command with code and no further message.
func exitCode(code int) error { return &exitError{code: code} }

// printErr writes a one-line error to w. A failed write to stderr has nowhere left to go.
func printErr(w io.Writer, err error) {
	if _, werr := fmt.Fprintln(w, "ness:", err); werr != nil {
		return
	}
}

// Main runs ness with args and returns the process exit code.
func Main(args []string, d Deps) int {
	d.ui = &ui.Settings{}
	root := newRoot(d)
	root.SetArgs(args)
	s, err := ui.Resolve(d.Term, uiFlag(args))
	switch {
	case agentFacing(root, args):
		// What agents read stays plain under any setting (R-UI-003), and a bad NESS_UI never
		// fails a hook, which fails open (R-HOOK-016, R-UI-002).
		s = ui.Settings{}
	case err != nil:
		printErr(d.Stderr, err)
		return ExitUsage
	}
	*d.ui = s
	if s.Rich {
		plainFlagNames(root)
		root.SetOut(s.Writer(d.Stdout))
		err = fang.Execute(context.Background(), root, fang.WithVersion(d.fullVersion()), fang.WithoutCompletions(),
			fang.WithoutManpage(), fang.WithErrorHandler(func(io.Writer, fang.Styles, error) {}),
			fang.WithColorSchemeFunc(func(lipgloss.LightDarkFunc) fang.ColorScheme {
				return fang.DefaultColorScheme(lipgloss.LightDark(s.Dark))
			}))
	} else {
		err = root.Execute()
	}
	if err == nil {
		return ExitOK
	}
	code, hint := ExitUsage, false
	var ee *exitError
	if errors.As(err, &ee) {
		code, hint, err = ee.code, ee.code == ExitUsage, ee.err
	}
	if err != nil {
		reportErr(d, root, args, hint, err)
	}
	return code
}

// agentFacing reports a command whose output agents read (R-UI-003): the hooks, post, contract
// and probe.
func agentFacing(root *cobra.Command, args []string) bool {
	cmd, _, err := root.Find(args)
	if err != nil {
		return false
	}
	for cmd.HasParent() && cmd.Parent() != root {
		cmd = cmd.Parent()
	}
	return slices.Contains([]string{"hook", "post", "contract", "probe"}, cmd.Name())
}

// plainFlagNames drops cobra's backtick placeholders (`ID`) from flag usages: fang prints the
// usage text as is.
func plainFlagNames(c *cobra.Command) {
	clean := func(f *pflag.Flag) { f.Usage = strings.ReplaceAll(f.Usage, "`", "") }
	c.Flags().VisitAll(clean)
	c.PersistentFlags().VisitAll(clean)
	for _, sub := range c.Commands() {
		plainFlagNames(sub)
	}
}

// uiFlag is the --ui value in args, read before cobra parses them: help and flag errors render
// before any command runs. Empty means not given.
func uiFlag(args []string) string {
	for i, a := range args {
		switch {
		case a == "--":
			return ""
		case a == "--ui" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(a, "--ui="):
			return strings.TrimPrefix(a, "--ui=")
		}
	}
	return ""
}

// fullVersion is the release version with the build as semver build metadata, one token so the
// worker's restart check can compare it with the installed binary's --version (R-WRK-009).
func (d Deps) fullVersion() string {
	if d.Build == "" {
		return d.Version
	}
	return d.Version + "+" + d.Build
}

func newRoot(d Deps) *cobra.Command {
	if d.ui == nil {
		d.ui = &ui.Settings{}
	}
	root := &cobra.Command{
		Use:           "ness",
		Short:         "Install and run an AI coding-agent harness in this project",
		Version:       d.fullVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.SetIn(d.Stdin)
	root.SetOut(d.Stdout)
	root.SetErr(d.Stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usage("%v", err) })
	// Main reads the value before parsing (uiFlag); cobra only accepts and documents it.
	root.PersistentFlags().String("ui", "auto", "renderer: auto, rich or plain (also NESS_UI)")
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		goos := d.GOOS
		if goos == "" {
			goos = runtime.GOOS
		}
		if goos != "linux" {
			return refuse("ness v1 runs on Linux only; this is %s", goos)
		}
		return nil
	}
	root.AddCommand(newInitCmd(d), newSyncCmd(d), newHookCmd(d), newSendCmd(d), newInboxCmd(d), newReplyCmd(d),
		newOpenCmd(d), newStatusCmd(d), newSizeCmd(d), newIntegrateCmd(d), newCloseCmd(d), newDoctorCmd(d),
		newWorkerCmd(d), newPostCmd(d), newContractCmd(d), newProbeCmd(d), newStatsCmd(d), newSecretCmd(d),
		newPickCmd(d), newSelfUpdateCmd(d), newUpgradeCmd(d))
	root.InitDefaultHelpCmd()
	if h, _, err := root.Find([]string{"help"}); err == nil && h != root {
		h.Hidden = true // the rich help lists available commands; plain's template below does the same
	}
	// cobra lists the help command even when hidden; the template stops that so only the available commands show.
	root.SetUsageTemplate(strings.Replace(root.UsageTemplate(), `(or .IsAvailableCommand (eq .Name "help"))`,
		`.IsAvailableCommand`, 1))
	return root
}
