package testkit

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// HerdrTab is one open tab of the fake herdr.
type HerdrTab struct {
	ID, Pane, Label, Cwd string
	Env                  map[string]string
	Focused              bool
}

// HerdrAgent is one agent of the fake herdr.
type HerdrAgent struct {
	Name, Pane, Kind, Status string // Status: gone, or a schema AgentStatus: idle | working | blocked | done | unknown
	Args                     []string
	PendingReady             bool // started blocked: the ready mark is written when it first leaves blocked
}

// HerdrFake is the fake herdr's state file, herdr.json.
type HerdrFake struct {
	Version      string            `json:"version"`
	Next         int               `json:"next"`
	Tabs         []HerdrTab        `json:"tabs"`
	Agents       []HerdrAgent      `json:"agents"`
	Typed        [][2]string       `json:"typed"`    // pane, text (send-text and run)
	Prompted     [][2]string       `json:"prompted"` // pane, text (agent prompt)
	Screens      map[string]string `json:"screens"`
	ReadyOnStart bool              `json:"ready_on_start"`
	FailNext     string            `json:"fail_next"`    // "tab create" | "agent start": the next such call fails once
	StartStatus  string            `json:"start_status"` // the status a started agent begins with; "" is idle
	// DropEnter is how many of the next Enters (an agent prompt's or a pane send-keys Enter) the
	// client drops, as Claude Code does while it starts: the text stays in Boxes, unsubmitted.
	DropEnter int               `json:"drop_enter"`
	Boxes     map[string]string `json:"boxes"` // pane → text typed into the client's box, not submitted
	Keys      [][2]string       `json:"keys"`  // pane, keys (pane send-keys)
}

func newHerdrState() HerdrFake {
	return HerdrFake{
		Version: "0.8.2", Next: 1, Screens: map[string]string{}, ReadyOnStart: true,
		Tabs: []HerdrTab{}, Agents: []HerdrAgent{}, Typed: [][2]string{}, Prompted: [][2]string{},
		Boxes: map[string]string{}, Keys: [][2]string{},
	}
}

// FakeHerdr is the stateful fake behind `herdr …` in FakeExec. On `agent start` with
// ready_on_start it writes the ready mark the real client's SessionStart hook would write, into
// the store of the tab's cwd, for the tab's NESS_SESSION.
type FakeHerdr struct{ Path string }

func (h FakeHerdr) load() (HerdrFake, error) {
	var st HerdrFake
	data, err := os.ReadFile(h.Path) //nolint:gosec // the fakes dir of a test
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(data, &st)
	if st.Boxes == nil {
		st.Boxes = map[string]string{}
	}
	return st, err
}

func herdrErr(code, msg string) error {
	return fmt.Errorf(`herdr: exit status 1: {"error":{"code":%q,"message":%q}}`, code, msg)
}

func envelope(id string, result map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{"id": "cli:" + id, "result": result})
}

// agentStatusOf is the status of the live agent in pane, else "unknown".
func agentStatusOf(st HerdrFake, pane string) string {
	for _, a := range st.Agents {
		if a.Pane == pane && a.Status != "gone" {
			return a.Status
		}
	}
	return "unknown"
}

// tabInfo is the real TabInfo; the fake keeps one pane per tab.
func tabInfo(st HerdrFake, t HerdrTab) map[string]any {
	n := slices.IndexFunc(st.Tabs, func(x HerdrTab) bool { return x.ID == t.ID }) + 1
	return map[string]any{
		"tab_id": t.ID, "workspace_id": workspaceOf(t.ID), "number": n, "label": t.Label, "focused": t.Focused,
		"pane_count": 1, "agent_status": agentStatusOf(st, t.Pane),
	}
}

func tabOf(st HerdrFake, pane string) HerdrTab {
	i := slices.IndexFunc(st.Tabs, func(t HerdrTab) bool { return t.Pane == pane })
	if i < 0 {
		return HerdrTab{}
	}
	return st.Tabs[i]
}

// workspaceOf is the workspace of a herdr id, which is <workspace>:<rest>.
func workspaceOf(id string) string {
	ws, _, _ := strings.Cut(id, ":")
	return ws
}

