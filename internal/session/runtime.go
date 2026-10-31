// Package session owns the session tree: the seams for the runtime, the client and the model
// policy, the environment checks, the launch state machine and the phase names (design §2, §4,
// §5). Everything that runs a program goes through an injected Exec.
package session

import "errors"

var (
	// ErrNotInstalled is returned by Runtime.Version and Client.Version for a missing program.
	ErrNotInstalled = errors.New("not installed")
	// ErrGone is returned when a tab, pane or agent no longer exists.
	ErrGone = errors.New("gone")
	// ErrBlocked is returned when the agent waits on a dialog and the runtime sent nothing.
	ErrBlocked = errors.New("blocked on a dialog")
	// ErrNotSubmitted is returned when a prompt went into the client's box but the client never
	// took its Enter (R-SESS-150).
	ErrNotSubmitted = errors.New("the prompt was not submitted")
)

// Tab is an open runtime tab, its root pane, its label and the working directory of that pane.
type Tab struct{ ID, Pane, Label, Dir string }

// TabSpec describes the tab to open: the working directory and environment of its shell, and
// Near, the id of a tab the new one opens beside (its workspace); "" is the runtime's default place.
type TabSpec struct {
	Dir, Label, Near string
	Env              map[string]string
}

// AgentSpec starts a client in a pane: Name is the runtime's agent name (unique, R-SESS-036),
// Kind the runtime's name for the client (claude, codex), Args the client's command line.
type AgentSpec struct {
	Name, Pane, Kind string
	Args             []string
}

// PaneState is what the runtime knows of a pane's agent: Status idle|working|blocked|done|unknown,
// whether an agent is there at all, its name (R-SESS-036) and whether the operator's focus is on
// the pane (R-WRK-017).
type PaneState struct {
	Status  string
	Live    bool
	Agent   string
	Focused bool
	// ClientSession is the client's own conversation id herdr reports for the agent; it survives
	// herdr clearing the agent's name (R-WRK-084).
	ClientSession string
}

// Runtime is the terminal multiplexer seam (R-SESS-110): herdr in v1, the test fake, tmux later.
// Thirteen methods because the design lists them (open tab, run, type, read, state, agents), the
// worker reads the screen as ANSI (R-WRK-015) and a launch submits the first prompt to a ready
// client and presses Enter on one the client dropped (R-SESS-025, R-SESS-150); each has two
// implementations.
type Runtime interface {
	Version() (string, error)
	OpenTab(TabSpec) (Tab, error)
	CloseTab(id string) error
	Tabs() ([]Tab, error)
	StartAgent(AgentSpec) error
	Agents() ([]string, error)
	State(pane string) (PaneState, error)
	ReadScreen(pane string) (string, error)
	ReadANSI(pane string) (string, error)
	TypeText(pane, text string) error
	Run(pane, command string) error
	Prompt(pane, text string) error
	PressEnter(pane string) error
}
