package herdr_test

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func fakeRuntime(t *testing.T) (herdr.Runtime, string) {
	t.Helper()
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	return herdr.Runtime{Exec: testkit.FakeExec{Dir: dir}.Run}, dir
}

func TestRuntimeContractOverFake(t *testing.T) {
	// covers: R-SESS-110, R-QA-024
	rt, _ := fakeRuntime(t)
	testkit.RuntimeContract(t, rt, true)
}

func TestHerdrErrors(t *testing.T) {
	// Review Focus 5: an error envelope, a missing program and garbage output are reported, never
	// decoded into an empty tab.
	// covers: R-SESS-001
	rt, dir := fakeRuntime(t)
	if _, err := rt.OpenTab(session.TabSpec{}); err != nil {
		t.Fatal(err)
	}
	if err := rt.CloseTab("wF:t99"); !errors.Is(err, session.ErrGone) {
		t.Fatalf("CloseTab unknown = %v", err)
	}
	if err := testkit.SetFake(dir, "missing", "herdr", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Version(); !errors.Is(err, session.ErrNotInstalled) {
		t.Fatalf("missing herdr: %v", err)
	}
	garbage := herdr.Runtime{Exec: func(string, string, ...string) ([]byte, error) { return []byte("not json"), nil }}
	if tab, err := garbage.OpenTab(session.TabSpec{}); err == nil || tab.ID != "" {
		t.Fatalf("garbage output: %+v, %v", tab, err)
	}
	noTab := func(string, string, ...string) ([]byte, error) {
		return []byte(`{"id":"x","result":{"type":"tab_created"}}`), nil
	}
	if tab, err := (herdr.Runtime{Exec: noTab}).OpenTab(session.TabSpec{}); err == nil || tab.ID != "" {
		t.Fatalf("envelope without a tab: %+v, %v", tab, err)
	}
}

func TestRealErrorEnvelopesAreGone(t *testing.T) {
	// The strings are what herdr 0.8.2 printed for tab close, agent get and pane read of ids that
	// do not exist, as SystemExec folds them. Other codes, and a code only in the argv, are not gone.
	// covers: R-SESS-001
	for verb, text := range map[string]string{
		"tab":   `herdr tab close wF:t999: exit status 1: {"error":{"code":"tab_not_found","message":"tab wF:t999 not found"},"id":"cli:tab:close"}`,
		"agent": `herdr agent get x: exit status 1: {"error":{"code":"agent_not_found","message":"agent target x not found"},"id":"cli:agent:get"}`,
		"pane":  `herdr pane read wF:p999: exit status 1: {"error":{"code":"pane_not_found","message":"pane wF:p999 not found"},"id":"cli:pane:read"}`,
	} {
		rt := herdr.Runtime{Exec: func(string, string, ...string) ([]byte, error) { return nil, errors.New(text) }}
		if err := rt.CloseTab("x"); !errors.Is(err, session.ErrGone) {
			t.Fatalf("%s: %v, want ErrGone", verb, err)
		}
	}
	for _, text := range []string{
		`herdr tab close not_found: exit status 1: {"error":{"code":"internal","message":"tab \"not_found\""}}`,
		`herdr tab close x: exit status 1: boom "not_found"`,
	} {
		rt := herdr.Runtime{Exec: func(string, string, ...string) ([]byte, error) { return nil, errors.New(text) }}
		if err := rt.CloseTab("x"); err == nil || errors.Is(err, session.ErrGone) {
			t.Fatalf("%q: %v, want a plain error", text, err)
		}
	}
}

func TestOpenTabWithoutPaneClosesTab(t *testing.T) {
	// covers: R-SESS-010
	var calls []string
	rt := herdr.Runtime{Exec: func(_ string, _ string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args[:2], " "))
		return []byte(`{"id":"x","result":{"type":"tab_created","tab":{"tab_id":"wF:t1"},"root_pane":{}}}`), nil
	}}
	if _, err := rt.OpenTab(session.TabSpec{}); err == nil {
		t.Fatal("answer without a pane accepted")
	}
	if got := strings.Join(calls, ","); got != "tab create,tab close" {
		t.Fatalf("calls = %s, want the half-made tab closed", got)
	}
}