func terminalID(pane string) string { return "term-" + strings.ReplaceAll(pane, ":", "-") }

// paneInfo is the real PaneInfo.
func paneInfo(st HerdrFake, pane string) map[string]any {
	t := tabOf(st, pane)
	info := map[string]any{
		"pane_id": pane, "terminal_id": terminalID(pane), "workspace_id": workspaceOf(pane), "tab_id": t.ID,
		"focused": t.Focused, "agent": nil, "agent_status": agentStatusOf(st, pane), "revision": 1, "cwd": t.Cwd,
	}
	for _, a := range st.Agents {
		if a.Pane == pane && a.Status != "gone" {
			info["agent"] = a.Kind
		}
	}
	return info
}

// agentInfo is the real AgentInfo: PaneInfo's identity fields plus the agent name.
func agentInfo(st HerdrFake, a HerdrAgent) map[string]any {
	t := tabOf(st, a.Pane)
	return map[string]any{
		"name": a.Name, "agent": a.Kind, "agent_status": a.Status, "pane_id": a.Pane, "tab_id": t.ID,
		"workspace_id": workspaceOf(a.Pane), "terminal_id": terminalID(a.Pane), "focused": t.Focused, "revision": 1,
		"cwd": t.Cwd,
	}
}

// Run answers one herdr command line.
func (h FakeHerdr) Run(_ string, args []string) ([]byte, error) {
	st, err := h.load()
	if err != nil {
		return nil, err
	}
	if len(args) == 1 && args[0] == "--version" {
		return []byte("herdr " + st.Version + "\n"), nil
	}
	if len(args) < 2 {
		return nil, herdrErr("usage", "unknown command")
	}
	verb := args[0] + " " + args[1]
	if st.FailNext == verb {
		st.FailNext = ""
		if serr := saveJSON(h.Path, st); serr != nil {
			return nil, serr
		}
		return nil, herdrErr("internal", "injected failure at "+verb)
	}
	handler, ok := herdrVerbs[verb]
	if !ok {
		return nil, herdrErr("usage", "unknown command "+verb)
	}
	out, err := handler(&st, args[2:])
	if serr := saveJSON(h.Path, st); serr != nil { // a refused start may still leave its agent
		return nil, serr
	}
	return out, err
}

func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// herdrVerbs maps "noun verb" to its handler.
var herdrVerbs = map[string]func(*HerdrFake, []string) ([]byte, error){
	"tab create":     tabCreate,
	"tab list":       tabList,
	"tab close":      tabClose,
	"pane list":      paneList,
	"agent list":     agentList,
	"agent start":    agentStart,
	"agent get":      agentGet,
	"pane read":      paneRead,
	"pane send-text": paneInput,
	"pane run":       paneInput,
	"agent prompt":   agentPrompt,
	"pane send-keys": paneSendKeys,
}

func tabCreate(st *HerdrFake, rest []string) ([]byte, error) {
	n := st.Next
	st.Next++
	ws := flagValue(rest, "--workspace")
	if ws == "" {
		ws = "wF"
	}
	t := HerdrTab{
		ID: fmt.Sprintf("%s:t%d", ws, n), Pane: fmt.Sprintf("%s:p%d", ws, n),
		Label: flagValue(rest, "--label"), Cwd: flagValue(rest, "--cwd"), Env: map[string]string{},
	}
	for i, a := range rest {
		if a == "--env" && i+1 < len(rest) {
			k, v, _ := strings.Cut(rest[i+1], "=")
			t.Env[k] = v
		}
	}
	st.Tabs = append(st.Tabs, t)
	return envelope("tab:create", map[string]any{
		"type": "tab_created", "tab": tabInfo(*st, t), "root_pane": paneInfo(*st, t.Pane),
	})
}

func tabList(st *HerdrFake, _ []string) ([]byte, error) {
	tabs := make([]map[string]any, 0, len(st.Tabs))
	for _, t := range st.Tabs {
		tabs = append(tabs, tabInfo(*st, t))
	}
	return envelope("tab:list", map[string]any{"type": "tab_list", "tabs": tabs})
}

