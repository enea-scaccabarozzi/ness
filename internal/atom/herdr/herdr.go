// Package herdr is the runtime atom over the herdr CLI (0.8.2): tabs, agents and panes through
// the socket API's JSON answers. It never reads the UI-focused pane; every call names its target.
package herdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Runtime implements session.Runtime over `herdr`. Workspace is where a tab opens when its spec
// names no tab to open beside: the caller's own workspace (HERDR_WORKSPACE_ID), "" for herdr's default.
type Runtime struct {
	Exec      Exec
	Workspace string
}

type envelope struct {
	Result json.RawMessage `json:"result"`
}

type tabInfo struct {
	TabID string `json:"tab_id"`
	Label string `json:"label"`
}

type paneInfo struct {
	PaneID string `json:"pane_id"`
	TabID  string `json:"tab_id"`
	Cwd    string `json:"cwd"`
}

type agentInfo struct {
	Name        string `json:"name"`
	AgentStatus string `json:"agent_status"`
	Focused     bool   `json:"focused"`
	Session     struct {
		Value string `json:"value"`
	} `json:"agent_session"`
}

// call runs herdr with args and decodes the result envelope into out. herdr prints an error
// envelope on stderr with exit 1; SystemExec folds that into the error text, which is kept.
func (r Runtime) call(out any, args ...string) error {
	data, err := r.raw(args...)
	if err != nil {
		return err
	}
	var env envelope
	if jerr := json.Unmarshal(data, &env); jerr != nil || env.Result == nil {
		return fmt.Errorf("herdr %s: unexpected output %.80q", strings.Join(args, " "), data)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Result, out)
}

// ok runs a command whose success is its exit status: `pane send-text` and `pane run` print
// nothing, and `agent start` is not exercised against the real CLI (it starts a client), so its
// stdout is not parsed either.
func (r Runtime) ok(args ...string) error {
	_, err := r.raw(args...)
	return err
}

// raw runs herdr and returns stdout untouched, mapping a missing binary, `*_not_found`,
// `agent_blocked` and `agent_prompt_stalled` errors.
func (r Runtime) raw(args ...string) ([]byte, error) {
	data, err := r.Exec("", "herdr", args...)
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("herdr: %w", session.ErrNotInstalled)
	}
	if err != nil {
		switch code := errorCode(err); {
		case strings.HasSuffix(code, "_not_found"):
			return nil, fmt.Errorf("%w: %w", session.ErrGone, err)
		case code == "agent_blocked":
			return nil, fmt.Errorf("%w: %w", session.ErrBlocked, err)
		case code == "agent_prompt_stalled":
			return nil, fmt.Errorf("%w: %w", session.ErrNotSubmitted, err)
		}
		return nil, err
	}
	return data, nil
}

// errorCode reads the code of herdr's `{"error":{"code","message"}}` envelope, which SystemExec
// leaves at the end of the error text; the argv before it is never matched.
func errorCode(err error) string {
	text := err.Error()
	i := strings.LastIndex(text, `{"error":`)
	if i < 0 {
		return ""
	}
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.NewDecoder(strings.NewReader(text[i:])).Decode(&e) != nil {
		return ""
	}
	return e.Error.Code
}

// Version returns herdr's version from `herdr --version`.
func (r Runtime) Version() (string, error) {
	data, err := r.Exec("", "herdr", "--version")
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("herdr: %w", session.ErrNotInstalled)
	}
	if err != nil {
		return "", err
	}
	return ParseVersion(string(data))
}

// ParseVersion reads "herdr X.Y.Z".
func ParseVersion(s string) (string, error) {
	name, v, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok || name != "herdr" || v == "" {
		return "", fmt.Errorf("herdr --version printed %q", strings.TrimSpace(s))
	}
	return v, nil
}

// OpenTab creates a tab whose shell starts in spec.Dir with spec.Env, unfocused, in the workspace
// of spec.Near (a herdr id is <workspace>:<tab>), else in r.Workspace (R-SESS-030).
func (r Runtime) OpenTab(spec session.TabSpec) (session.Tab, error) {
	args := []string{"tab", "create", "--no-focus"}
	if ws := r.workspace(spec.Near); ws != "" {
		args = append(args, "--workspace", ws)
	}
	if spec.Dir != "" {
		args = append(args, "--cwd", spec.Dir)
	}
	if spec.Label != "" {
		args = append(args, "--label", spec.Label)
	}
	keys := make([]string, 0, len(spec.Env))
	for k := range spec.Env {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		args = append(args, "--env", k+"="+spec.Env[k])
	}
	var res struct {
		Tab      tabInfo  `json:"tab"`
		RootPane paneInfo `json:"root_pane"`
	}
	if err := r.call(&res, args...); err != nil {
		return session.Tab{}, err
	}
	if res.Tab.TabID == "" || res.RootPane.PaneID == "" {
		err := errors.New("herdr tab create: answer has no tab or pane id")
		if res.Tab.TabID != "" {
			err = errors.Join(err, r.CloseTab(res.Tab.TabID))
		}
		return session.Tab{}, err
	}
	return session.Tab{ID: res.Tab.TabID, Pane: res.RootPane.PaneID}, nil
}

