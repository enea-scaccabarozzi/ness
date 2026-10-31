package config_test

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

const minimal = `ness = "0.1"
project = "p"
base = "main"
gate = "make validate"
`

func load(t *testing.T, body string, defaults map[string]any) (*config.Config, error) {
	t.Helper()
	f, err := config.Parse([]byte(body))
	if err != nil {
		return nil, err
	}
	return f.Config(defaults)
}

func mustFail(t *testing.T, body string, wants ...string) {
	t.Helper()
	_, err := load(t, body, nil)
	if err == nil {
		t.Fatalf("no error for:\n%s", body)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not contain %q", err, w)
		}
	}
}

func TestDefaults(t *testing.T) {
	// covers: R-CFG-042, R-CFG-060, R-MOD-001, R-MOD-003
	c, err := load(t, minimal, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sessions != "docs/sessions" || c.Client != "claude" || c.Integrate.Mode != "squash" || c.Integrate.Push {
		t.Errorf("defaults: %+v", c)
	}
	if c.Models.Policy != "declared" || c.Models.RouterThreshold != 0.8 || c.Launch.ReadyTimeout.Duration != time.Minute {
		t.Errorf("model/launch defaults: %+v %+v", c.Models, c.Launch)
	}
	if c.Worker.StallMinutes != 30 || c.Worker.QuietSeconds != 60 {
		t.Errorf("worker defaults: %+v", c.Worker)
	}
	want := map[string]config.Tier{
		"standard": {Claude: "opus/medium", Codex: "gpt-6.1-sol/medium"},
		"frontier": {Claude: "fable/medium", Codex: "astra/medium"},
		"economy":  {Claude: "sonnet/medium", Codex: "gpt-6-luna/medium"},
	}
	for name, tier := range want {
		if c.Tiers[name] != tier {
			t.Errorf("tier %s = %+v, want %+v", name, c.Tiers[name], tier)
		}
	}
}

func TestTierRowEditKeepsOtherClient(t *testing.T) {
	// covers: R-MOD-003
	c, err := load(t, minimal+"[tiers.economy]\nclaude = \"haiku/low\"\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tiers["economy"] != (config.Tier{Claude: "haiku/low", Codex: "gpt-6-luna/medium"}) {
		t.Fatalf("economy = %+v", c.Tiers["economy"])
	}
}

func TestSyntaxErrorNamesLineAndColumn(t *testing.T) {
	// covers: R-CFG-003
	mustFail(t, minimal+"x = ]\n", ".ness/ness.toml:5:")
}

func TestUnknownKeyNamed(t *testing.T) {
	// covers: R-CFG-043, R-CFG-046
	mustFail(t, minimal+"[worker]\nstal_minutes = 5\n", "worker.stal_minutes")
	mustFail(t, minimal+"[notify.channels.r]\nkind = \"slack\"\ntoken = \"xoxb\"\nchannel_id = \"C1\"\n",
		"notify.channels.r.token")
}

func TestRequiredKeys(t *testing.T) {
	// covers: R-CFG-040
	for _, key := range []string{"ness", "project", "base", "gate"} {
		var kept []string
		for _, line := range strings.Split(strings.TrimSpace(minimal), "\n") {
			if !strings.HasPrefix(line, key+" ") {
				kept = append(kept, line)
			}
		}
		mustFail(t, strings.Join(kept, "\n")+"\n", key+" is required")
	}
}

func TestEnumValues(t *testing.T) {
	// covers: R-CFG-004
	mustFail(t, minimal+"client = \"gpt\"\n", "client", "claude", "codex")
	mustFail(t, minimal+"[integrate]\nmode = \"octopus\"\n", "integrate.mode", "merge, squash, rebase, pr")
	mustFail(t, minimal+"features = [\"x\"]\n", "features", "probes")
	mustFail(t, minimal+"[tiers.standard]\nclaude = \"opus\"\n", "tiers.standard.claude")
	mustFail(t, minimal+"[tiers.standard]\nclaude = \"opus/huge\"\n", "tiers.standard.claude", "medium")
}

func TestCodexNeedsExperimental(t *testing.T) {
	// covers: R-CFG-044
	mustFail(t, minimal+"client = \"codex\"\n", "experimental")
	if _, err := load(t, minimal+"client = \"codex\"\nexperimental = [\"codex\"]\n", nil); err != nil {
		t.Fatal(err)
	}
}

func TestChannels(t *testing.T) {
	// covers: R-CFG-045
	mustFail(t, minimal+"[notify.channels.r]\nkind = \"discord\"\n", "notify.channels.r.kind")
	mustFail(t, minimal+"[notify.channels.r]\nkind = \"slack\"\ntoken = \"env:T\"\n", "notify.channels.r.channel_id")
	ok := minimal + "[notify.channels.r]\nkind = \"command\"\ncommand = \"cat\"\n"
	if _, err := load(t, ok, nil); err != nil {
		t.Fatal(err)
	}
}

func TestChannelSecrets(t *testing.T) {
	// A token is a reference, never a literal; a plain key may be either; token_env names its rewrite.
	// covers: R-CFG-080, R-CFG-081, R-CFG-084
	tg := minimal + "[notify.channels.p]\nkind = \"telegram\"\n"
	mustFail(t, tg+"token = \"123:abc\"\nchat_id = \"1\"\n", "notify.channels.p.token", "secret:NAME")
	mustFail(t, tg+"token_env = \"TG\"\nchat_id = \"1\"\n", "token_env", `token = "env:TG"`)
	mustFail(t, tg+"token = \"env:T\"\nchat_id = \"secret:bad name\"\n", "notify.channels.p.chat_id")
	for _, chat := range []string{"1", "secret:chat", "env:CHAT"} {
		if _, err := load(t, tg+"token = \"secret:tg\"\nchat_id = \""+chat+"\"\n", nil); err != nil {
			t.Fatalf("chat_id = %q: %v", chat, err)
		}
	}
}

func TestOperatorChannel(t *testing.T) {
	// notify.operator names a configured channel and needs the notify feature (CX-16).
	// covers: R-CFG-055
	ch := "[notify.channels.phone]\nkind = \"telegram\"\ntoken = \"env:T\"\nchat_id = \"1\"\n"
	mustFail(t, minimal+"features = [\"notify\"]\n[notify]\noperator = \"nope\"\n"+ch, "notify.operator", "phone")
	mustFail(t, minimal+"[notify]\noperator = \"phone\"\n"+ch, "notify.operator", "notify")
	gh := "[notify.channels.gh]\nkind = \"github\"\ntoken = \"env:T\"\nrepo = \"o/r\"\n"
	mustFail(t, minimal+"features = [\"notify\"]\n[notify]\noperator = \"gh\"\n"+gh, "notify.operator", "thread")
	c, err := load(t, minimal+"features = [\"notify\"]\n[notify]\noperator = \"phone\"\n"+ch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Notify.Operator != "phone" {
		t.Fatalf("operator = %q", c.Notify.Operator)
	}
}

func TestProfileDefaultsLoseToFile(t *testing.T) {
	// covers: R-PROF-008
	c, err := load(t, minimal+"[integrate]\npush = false\n",
		map[string]any{"integrate.push": true, "integrate.mode": "squash"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Integrate.Push || c.Integrate.Mode != "squash" {
		t.Fatalf("integrate = %+v", c.Integrate)
	}
	if err := config.ValidKey("integrate.pushh", true); err == nil {
		t.Fatal("unknown key accepted")
	}
	if err := config.ValidKey("worker.stall_minutes", "ten"); err == nil {
		t.Fatal("wrong type accepted")
	}
}

func TestWrongTypeNamesKey(t *testing.T) {
	// covers: R-CFG-004
	mustFail(t, minimal+"client = 5\n", "client", "claude, codex")
	mustFail(t, minimal+"[worker]\nstall_minutes = \"x\"\n", "worker.stall_minutes", "integer")
}

func TestValidKeyChecksValue(t *testing.T) {
	// covers: R-PROF-008
	if err := config.ValidKey("integrate.mode", "octopus"); err == nil || !strings.Contains(err.Error(), "integrate.mode") {
		t.Fatalf("err = %v", err)
	}
	if err := config.ValidKey("integrate.mode", "squash"); err != nil {
		t.Fatal(err)
	}
}

func FuzzParse(f *testing.F) {
	// covers: R-CFG-005
	f.Add([]byte(minimal))
	f.Add([]byte(minimal + "[tiers.standard]\nclaude = \"opus/medium\"\n[notify.channels.a]\nkind = \"slack\"\n"))
	f.Add([]byte("ness = \n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := config.Parse(data)
		if err != nil {
			return
		}
		_, _ = file.Config(map[string]any{"integrate.push": true})
	})
}

// Rejects an implementation that does not read [integrate] setup, or defaults it to non-empty.
func TestIntegrateSetupLoads(t *testing.T) {
	// covers: R-CFG-061
	c, err := load(t, minimal+"[integrate]\nsetup = \"just data-link\"\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Integrate.Setup != "just data-link" {
		t.Errorf("Setup = %q", c.Integrate.Setup)
	}
	if d, err := load(t, minimal, nil); err != nil || d.Integrate.Setup != "" {
		t.Errorf("default Setup = %q, %v", d.Integrate.Setup, err)
	}
}

func TestDefaultLadder(t *testing.T) {
	// Rejects: the old three-row table, a rung with a swapped model/effort, a ladder in table
	// (map) order instead of cheapest first, the old "text" default, a zero command timeout.
	// covers: R-MOD-001, R-MOD-005, R-MOD-006, R-MOD-008
	want := map[string]config.Tier{
		"bulk":     {Claude: "haiku/medium", Codex: "gpt-6-luna/low"},
		"economy":  {Claude: "sonnet/medium", Codex: "gpt-6-luna/medium"},
		"lean":     {Claude: "opus/low", Codex: "gpt-6.1-sol/low"},
		"standard": {Claude: "opus/medium", Codex: "gpt-6.1-sol/medium"},
		"deep":     {Claude: "opus/high", Codex: "gpt-6.1-sol/high"},
		"frontier": {Claude: "fable/medium", Codex: "astra/medium"},
		"apex":     {Claude: "fable/high", Codex: "astra/high"},
	}
	got := config.DefaultTiers()
	if len(got) != len(want) {
		t.Errorf("DefaultTiers has %d rows, want %d: %v", len(got), len(want), got)
	}
	for name, tier := range want {
		if got[name] != tier {
			t.Errorf("DefaultTiers[%s] = %+v, want %+v", name, got[name], tier)
		}
	}
	order := []string{"bulk", "economy", "lean", "standard", "deep", "frontier", "apex"}
	if !slices.Equal(config.DefaultLadder(), order) {
		t.Errorf("DefaultLadder = %v, want %v", config.DefaultLadder(), order)
	}
	kinds := map[string]int{
		"search": -2, "run": -2, "implement": 0, "debug": 0, "research": 1, "review": 1, "verify": 1, "design": 1,
	}
	if !maps.Equal(config.DefaultKinds(), kinds) {
		t.Errorf("DefaultKinds = %v, want %v", config.DefaultKinds(), kinds)
	}
	c, err := load(t, minimal, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := c.Models
	if !slices.Equal(m.Ladder, order) || m.Base != "standard" || m.Ceiling != "" || m.Policy != "declared" ||
		!maps.Equal(m.Kinds, kinds) || m.CommandTimeout.Duration != 10*time.Second {
		t.Errorf("models defaults: %+v", m)
	}
	for name, tier := range want {
		if c.Tiers[name] != tier {
			t.Errorf("loaded tier %s = %+v, want %+v", name, c.Tiers[name], tier)
		}
	}
}

func TestLadderValidation(t *testing.T) {
	// Rejects: a loader that accepts a ladder/base/ceiling naming no rung, a ceiling below base,
	// a command policy with nothing to run, a non-positive timeout, a silent "text" alias, a
	// kinds table that replaces the defaults instead of merging, and one that refuses a
	// project's own reordered or shortened ladder.
	// covers: R-MOD-005, R-MOD-006, R-MOD-007, R-MOD-008
	refused := []struct {
		name, body string
		wants      []string
	}{
		{"rung not in tiers", "[models]\nladder = [\"economy\", \"standard\", \"ultra\"]\n", []string{"models.ladder", "ultra"}},
		{"duplicate rung", "[models]\nladder = [\"economy\", \"standard\", \"economy\"]\n",
			[]string{"models.ladder", "economy", "twice"}},
		{"base not on ladder", "[models]\nbase = \"ultra\"\n", []string{"models.base"}},
		{"base dropped from ladder", "[models]\nladder = [\"economy\", \"frontier\"]\n", []string{"models.base"}},
		{"ceiling not on ladder", "[models]\nceiling = \"ultra\"\n", []string{"models.ceiling"}},
		{"ceiling below base", "[models]\nceiling = \"economy\"\n", []string{"models.ceiling"}},
		{"command without command", "[models]\npolicy = \"command\"\n", []string{"models.command"}},
		{"zero timeout", "[models]\npolicy = \"command\"\ncommand = \"./pick\"\ncommand_timeout = \"0s\"\n",
			[]string{"models.command_timeout"}},
		{"text policy", "[models]\npolicy = \"text\"\n",
			[]string{`models.policy "text" was replaced by "declared" (ladder and kinds; see ness pick)`}},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) { mustFail(t, minimal+tc.body, tc.wants...) })
	}
	accepted := []struct{ name, body string }{
		{"own short ladder", "[models]\nladder = [\"economy\", \"standard\", \"deep\"]\nceiling = \"deep\"\n"},
		{"ceiling at base", "[models]\nceiling = \"standard\"\n"},
		{"own rung", "[tiers.mini]\nclaude = \"haiku/low\"\ncodex = \"gpt-6-luna/low\"\n" +
			"[models]\nladder = [\"mini\", \"standard\"]\n"},
		{"command", "[models]\npolicy = \"command\"\ncommand = \"./pick\"\ncommand_timeout = \"3s\"\n"},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := load(t, minimal+tc.body, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
	c, err := load(t, minimal+"[models.kinds]\nreview = 2\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := config.DefaultKinds()
	want["review"] = 2
	if !maps.Equal(c.Models.Kinds, want) {
		t.Errorf("kinds = %v, want %v", c.Models.Kinds, want)
	}
}

func TestJevConfig(t *testing.T) {
	// Rejects: missing defaults (the selector would post to "" with no model), a floating alias as
	// the default model, a literal key accepted into ness.toml (R-CFG-081), a malformed reference
	// name, a non-positive timeout, and a loader that still refuses policy = "jev".
	c, err := load(t, minimal, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := config.Jev{Model: "jev-1.14.0", Key: "env:TYPESAFE_API_KEY", Endpoint: "https://api.typesafe.ai",
		Timeout: config.Duration{Duration: 5 * time.Second}}
	if c.Models.Jev != want {
		t.Errorf("models.jev = %+v, want %+v", c.Models.Jev, want)
	}
	for _, body := range []string{
		"[models]\npolicy = \"jev\"\n",
		"[models]\npolicy = \"jev\"\n[models.jev]\nkey = \"secret:jev\"\ntimeout = \"2s\"\n",
		"[models.jev]\nkey = \"env:JEV_KEY\"\nmodel = \"jev-1.13.0\"\nendpoint = \"http://127.0.0.1:9\"\n",
	} {
		if _, err := load(t, minimal+body, nil); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
	mustFail(t, minimal+"[models.jev]\nkey = \"tsk-literal-key\"\n", "models.jev.key", "secret:NAME", "R-CFG-081")
	mustFail(t, minimal+"[models.jev]\nkey = \"secret:bad name\"\n", "models.jev.key")
	mustFail(t, minimal+"[models.jev]\ntimeout = \"0s\"\n", "models.jev.timeout")
	// Ruling: aliases move and change answers, so a calibrated selector must name a pinned version.
	for _, alias := range []string{"jev-latest", "jev-preview"} {
		mustFail(t, minimal+"[models.jev]\nmodel = \""+alias+"\"\n", "models.jev.model", alias)
	}
}

func TestJevLimits(t *testing.T) {
	// covers: R-MOD-069
	// Rejects: a timeout the pre-tool hook cannot wait for (only "> 0" checked), and an endpoint
	// that sends the key in clear or to a lookalike host (only a scheme prefix or "127." checked).
	for _, ep := range []string{"https://jev.example", "http://127.0.0.1:9", "http://127.8.0.1", "http://[::1]:8080", "http://localhost:1"} {
		if _, err := load(t, minimal+"[models.jev]\nendpoint = \""+ep+"\"\ntimeout = \"10s\"\n", nil); err != nil {
			t.Errorf("endpoint %s, timeout 10s: %v", ep, err)
		}
	}
	mustFail(t, minimal+"[models.jev]\ntimeout = \"11s\"\n", "models.jev.timeout")
	for _, ep := range []string{"http://api.typesafe.ai", "http://127.0.0.1.evil.example", "http://localhost.evil.example",
		"ftp://127.0.0.1", "api.typesafe.ai", ""} {
		mustFail(t, minimal+"[models.jev]\nendpoint = \""+ep+"\"\n", "models.jev.endpoint")
	}
}