func paneList(st *HerdrFake, _ []string) ([]byte, error) {
	panes := make([]map[string]any, 0, len(st.Tabs))
	for _, t := range st.Tabs {
		panes = append(panes, paneInfo(*st, t.Pane))
	}
	return envelope("pane:list", map[string]any{"type": "pane_list", "panes": panes})
}

func tabClose(st *HerdrFake, rest []string) ([]byte, error) {
	i := slices.IndexFunc(st.Tabs, func(t HerdrTab) bool { return t.ID == rest[0] })
	if i < 0 {
		return nil, herdrErr("tab_not_found", "tab "+rest[0]+" not found")
	}
	for j := range st.Agents {
		if st.Agents[j].Pane == st.Tabs[i].Pane {
			st.Agents[j].Status = "gone"
		}
	}
	st.Tabs = slices.Delete(st.Tabs, i, i+1)
	return envelope("tab:close", map[string]any{"type": "ok"})
}

func agentList(st *HerdrFake, _ []string) ([]byte, error) {
	agents := make([]map[string]any, 0, len(st.Agents))
	for _, a := range st.Agents {
		if a.Status != "gone" {
			agents = append(agents, agentInfo(*st, a))
		}
	}
	return envelope("agent:list", map[string]any{"type": "agent_list", "agents": agents})
}

func agentGet(st *HerdrFake, rest []string) ([]byte, error) {
	i := slices.IndexFunc(st.Agents, func(a HerdrAgent) bool {
		return (a.Name == rest[0] || a.Pane == rest[0]) && a.Status != "gone"
	})
	if i < 0 {
		return nil, herdrErr("agent_not_found", "agent target "+rest[0]+" not found")
	}
	return envelope("agent:get", map[string]any{"type": "agent_info", "agent": agentInfo(*st, st.Agents[i])})
}

func paneRead(st *HerdrFake, rest []string) ([]byte, error) {
	pane := rest[0]
	if !slices.ContainsFunc(st.Tabs, func(t HerdrTab) bool { return t.Pane == pane }) {
		return nil, herdrErr("pane_not_found", "pane "+pane+" not found")
	}
	// The CLI prints the pane's raw text, not the socket API's {"read":{"text":…}} envelope;
	// --format ansi keeps the escapes the screen was set with, text strips them.
	screen := st.Screens[pane]
	if box := st.Boxes[pane]; box != "" { // the unsubmitted prompt, drawn as the client's box
		screen += "\n❯ " + box
	}
	if flagValue(rest, "--format") == "ansi" {
		return []byte(screen), nil
	}
	return []byte(session.StripANSI(screen)), nil
}

func paneInput(st *HerdrFake, rest []string) ([]byte, error) {
	st.Typed = append(st.Typed, [2]string{rest[0], strings.Join(rest[1:], " ")})
	return nil, nil // the CLI prints nothing for send-text and run
}

// agentPrompt is `agent prompt PANE TEXT [--wait …]`: it submits TEXT to the pane's live agent, and
// refuses while that agent waits on a dialog, as herdr does. A dropped Enter leaves TEXT in the box:
// without --wait the call still succeeds (the bug herdr shows), with it the answer is
// agent_prompt_stalled. A submission takes what was already in the box with it.
func agentPrompt(st *HerdrFake, rest []string) ([]byte, error) {
	i := slices.IndexFunc(st.Agents, func(a HerdrAgent) bool {
		return (a.Pane == rest[0] || a.Name == rest[0]) && a.Status != "gone"
	})
	switch {
	case i < 0:
		return nil, herdrErr("agent_not_found", "agent target "+rest[0]+" not found")
	case st.Agents[i].Status == "blocked":
		return nil, herdrErr("agent_blocked", "agent "+st.Agents[i].Name+" is blocked on a dialog")
	}
	pane := st.Agents[i].Pane
	st.Boxes[pane] += rest[1]
	if !enter(st, pane) && slices.Contains(rest[2:], "--wait") {
		return nil, herdrErr("agent_prompt_stalled", "agent prompt produced no observed state change within 5000 ms")
	}
	return nil, nil
}

