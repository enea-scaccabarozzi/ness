package sync_test

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/profiles"
)

func testConfig(t *testing.T, extra string) *config.Config {
	t.Helper()
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"true\"\n" + extra))
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func openRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func render(t *testing.T, root *os.Root, extra string, ps ...*profile.Profile) ([]nsync.Entry, error) {
	t.Helper()
	core, err := profile.LoadCore(profiles.Core(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	res, err := profile.Resolve(append([]*profile.Profile{core}, ps...), func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	return nsync.Render(res, testConfig(t, extra), root, profiles.Features())
}

func find(entries []nsync.Entry, id string) *nsync.Entry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}

func TestRenderCore(t *testing.T) {
	// covers: R-SYNC-060, R-SYNC-061, R-SYNC-066, R-SYNC-067, R-SYNC-069, R-SYNC-084, R-SYNC-085, R-MOD-020
	entries, err := render(t, openRoot(t), "")
	if err != nil {
		t.Fatal(err)
	}
	router := find(entries, "AGENTS.md#core/router")
	if router == nil || !strings.HasPrefix(string(router.Content),
		"## Sessions (ness)\nApplies only when the `ness` command exists in this checkout. Otherwise ignore this block.\n") {
		t.Fatalf("router block: %+v", router)
	}
	var keys []string
	for _, e := range entries {
		if e.Path == ".claude/settings.json" {
			keys = append(keys, e.Name)
		}
	}
	wantKeys := []string{
		"autoCompactWindow", "env.CLAUDE_CODE_SUBAGENT_MODEL",
		"hooks.PreToolUse[ness hook pre-tool]", "hooks.SessionStart[ness hook session-start]",
	}
	slices.Sort(keys)
	if !slices.Equal(keys, wantKeys) {
		t.Fatalf("settings keys %q", keys)
	}
	if e := find(entries, ".claude/settings.json:autoCompactWindow"); string(e.Content) != "265000" {
		t.Fatalf("autoCompactWindow = %s", e.Content)
	}
	if e := find(entries, ".claude/settings.json:env.CLAUDE_CODE_SUBAGENT_MODEL"); string(e.Content) != `"sonnet"` {
		t.Fatalf("subagent model = %s", e.Content)
	}
	hook := find(entries, ".claude/settings.json:hooks.SessionStart[ness hook session-start]")
	if !strings.Contains(string(hook.Content), `"command -v ness >/dev/null || exit 0; ness hook session-start"`) {
		t.Fatalf("hook entry %s", hook.Content)
	}
	if e := find(entries, ".gitignore#core/ignore"); string(e.Content) != ".worktrees/\n*.ness-new\n" {
		t.Fatalf("gitignore block %q", e.Content)
	}
	checkAgents(t, entries, map[string]string{
		"ness-sweep": "sonnet/medium", "ness-review": "opus/high",
		"ness-verify": "opus/high", "ness-design": "opus/high",
	})
}

// checkAgents fails unless each named agent's frontmatter carries the model/effort in want.
func checkAgents(t *testing.T, entries []nsync.Entry, want map[string]string) {
	t.Helper()
	for name, me := range want {
		model, effort, _ := strings.Cut(me, "/")
		e := find(entries, ".claude/agents/"+name+".md")
		head := "---\nname: " + name + "\n"
		if e == nil || e.Owner != "core" || !strings.HasPrefix(string(e.Content), head) ||
			!strings.Contains(string(e.Content), "\nmodel: "+model+"\neffort: "+effort+"\n---\n") {
			t.Errorf("subagent %s, want %s: %q", name, me, content(e))
		}
	}
}

// content is e's content, or "<missing>".
func content(e *nsync.Entry) string {
	if e == nil {
		return "<missing>"
	}
	return string(e.Content)
}

func TestNamedAgentsFollowBase(t *testing.T) {
	// covers: R-MOD-020, R-MOD-050
	// Rejects: agents and subagent defaults pinned to fixed tier names instead of base+kind
	// offset clamped to the ceiling (each case moves base or ceiling and expects the agents to move).
	cases := []struct {
		name, extra, subagent string
		agents                map[string]string
	}{
		{"base economy", "[models]\nbase = \"economy\"\n", `"haiku"`, map[string]string{
			"ness-sweep": "haiku/medium", "ness-review": "opus/low",
			"ness-verify": "opus/low", "ness-design": "opus/low",
		}},
		{"ceiling deep", "[models]\nceiling = \"deep\"\n", `"sonnet"`, map[string]string{
			"ness-sweep": "sonnet/medium", "ness-verify": "opus/high", "ness-design": "opus/high",
		}},
		{"project kinds", "[models.kinds]\nsearch = -1\n", `"opus"`, map[string]string{
			"ness-sweep": "opus/low", "ness-review": "opus/high",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := render(t, openRoot(t), tc.extra)
			if err != nil {
				t.Fatal(err)
			}
			if e := find(entries, ".claude/settings.json:env.CLAUDE_CODE_SUBAGENT_MODEL"); e == nil ||
				string(e.Content) != tc.subagent {
				t.Errorf("subagent model = %q, want %s", content(e), tc.subagent)
			}
			checkAgents(t, entries, tc.agents)
		})
	}
}

func TestRenderClaudeMD(t *testing.T) {
	// covers: R-SYNC-062, R-SYNC-063, R-SYNC-065
	root := openRoot(t)
	entries, err := render(t, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if e := find(entries, "CLAUDE.md"); e == nil || e.Kind != nsync.KindLink || string(e.Content) != "AGENTS.md" {
		t.Fatalf("CLAUDE.md entry %+v", e)
	}
	if e := find(entries, ".claude/skills"); e == nil || e.Kind != nsync.KindLink || string(e.Content) != "../.agents/skills" {
		t.Fatalf(".claude/skills entry %+v", e)
	}
	if err := root.WriteFile("CLAUDE.md", []byte("# mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err = render(t, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if e := find(entries, "CLAUDE.md#core/import"); e == nil || string(e.Content) != "@AGENTS.md\n" {
		t.Fatalf("import block %+v", e)
	}
}

func local(t *testing.T, manifest string, files map[string]string) *profile.Profile {
	t.Helper()
	fsys := fstest.MapFS{}
	for k, v := range files {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	p, err := profile.Parse("local", []byte("name = \"local\"\n"+manifest), fsys)
	if err != nil {
		t.Fatal(err)
	}
	p.Local = true
	return p
}

func TestRenderSlots(t *testing.T) {
	// covers: R-PROF-041, R-PROF-012
	p := local(t, "[slots]\n\"ness-entry#stop-and-ask\" = { append = \"a.md\" }\n", map[string]string{"a.md": "Ask before deploys.\n"})
	entries, err := render(t, openRoot(t), "", p)
	if err != nil {
		t.Fatal(err)
	}
	skill := string(find(entries, ".agents/skills/ness-entry/SKILL.md").Content)
	if !strings.Contains(skill, "anything you cannot undo.\nAsk before deploys.\n\n## Reserved decisions") ||
		strings.Contains(skill, "ness:slot") {
		t.Fatalf("rendered skill:\n%s", skill)
	}
	for _, bad := range []string{
		"[slots]\n\"ness-entry#nope\" = { append = \"a.md\" }\n",
		"[slots]\n\"ness-entry#sizes\" = { set = \"a.md\" }\n",
		"[slots]\n\"ness-nothing#sizes\" = { append = \"a.md\" }\n",
	} {
		if _, err := render(t, openRoot(t), "", local(t, bad, map[string]string{"a.md": "x\n"})); err == nil ||
			!strings.Contains(err.Error(), "local") {
			t.Errorf("slot %q: err = %v", bad, err)
		}
	}
}

func TestRenderRejectsDoubleTargets(t *testing.T) {
	p := local(t, "[files]\n\".claude/agents/ness-sweep.md\" = { own = \"a.md\" }\n", map[string]string{"a.md": "x\n"})
	if _, err := render(t, openRoot(t), "", p); err == nil || !strings.Contains(err.Error(), ".claude/agents/ness-sweep.md") {
		t.Fatalf("err = %v", err)
	}
}

func TestRenderAgentsBlocksInProfileOrder(t *testing.T) {
	// covers: R-SYNC-001, R-SYNC-002
	fsys := fstest.MapFS{"r.md": &fstest.MapFile{Data: []byte("## Rules\nBe kind.\n")}}
	p, err := profile.Parse("aaa", []byte("name = \"aaa\"\n[slots]\n\"AGENTS.md#rules\" = { block = \"r.md\" }\n"), fsys)
	if err != nil {
		t.Fatal(err)
	}
	p.Local = true
	var agents []string
	for range 2 {
		root := openRoot(t)
		entries, err := render(t, root, "", p)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := nsync.Compute(root, entries, nsync.LockMeta{Ness: "0.1.0"})
		if err != nil {
			t.Fatal(err)
		}
		if err := nsync.Apply(root, plan); err != nil {
			t.Fatal(err)
		}
		b, err := root.ReadFile("AGENTS.md")
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, string(b))
	}
	core, aaa := strings.Index(agents[0], "ness:begin core/router"), strings.Index(agents[0], "ness:begin aaa/rules")
	if core < 0 || aaa < 0 || core > aaa {
		t.Fatalf("core/router must come before aaa/rules:\n%s", agents[0])
	}
	if agents[0] != agents[1] {
		t.Fatalf("two syncs of the same inputs differ:\n%s\n---\n%s", agents[0], agents[1])
	}
}

func TestCodexEntriesOnlyForCodex(t *testing.T) {
	// covers: R-SYNC-068, R-SYNC-086, R-MOD-050
	root := openRoot(t)
	entries, err := render(t, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if find(entries, ".codex/config.toml:agents.default_subagent_model") != nil {
		t.Fatal("codex entries rendered for the claude client")
	}
	entries, err = render(t, root, "client = \"codex\"\nexperimental = [\"codex\"]\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		".codex/config.toml:model_auto_compact_token_limit":           "235000",
		".codex/config.toml:agents.default_subagent_model":            `"gpt-6-luna"`,
		".codex/config.toml:agents.default_subagent_reasoning_effort": `"medium"`,
	}
	for id, content := range want {
		e := find(entries, id)
		if e == nil || string(e.Content) != content || e.Kind != nsync.KindKey {
			t.Fatalf("%s = %+v, want %q", id, e, content)
		}
	}
	for _, e := range entries {
		if e.Path == ".codex/config.toml" && want[e.ID] == "" {
			t.Fatalf("unexpected codex entry %s", e.ID)
		}
	}
	// Rejects: codex defaults read from the economy row instead of base-2: base economy is bulk.
	entries, err = render(t, root, "client = \"codex\"\nexperimental = [\"codex\"]\n[models]\nbase = \"economy\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if e := find(entries, ".codex/config.toml:agents.default_subagent_reasoning_effort"); e == nil ||
		string(e.Content) != `"low"` {
		t.Fatalf("codex subagent effort at base economy = %q, want \"low\"", content(e))
	}
}

func TestFeatureSkillsRenderOnlyWhenOn(t *testing.T) {
	// A feature brings its own skill text (design §2); it is a target only while the feature is
	// on, owned by feature/<name>, so sync removes it when the feature goes off.
	// covers: R-PROF-015
	entries, err := render(t, openRoot(t), "features = [\"probes\"]\n")
	if err != nil {
		t.Fatal(err)
	}
	e := find(entries, ".agents/skills/ness-probes/SKILL.md")
	if e == nil || e.Kind != nsync.KindFile || e.Owner != "feature/probes" ||
		!bytes.Contains(e.Content, []byte("ness probe start")) {
		t.Fatalf("probes skill = %+v; want a feature/probes file naming ness probe start", e)
	}
	for _, extra := range []string{".agents/skills/ness-monitor/SKILL.md", ".agents/skills/ness-monitor/references/example-prod-week.md"} {
		if e := find(entries, extra); e == nil || e.Owner != "feature/probes" {
			t.Fatalf("%s = %+v; want a feature/probes file", extra, e)
		}
	}
	for _, off := range []string{".agents/skills/ness-notify/SKILL.md", ".agents/skills/ness-contracts/SKILL.md"} {
		if find(entries, off) != nil {
			t.Fatalf("%s rendered while its feature is off", off)
		}
	}
	none, err := render(t, openRoot(t), "")
	if err != nil {
		t.Fatal(err)
	}
	if find(none, ".agents/skills/ness-probes/SKILL.md") != nil || find(none, ".agents/skills/ness-monitor/SKILL.md") != nil {
		t.Fatal("probes skills rendered with no feature on")
	}
}