func TestParseVersion(t *testing.T) {
	// covers: R-SESS-002
	for in, want := range map[string]string{"herdr 0.8.2\n": "0.8.2", "herdr 1.0.0-rc1": "1.0.0-rc1"} {
		if got, err := herdr.ParseVersion(in); err != nil || got != want {
			t.Fatalf("ParseVersion(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := herdr.ParseVersion("tmux 3.4"); err == nil {
		t.Fatal("tmux accepted as herdr")
	}
}

func TestOpenTabUnfocusedAndAgentStartTimeout(t *testing.T) {
	// herdr focuses a new tab by default; a child launch must not take the operator's focus. herdr's
	// own agent start timeout (30 s) is below a cold Claude start, so ness passes 120 s.
	// covers: R-SESS-030
	rt, dir := fakeRuntime(t)
	var calls []string
	rt.Exec = func(d, name string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return testkit.FakeExec{Dir: dir}.Run(d, name, args...)
	}
	tab, err := rt.OpenTab(session.TabSpec{Dir: "/w", Label: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.StartAgent(session.AgentSpec{Name: "p-x", Pane: tab.Pane, Kind: "claude"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(calls[0], "tab create --no-focus ") {
		t.Fatalf("tab create argv %q lacks --no-focus", calls[0])
	}
	if !strings.HasSuffix(calls[len(calls)-1], " --timeout 120000") {
		t.Fatalf("agent start argv %q lacks --timeout 120000", calls[len(calls)-1])
	}
}

func TestStateReportsAgentNameAndFocus(t *testing.T) {
	// The worker types only into the session's own agent when the operator is elsewhere
	// (R-WRK-017, P3a's foreign-agent rule): State carries the name and the focus of the pane.
	// covers: R-WRK-017
	rt, dir := fakeRuntime(t)
	tab, err := rt.OpenTab(session.TabSpec{Label: "root"})
	if err != nil {
		t.Fatal(err)
	}
	if err = rt.StartAgent(session.AgentSpec{Name: "p-root", Pane: tab.Pane, Kind: "claude"}); err != nil {
		t.Fatal(err)
	}
	st, err := rt.State(tab.Pane)
	if err != nil || st.Agent != "p-root" || st.Focused {
		t.Fatalf("State = %+v, %v; want agent p-root, unfocused", st, err)
	}
	if err = testkit.SetHerdr(dir, "focus", "root"); err != nil {
		t.Fatal(err)
	}
	if st, err = rt.State(tab.Pane); err != nil || !st.Focused {
		t.Fatalf("State after focus = %+v, %v; want focused", st, err)
	}
}

func TestReadANSIKeepsEscapesAndReadsTheVisibleScreen(t *testing.T) {
	// ReadScreen is the launcher's text read of the recent output; ReadANSI is the worker's read
	// of what is on screen now, escapes included, so dim text can be told from a draft (R-WRK-015).
	// covers: R-WRK-015
	rt, dir := fakeRuntime(t)
	tab, err := rt.OpenTab(session.TabSpec{Label: "root"})
	if err != nil {
		t.Fatal(err)
	}
	screen := "│ > \x1b[2mTry \"fix\"\x1b[22m\n"
	if err = testkit.SetHerdrScreen(dir, tab.Pane, screen); err != nil {
		t.Fatal(err)
	}
	if got, rerr := rt.ReadANSI(tab.Pane); rerr != nil || got != screen {
		t.Fatalf("ReadANSI = %q, %v; want the escapes kept", got, rerr)
	}
	if got, rerr := rt.ReadScreen(tab.Pane); rerr != nil || got != "│ > Try \"fix\"\n" {
		t.Fatalf("ReadScreen = %q, %v; want plain text", got, rerr)
	}
	var calls []string
	rt.Exec = func(d, name string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return testkit.FakeExec{Dir: dir}.Run(d, name, args...)
	}
	if _, err = rt.ReadANSI(tab.Pane); err != nil {
		t.Fatal(err)
	}
	if want := "pane read " + tab.Pane + " --source visible --format ansi"; calls[len(calls)-1] != want {
		t.Fatalf("ReadANSI argv %q, want %q", calls[len(calls)-1], want)
	}
	if _, err = rt.ReadANSI("wF:p99"); !errors.Is(err, session.ErrGone) {
		t.Fatalf("ReadANSI of a missing pane: %v, want ErrGone", err)
	}
}

// argvOf runs OpenTab on rt and returns the `tab create` argv and the created tab.
func argvOf(t *testing.T, rt herdr.Runtime, spec session.TabSpec) ([]string, session.Tab) {
	t.Helper()
	var create []string
	inner := rt.Exec
	rt.Exec = func(d, name string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "tab" && args[1] == "create" {
			create = args
		}
		return inner(d, name, args...)
	}
	tab, err := rt.OpenTab(spec)
	if err != nil {
		t.Fatal(err)
	}
	return create, tab
}

func TestOpenTabWorkspace(t *testing.T) {
	// A tab opens beside Near (its id is <workspace>:<tab>), else in the runtime's own workspace,
	// else herdr's default place: the flag is absent then, never empty.
	// covers: R-SESS-030
	for name, tc := range map[string]struct {
		near, workspace string
		want            string // the workspace after --workspace, "" for no flag
	}{
		"near wins":        {"w7:t3", "w9", "w7"},
		"near alone":       {"w7:t3", "", "w7"},
		"runtime fallback": {"", "w9", "w9"},
		"default place":    {"", "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			rt, _ := fakeRuntime(t)
			rt.Workspace = tc.workspace
			argv, tab := argvOf(t, rt, session.TabSpec{Dir: "/w", Label: "x", Near: tc.near})
			i := slices.Index(argv, "--workspace")
			switch {
			case tc.want == "" && i >= 0:
				t.Fatalf("argv %q passes --workspace with nothing to say", argv)
			case tc.want != "" && (i < 0 || i+1 >= len(argv) || argv[i+1] != tc.want):
				t.Fatalf("argv %q, want --workspace %s", argv, tc.want)
			}
			wantID := cmp.Or(tc.want, "wF") + ":"
			if !strings.HasPrefix(tab.ID, wantID) || !strings.HasPrefix(tab.Pane, wantID) {
				t.Fatalf("tab %+v, want ids in workspace %s", tab, wantID)
			}
		})
	}
}

func TestPromptSubmitsToTheAgentAndWaitsForAStateChange(t *testing.T) {
	// `agent prompt PANE TEXT --wait …`: the pane and the text as single argv elements, then a wait
	// for any first state change, capped; a blocked agent refuses it and a pane without an agent is
	// gone.
	// covers: R-SESS-025, R-SESS-150
	rt, dir := fakeRuntime(t)
	tab, err := rt.OpenTab(session.TabSpec{Label: "root"})
	if err != nil {
		t.Fatal(err)
	}
	if err = rt.Prompt(tab.Pane, "hi"); !errors.Is(err, session.ErrGone) {
		t.Fatalf("Prompt without an agent = %v, want ErrGone", err)
	}
	if err = rt.StartAgent(session.AgentSpec{Name: "p-root", Pane: tab.Pane, Kind: "claude"}); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	inner := rt.Exec
	rt.Exec = func(d, name string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return inner(d, name, args...)
	}
	text := "You are ness session s-1 (x). Read r/brief.md, then load ness-entry."
	if err = rt.Prompt(tab.Pane, text); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"agent", "prompt", tab.Pane, text, "--wait", "--until", "working", "--until", "blocked",
		"--until", "done", "--until", "idle", "--timeout", "10000",
	}
	if len(calls) != 1 || !slices.Equal(calls[0], want) {
		t.Fatalf("argv = %q, want %q", calls, want)
	}
	st, err := testkit.HerdrState(dir)
	if err != nil || len(st.Prompted) != 1 || st.Prompted[0] != [2]string{tab.Pane, text} || len(st.Typed) != 0 {
		t.Fatalf("fake state: prompted %q typed %q, %v", st.Prompted, st.Typed, err)
	}
	if err = testkit.SetHerdr(dir, "status", "p-root,blocked"); err != nil {
		t.Fatal(err)
	}
	if err = rt.Prompt(tab.Pane, "again"); !errors.Is(err, session.ErrBlocked) || errors.Is(err, session.ErrGone) ||
		!strings.Contains(err.Error(), "agent_blocked") {
		t.Fatalf("Prompt to a blocked agent = %v, want session.ErrBlocked", err)
	}
	if st, _ = testkit.HerdrState(dir); len(st.Prompted) != 1 {
		t.Fatalf("a blocked agent received a prompt: %q", st.Prompted)
	}
}

func TestPressEnterIsPaneSendKeysEnter(t *testing.T) {
	// `pane send-keys PANE Enter`, not `agent send-keys`, which the client ignored; a pane that is
	// gone is ErrGone.
	// covers: R-SESS-150
	rt, dir := fakeRuntime(t)
	tab, err := rt.OpenTab(session.TabSpec{Label: "root"})
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	inner := rt.Exec
	rt.Exec = func(d, name string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return inner(d, name, args...)
	}
	if err = rt.PressEnter(tab.Pane); err != nil {
		t.Fatal(err)
	}
	if want := []string{"pane", "send-keys", tab.Pane, "Enter"}; len(calls) != 1 || !slices.Equal(calls[0], want) {
		t.Fatalf("argv = %q, want %q", calls, want)
	}
	if st, err := testkit.HerdrState(dir); err != nil || len(st.Keys) != 1 || st.Keys[0] != [2]string{tab.Pane, "Enter"} {
		t.Fatalf("fake keys = %q, %v", st.Keys, err)
	}
	if err = rt.PressEnter("wF:p99"); !errors.Is(err, session.ErrGone) {
		t.Fatalf("PressEnter to a missing pane = %v, want ErrGone", err)
	}
}

func TestStalledPromptIsErrNotSubmitted(t *testing.T) {
	// A dropped Enter makes `agent prompt --wait` answer agent_prompt_stalled: ErrNotSubmitted (and
	// not ErrGone or ErrBlocked), the text left in the client's box and nothing submitted.
	// covers: R-SESS-150
	rt, dir := fakeRuntime(t)
	tab, err := rt.OpenTab(session.TabSpec{Label: "root"})
	if err != nil {
		t.Fatal(err)
	}
	if err = rt.StartAgent(session.AgentSpec{Name: "p-root", Pane: tab.Pane, Kind: "claude"}); err != nil {
		t.Fatal(err)
	}
	if err = testkit.SetHerdr(dir, "drop_enter", "1"); err != nil {
		t.Fatal(err)
	}
	err = rt.Prompt(tab.Pane, "hello")
	if !errors.Is(err, session.ErrNotSubmitted) || errors.Is(err, session.ErrGone) || errors.Is(err, session.ErrBlocked) ||
		!strings.Contains(err.Error(), "agent_prompt_stalled") {
		t.Fatalf("stalled Prompt = %v, want session.ErrNotSubmitted", err)
	}
	if st, err := testkit.HerdrState(dir); err != nil || len(st.Prompted) != 0 || st.Boxes[tab.Pane] != "hello" {
		t.Fatalf("fake state: prompted %q boxes %q, %v", st.Prompted, st.Boxes, err)
	}
}
