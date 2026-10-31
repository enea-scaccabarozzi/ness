package codex_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/enea-scaccabarozzi/ness/internal/atom/codex"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func fakeClient(t *testing.T) (*codex.Client, string) {
	t.Helper()
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	return &codex.Client{Exec: testkit.FakeExec{Dir: dir}.Run}, dir
}

func TestClientContractOverFake(t *testing.T) {
	// covers: R-SESS-111, R-QA-024
	c, _ := fakeClient(t)
	testkit.ClientContract(t, c)
}

func TestArgsCarryTrustUpdateAndHooks(t *testing.T) {
	// covers: R-SESS-113, R-HOOK-018, R-MOD-010
	c, _ := fakeClient(t)
	got := c.Args(session.LaunchSpec{Dir: "/w/t", Model: "gpt-6-luna", Effort: "low"})
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"-m gpt-6-luna", `-c model_reasoning_effort="low"`, `-c projects={"/w/t"={trust_level="trusted"}}`,
		"-c check_for_update_on_startup=false", "--enable hooks", "--dangerously-bypass-hook-trust",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q lacks %q", joined, want)
		}
	}
	if strings.Contains(joined, "load ness-entry") || strings.HasPrefix(got[len(got)-1], "You are") {
		t.Fatalf("the prompt is in the argv: %q", got) // the launch submits it (R-SESS-025)
	}
	for i, a := range got {
		if a == "-c" && strings.HasPrefix(got[i+1], "hooks.") && !strings.HasPrefix(got[i+1], "hooks.SessionStart=") && !strings.HasPrefix(got[i+1], "hooks.PreToolUse=") {
			t.Fatalf("unexpected hook key %q", got[i+1])
		}
	}
}

func TestModelsFromDebugModels(t *testing.T) {
	// covers: R-MOD-030, R-MOD-032, R-MOD-033
	c, dir := fakeClient(t)
	if err := testkit.SetFake(dir, "output", "codex debug models", `{"models":[{"slug":"gpt-6-luna"}]}`); err != nil {
		t.Fatal(err)
	}
	if err := c.ResolveModel("gpt-6-luna"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"astra", "gpt-6.1-sol", "best", "opus"} {
		if err := c.ResolveModel(bad); err == nil || !strings.Contains(err.Error(), "codex debug models") ||
			!strings.Contains(err.Error(), "upgrade Codex") || !strings.Contains(err.Error(), "[tiers.") {
			t.Fatalf("ResolveModel(%q) = %v", bad, err)
		}
	}
	if !slices.Equal(c.Efforts(), []string{"minimal", "low", "medium", "high", "xhigh"}) || slices.Contains(c.Efforts(), "max") {
		t.Fatalf("Efforts = %v", c.Efforts())
	}
	if c.NewSessionID() != "" || c.CurrentSession() != "" {
		t.Fatal("codex has no session id flag; the atom must not invent one")
	}
}

// hookCommands decodes every hooks.* -c value as TOML, as codex does, and returns event -> command.
func hookCommands(t *testing.T, args []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for i, a := range args {
		if a != "-c" || !strings.HasPrefix(args[i+1], "hooks.") {
			continue
		}
		key, val, _ := strings.Cut(args[i+1], "=")
		var doc struct {
			V []struct {
				Hooks []struct{ Type, Command string }
			}
		}
		if _, err := toml.Decode("v = "+val, &doc); err != nil {
			t.Fatalf("-c %s is not valid TOML: %v", args[i+1], err)
		}
		if len(doc.V) != 1 || len(doc.V[0].Hooks) != 1 || doc.V[0].Hooks[0].Type != "command" {
			t.Fatalf("decoded %+v", doc)
		}
		out[strings.TrimPrefix(key, "hooks.")] = doc.V[0].Hooks[0].Command
	}
	return out
}

func TestHooksDecodeAsTOML(t *testing.T) {
	// covers: R-HOOK-018
	spec := session.LaunchSpec{Dir: "/w", Model: "m", Effort: "low"}
	c, _ := fakeClient(t)
	for ness, want := range map[string]string{
		"":              "ness",
		"/usr/bin/ness": "/usr/bin/ness",
		"/my dir/ness":  "'/my dir/ness'",
		"/it's/ness":    `'/it'\''s/ness'`,
	} {
		spec.Ness = ness
		got := hookCommands(t, c.Args(spec))
		if got["SessionStart"] != want+" hook session-start" || got["PreToolUse"] != want+" hook pre-tool" {
			t.Fatalf("ness %q: %v", ness, got)
		}
	}
	// the old JSON form is not TOML and must not decode
	var doc map[string]any
	if _, err := toml.Decode(`v = [{"hooks":[{"type":"command","command":"ness hook x"}]}]`, &doc); err == nil {
		t.Fatal("JSON decoded as TOML; the test would not catch the old bug")
	}
}

func TestTrustKeyEscapesControlCharacters(t *testing.T) {
	// covers: R-SESS-113
	c, _ := fakeClient(t)
	// The key has no dot for Codex's -c splitter; the value decodes as TOML for a path with dots
	// (every worktree path), quotes, backslashes and control characters.
	for _, dir := range []string{"/w/a\"b\\c\nd\x01 e", "/home/u/p/.worktrees/x-0123456789ab", "/w/a.b.c"} {
		got := c.Args(session.LaunchSpec{Dir: dir, Model: "m", Effort: "low"})
		i := slices.IndexFunc(got, func(a string) bool { return strings.HasPrefix(a, "projects") })
		key, val, _ := strings.Cut(got[i], "=")
		var doc struct {
			V map[string]struct{ Trust_level string }
		}
		if _, err := toml.Decode("v = "+val, &doc); key != "projects" || err != nil || len(doc.V) != 1 ||
			doc.V[dir].Trust_level != "trusted" {
			t.Fatalf("%q: key %q, value decoded to %+v, %v", got[i], key, doc, err)
		}
	}
}