// paneSendKeys is `pane send-keys PANE KEY…`; an Enter submits the box like a prompt's Enter and,
// taken, starts the agent's turn (working).
func paneSendKeys(st *HerdrFake, rest []string) ([]byte, error) {
	if !slices.ContainsFunc(st.Tabs, func(t HerdrTab) bool { return t.Pane == rest[0] }) {
		return nil, herdrErr("pane_not_found", "pane "+rest[0]+" not found")
	}
	st.Keys = append(st.Keys, [2]string{rest[0], strings.Join(rest[1:], " ")})
	if slices.Contains(rest[1:], "Enter") && st.Boxes[rest[0]] != "" && enter(st, rest[0]) {
		for j := range st.Agents {
			if st.Agents[j].Pane == rest[0] && st.Agents[j].Status != "gone" {
				st.Agents[j].Status = "working"
			}
		}
	}
	return nil, nil
}

// enter presses Enter in the pane's box: dropped while DropEnter lasts, else the box is submitted.
func enter(st *HerdrFake, pane string) bool {
	if st.DropEnter > 0 {
		st.DropEnter--
		return false
	}
	st.Prompted = append(st.Prompted, [2]string{pane, st.Boxes[pane]})
	delete(st.Boxes, pane)
	return true
}

var agentNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// startRefusal is herdr's refusal of an agent start, nil when it may start.
func startRefusal(st *HerdrFake, name, pane string) error {
	live := func(f func(HerdrAgent) bool) bool {
		return slices.ContainsFunc(st.Agents, func(a HerdrAgent) bool { return a.Status != "gone" && f(a) })
	}
	switch {
	case !agentNameRe.MatchString(name):
		return herdrErr("invalid_agent_name", name)
	case live(func(a HerdrAgent) bool { return a.Name == name }):
		return herdrErr("agent_name_taken", name)
	case !slices.ContainsFunc(st.Tabs, func(t HerdrTab) bool { return t.Pane == pane }):
		return herdrErr("pane_not_found", "pane "+pane+" not found")
	case live(func(a HerdrAgent) bool { return a.Pane == pane }):
		return herdrErr("pane_busy", "pane "+pane+" already runs an agent")
	}
	return nil
}

func agentStart(st *HerdrFake, rest []string) ([]byte, error) {
	name, pane, kind := rest[0], flagValue(rest, "--pane"), flagValue(rest, "--kind")
	if err := startRefusal(st, name, pane); err != nil {
		return nil, err
	}
	ti := slices.IndexFunc(st.Tabs, func(t HerdrTab) bool { return t.Pane == pane })
	var args []string
	if i := slices.Index(rest, "--"); i >= 0 {
		args = rest[i+1:]
	}
	status := cmp.Or(st.StartStatus, "idle")
	st.Agents = append(st.Agents, HerdrAgent{
		Name: name, Pane: pane, Kind: kind, Status: status, Args: args, PendingReady: status == "blocked",
	})
	// A client blocked on a dialog has not started its session yet: the hook writes the mark later.
	if tab := st.Tabs[ti]; st.ReadyOnStart && status != "blocked" && tab.Env["NESS_SESSION"] != "" {
		if err := markReady(tab.Cwd, tab.Env["NESS_SESSION"]); err != nil {
			return nil, err
		}
	}
	if status == "blocked" { // herdr 0.8.2 answers at once and keeps the agent and its name
		return nil, herdrErr("agent_not_ready", "agent "+name+" is blocked during startup")
	}
	return nil, nil // the atom reads only the exit status (start is not exercised against real herdr)
}

// markReady does what the client's SessionStart hook does in a real launch (R-HOOK-003). A tab
// outside any repository has no store to mark, as with a hook that cannot find one.
func markReady(cwd, id string) (err error) {
	path, lerr := store.Locate(cwd)
	if lerr != nil {
		return nil //nolint:nilerr // no repository, no store, nothing to mark
	}
	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	return s.MarkReady(id, scriptClock())
}