// workspace is the workspace of the tab near (a herdr id is <workspace>:<tab>), else r.Workspace.
func (r Runtime) workspace(near string) string {
	if ws, _, _ := strings.Cut(near, ":"); ws != "" {
		return ws
	}
	return r.Workspace
}

// CloseTab closes the tab; a tab that is already gone is ErrGone.
func (r Runtime) CloseTab(id string) error { return r.call(nil, "tab", "close", id) }

// Tabs lists open tabs with their label and first pane and its directory (the pane a launch binds to).
func (r Runtime) Tabs() ([]session.Tab, error) {
	var tabs struct {
		Tabs []tabInfo `json:"tabs"`
	}
	if err := r.call(&tabs, "tab", "list"); err != nil {
		return nil, err
	}
	var panes struct {
		Panes []paneInfo `json:"panes"`
	}
	if err := r.call(&panes, "pane", "list"); err != nil {
		return nil, err
	}
	out := make([]session.Tab, 0, len(tabs.Tabs))
	for _, t := range tabs.Tabs {
		tab := session.Tab{ID: t.TabID, Label: t.Label}
		if i := slices.IndexFunc(panes.Panes, func(p paneInfo) bool { return p.TabID == t.TabID }); i >= 0 {
			tab.Pane, tab.Dir = panes.Panes[i].PaneID, panes.Panes[i].Cwd
		}
		out = append(out, tab)
	}
	return out, nil
}

// StartAgent starts the client in the pane under a herdr agent name; herdr returns once the
// client is ready for input, or at once with agent_not_ready when it is blocked on a dialog
// (R-SESS-030, R-SESS-035).
func (r Runtime) StartAgent(spec session.AgentSpec) error {
	args := []string{"agent", "start", spec.Name, "--kind", spec.Kind, "--pane", spec.Pane, "--timeout", "120000"}
	if len(spec.Args) > 0 {
		args = append(append(args, "--"), spec.Args...)
	}
	return r.ok(args...)
}

// Agents lists the live agent names of every repository (R-SESS-036).
func (r Runtime) Agents() ([]string, error) {
	var res struct {
		Agents []agentInfo `json:"agents"`
	}
	if err := r.call(&res, "agent", "list"); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(res.Agents))
	for _, a := range res.Agents {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return names, nil
}

// State reports the agent in the pane; a pane without an agent is Live false, a pane that no
// longer exists is ErrGone.
func (r Runtime) State(pane string) (session.PaneState, error) {
	var res struct {
		Agent agentInfo `json:"agent"`
	}
	err := r.call(&res, "agent", "get", pane)
	if errors.Is(err, session.ErrGone) {
		// No agent in the pane, or no pane: tell them apart with a read.
		if _, rerr := r.ReadScreen(pane); rerr != nil {
			return session.PaneState{}, rerr
		}
		return session.PaneState{Status: "unknown"}, nil
	}
	if err != nil {
		return session.PaneState{}, err
	}
	return session.PaneState{
		Status: res.Agent.AgentStatus, Live: true, Agent: res.Agent.Name, Focused: res.Agent.Focused,
		ClientSession: res.Agent.Session.Value,
	}, nil
}

// ReadScreen returns the pane's recent output as plain text. Unlike the socket API, the CLI
// prints the raw text, not a JSON envelope; a fresh pane reads as "".
func (r Runtime) ReadScreen(pane string) (string, error) {
	data, err := r.raw("pane", "read", pane, "--source", "recent-unwrapped", "--format", "text", "--lines", "200")
	return string(data), err
}

// ReadANSI returns the visible screen with its escape sequences: what the operator sees now, not
// the scrollback, so a notice that scrolled away is gone and dim text is told from a draft
// (R-WRK-015, R-WRK-045).
func (r Runtime) ReadANSI(pane string) (string, error) {
	data, err := r.raw("pane", "read", pane, "--source", "visible", "--format", "ansi")
	return string(data), err
}

// TypeText sends literal text without Enter.
func (r Runtime) TypeText(pane, text string) error {
	return r.ok("pane", "send-text", pane, text)
}

// Run sends a command line and Enter.
func (r Runtime) Run(pane, command string) error { return r.ok("pane", "run", pane, command) }

// Prompt submits text to the agent in the pane and waits for the agent's first state change, any
// state; herdr refuses it with agent_blocked, sending nothing, while the agent waits on a dialog
// (R-SESS-025), and answers agent_prompt_stalled when nothing changed within 5 s: the client
// dropped the Enter and the text sits in its box (R-SESS-150).
func (r Runtime) Prompt(pane, text string) error {
	return r.ok("agent", "prompt", pane, text, "--wait", "--until", "working", "--until", "blocked",
		"--until", "done", "--until", "idle", "--timeout", "10000")
}

// PressEnter presses Enter in the pane. `agent send-keys` reaches herdr but not the client's box;
// `pane send-keys` does (R-SESS-150).
func (r Runtime) PressEnter(pane string) error { return r.ok("pane", "send-keys", pane, "Enter") }