func scriptClock() time.Time {
	if v := os.Getenv("NESS_TEST_NOW"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Now()
}

// herdrCmd drives and checks the fake herdr:
//
//	fake herdr version V              herdr --version answers V
//	fake herdr agent NAME PANE        a live agent of another repository
//	fake herdr tabs N                 fail unless N tabs are open (negatable)
//	fake herdr tab LABEL              fail unless a tab with LABEL is open (negatable)
//	fake herdr typed N                fail unless N texts were typed or run (negatable)
//	fake herdr args REGEX             fail unless the last agent start args, space-joined, match
//	fake herdr env KEY=VALUE          fail unless the last tab was created with that env entry (exact value)
//	fake herdr noenv KEY              fail if the last tab was created with KEY in its env
//	fake herdr agents REGEX           fail unless a live agent name matches (negatable)
//	fake herdr ready-on-start BOOL    whether agent start writes the ready mark
//	fake herdr fail 'tab create'|'agent start'   the next such call fails once
//	fake herdr close-tab LABEL        close the tab out of band
//	fake herdr exit NAME              the agent exited; its pane stays
//	fake herdr screen PANE TEXT       what pane read returns
//	fake herdr status NAME STATUS     the agent's status: idle, working, blocked, done (idle, tab unseen) or unknown
//	fake herdr workspace W            fail unless the last tab was created in workspace W (negatable)
//	fake herdr start-status STATUS    agents start with STATUS (blocked: no ready mark until the agent leaves blocked)
//	fake herdr drop-enter N           the client drops the next N Enters (the text stays in its box)
//	fake herdr prompted N             fail unless N prompts were submitted with agent prompt (negatable)
//	fake herdr prompt REGEX           fail unless the last submitted prompt text matches (negatable)
//	fake herdr focus LABEL            the operator's focus is on that tab
func herdrCmd(ts *testscript.TestScript, neg bool, path string, args []string) {
	h := FakeHerdr{Path: path}
	st, err := h.load()
	ts.Check(err)
	sub, ok := herdrSubs[args[0]]
	if !ok {
		ts.Fatalf("fake herdr: unknown subcommand %s", args[0])
	}
	sub(ts, neg, &st, args[1:])
	ts.Check(saveJSON(path, st))
}

// herdrSubs maps a `fake herdr` subcommand to its handler; args exclude the subcommand.
var herdrSubs = map[string]func(ts *testscript.TestScript, neg bool, st *HerdrFake, args []string){
	"version": func(_ *testscript.TestScript, _ bool, st *HerdrFake, a []string) { st.Version = a[0] },
	"ready-on-start": func(_ *testscript.TestScript, _ bool, st *HerdrFake, a []string) {
		st.ReadyOnStart = a[0] == "true"
	},
	"fail":   func(_ *testscript.TestScript, _ bool, st *HerdrFake, a []string) { st.FailNext = a[0] },
	"screen": func(_ *testscript.TestScript, _ bool, st *HerdrFake, a []string) { st.Screens[a[0]] = a[1] },
	"agent": func(_ *testscript.TestScript, _ bool, st *HerdrFake, a []string) {
		st.Agents = append(st.Agents, HerdrAgent{Name: a[0], Pane: a[1], Kind: "claude", Status: "idle"})
	},
	"status": func(ts *testscript.TestScript, _ bool, st *HerdrFake, a []string) {
		ts.Check(herdrStatus(st, a[0]+","+a[1]))
	},
	"start-status": func(_ *testscript.TestScript, _ bool, st *HerdrFake, a []string) { st.StartStatus = a[0] },
	"drop-enter": func(ts *testscript.TestScript, _ bool, st *HerdrFake, a []string) {
		ts.Check(herdrSetters["drop_enter"](st, a[0]))
	},
	"workspace": herdrWorkspace,
	"prompted":  herdrPrompted,
	"prompt":    herdrPrompt,
	"tabs":      herdrTabs,
	"tab":       herdrTab,
	"typed":     herdrTyped,
	"args":      herdrArgs,
	"env":       herdrEnv,
	"noenv":     herdrNoEnv,
	"agents":    herdrAgents,
	"close-tab": herdrCloseTab,
	"exit":      herdrExit,
	"focus": func(ts *testscript.TestScript, _ bool, st *HerdrFake, a []string) {
		ts.Check(herdrFocus(st, a[0]))
	},
}

// herdrStatus sets an agent's status from "name,status".
func herdrStatus(st *HerdrFake, v string) error {
	name, status, _ := strings.Cut(v, ",")
	i := slices.IndexFunc(st.Agents, func(a HerdrAgent) bool { return a.Name == name && a.Status != "gone" })
	if i < 0 {
		return fmt.Errorf("fake herdr: no live agent %q", name)
	}
	st.Agents[i].Status = status
	if st.Agents[i].PendingReady && status != "blocked" {
		st.Agents[i].PendingReady = false
		tab := tabOf(*st, st.Agents[i].Pane)
		if st.ReadyOnStart && tab.Env["NESS_SESSION"] != "" {
			return markReady(tab.Cwd, tab.Env["NESS_SESSION"])
		}
	}
	return nil
}

func herdrFocus(st *HerdrFake, label string) error {
	i := slices.IndexFunc(st.Tabs, func(t HerdrTab) bool { return t.Label == label })
	if i < 0 {
		return fmt.Errorf("fake herdr: no tab %q", label)
	}
	for j := range st.Tabs {
		st.Tabs[j].Focused = j == i
	}
	return nil
}

func herdrCheck(ts *testscript.TestScript, neg, ok bool, format string, a ...any) {
	if ok == neg {
		ts.Fatalf(format, a...)
	}
}

func herdrCount(ts *testscript.TestScript, arg string) int {
	n, err := strconv.Atoi(arg)
	ts.Check(err)
	return n
}

func herdrTabs(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	herdrCheck(ts, neg, len(st.Tabs) == herdrCount(ts, a[0]), "fake herdr has %d tabs, want %s", len(st.Tabs), a[0])
}

func herdrTab(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	open := slices.ContainsFunc(st.Tabs, func(t HerdrTab) bool { return t.Label == a[0] })
	herdrCheck(ts, neg, open, "fake herdr: tab %q open = %v", a[0], open)
}

func herdrTyped(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	n := len(st.Typed)
	herdrCheck(ts, neg, n == herdrCount(ts, a[0]), "fake herdr typed %d texts, want %s", n, a[0])
}

func herdrWorkspace(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	if len(st.Tabs) == 0 {
		ts.Fatalf("fake herdr: no tab")
	}
	got := workspaceOf(st.Tabs[len(st.Tabs)-1].ID)
	herdrCheck(ts, neg, got == a[0], "last tab is in workspace %q, want %q", got, a[0])
}

func herdrPrompted(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	n := len(st.Prompted)
	herdrCheck(ts, neg, n == herdrCount(ts, a[0]), "fake herdr was prompted %d times, want %s", n, a[0])
}

func herdrPrompt(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	if len(st.Prompted) == 0 {
		ts.Fatalf("fake herdr: no prompt submitted")
	}
	last := st.Prompted[len(st.Prompted)-1][1]
	herdrCheck(ts, neg, regexp.MustCompile(a[0]).MatchString(last), "last prompt %q does not match %q", last, a[0])
}

func herdrArgs(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	if len(st.Agents) == 0 {
		ts.Fatalf("fake herdr: no agent started")
	}
	joined := strings.Join(st.Agents[len(st.Agents)-1].Args, " ")
	herdrCheck(ts, neg, regexp.MustCompile(a[0]).MatchString(joined), "agent args %q do not match %q", joined, a[0])
}

func herdrEnv(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	if len(st.Tabs) == 0 {
		ts.Fatalf("fake herdr: no tab")
	}
	k, v, _ := strings.Cut(a[0], "=")
	got := st.Tabs[len(st.Tabs)-1].Env[k]
	herdrCheck(ts, neg, got == v, "last tab env %s = %q, want %q", k, got, v)
}

func herdrNoEnv(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	if len(st.Tabs) == 0 {
		ts.Fatalf("fake herdr: no tab")
	}
	_, set := st.Tabs[len(st.Tabs)-1].Env[a[0]]
	herdrCheck(ts, neg, !set, "last tab env has %s set", a[0])
}

func herdrAgents(ts *testscript.TestScript, neg bool, st *HerdrFake, a []string) {
	re := regexp.MustCompile(a[0])
	live := slices.ContainsFunc(st.Agents, func(g HerdrAgent) bool {
		return g.Status != "gone" && re.MatchString(g.Name)
	})
	herdrCheck(ts, neg, live, "no live agent matches %q", a[0])
}

func herdrCloseTab(ts *testscript.TestScript, _ bool, st *HerdrFake, a []string) {
	i := slices.IndexFunc(st.Tabs, func(t HerdrTab) bool { return t.Label == a[0] })
	if i < 0 {
		ts.Fatalf("fake herdr: no tab %q", a[0])
	}
	_, err := tabClose(st, []string{st.Tabs[i].ID})
	ts.Check(err)
}

func herdrExit(ts *testscript.TestScript, _ bool, st *HerdrFake, a []string) {
	i := slices.IndexFunc(st.Agents, func(g HerdrAgent) bool { return g.Name == a[0] && g.Status != "gone" })
	if i < 0 {
		ts.Fatalf("fake herdr: no agent %q", a[0])
	}
	st.Agents[i].Status = "gone"
}

// HerdrState returns the fake herdr's state for assertions in Go tests.
func HerdrState(fakes string) (HerdrFake, error) {
	return FakeHerdr{Path: filepath.Join(fakes, "herdr.json")}.load()
}

// SetHerdr changes the fake herdr from a Go test: key is fail_next, start_status, ready_on_start,
// drop_enter (how many of the next Enters the client drops), box ("pane,text": the client's
// unsubmitted box, empty text clears it), close_tab
// (a tab id), focus (a tab label), exit (an agent name), status ("name,status") or agent ("name,pane": a live
// agent of another repository).
func SetHerdr(fakes, key, value string) error {
	h := FakeHerdr{Path: filepath.Join(fakes, "herdr.json")}
	st, err := h.load()
	if err != nil {
		return err
	}
	set, ok := herdrSetters[key]
	if !ok {
		return fmt.Errorf("SetHerdr: unknown key %s", key)
	}
	if err := set(&st, value); err != nil {
		return err
	}
	return saveJSON(h.Path, st)
}

// SetHerdrScreen sets what pane read returns for pane, escapes included.
func SetHerdrScreen(fakes, pane, text string) error {
	h := FakeHerdr{Path: filepath.Join(fakes, "herdr.json")}
	st, err := h.load()
	if err != nil {
		return err
	}
	st.Screens[pane] = text
	return saveJSON(h.Path, st)
}

var herdrSetters = map[string]func(*HerdrFake, string) error{
	"focus":          herdrFocus,
	"status":         herdrStatus,
	"start_status":   func(st *HerdrFake, v string) error { st.StartStatus = v; return nil },
	"fail_next":      func(st *HerdrFake, v string) error { st.FailNext = v; return nil },
	"ready_on_start": func(st *HerdrFake, v string) error { st.ReadyOnStart = v == "true"; return nil },
	"box": func(st *HerdrFake, v string) error { // "PANE,TEXT": what the client's box holds, "" clears it
		pane, text, _ := strings.Cut(v, ",")
		if text == "" {
			delete(st.Boxes, pane)
		} else {
			st.Boxes[pane] = text
		}
		return nil
	},
	"drop_enter": func(st *HerdrFake, v string) (err error) {
		st.DropEnter, err = strconv.Atoi(v)
		return err
	},
	"close_tab": func(st *HerdrFake, v string) error {
		_, err := tabClose(st, []string{v})
		return err
	},
	"agent": func(st *HerdrFake, v string) error {
		name, pane, _ := strings.Cut(v, ",")
		st.Agents = append(st.Agents, HerdrAgent{Name: name, Pane: pane, Kind: "claude", Status: "idle"})
		return nil
	},
	"exit": func(st *HerdrFake, v string) error {
		for i := range st.Agents {
			if st.Agents[i].Name == v {
				st.Agents[i].Status = "gone"
			}
		}
		return nil
	},
}
