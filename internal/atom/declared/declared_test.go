package declared_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/declared"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestContract(t *testing.T) {
	// Rejects: a declared selector with confidence outside 0..1 or an empty/multi-line reason.
	testkit.PolicyContract(t, declared.Policy{Kinds: config.DefaultKinds()})
}

func TestChoose(t *testing.T) {
	// Rejects: ignoring AgentType, letting AgentType beat an explicit Kind, case-folding kinds,
	// a confidence other than 1, and a reason that does not name the kind.
	p := declared.Policy{Kinds: config.DefaultKinds()}
	cases := []struct {
		name   string
		brief  session.Brief
		offset int
		kind   string
	}{
		{"named agent", session.Brief{Role: "subagent", AgentType: "ness-sweep"}, -2, "search"},
		{"named design", session.Brief{Role: "subagent", AgentType: "ness-design"}, 1, "design"},
		{"kind", session.Brief{Role: "child", Kind: "verify"}, 1, "verify"},
		{"kind beats agent", session.Brief{Role: "subagent", Kind: "design", AgentType: "ness-sweep"}, 1, "design"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := p.Choose(tc.brief)
			if err != nil {
				t.Fatal(err)
			}
			if v != (session.Verdict{Offset: tc.offset, Confidence: 1, Reason: "kind " + tc.kind}) {
				t.Fatalf("Verdict = %+v", v)
			}
		})
	}
}

func TestNoKindRefusedWithList(t *testing.T) {
	// Rejects: defaulting to offset 0 when the kind is missing or unknown, case-insensitive
	// matching ("Design"), and an error that does not list the kinds to choose from.
	p := declared.Policy{Kinds: config.DefaultKinds()}
	for _, b := range []session.Brief{
		{Role: "child"},
		{Role: "child", Kind: "Design"},
		{Role: "child", Kind: "refactor"},
		{Role: "subagent", AgentType: "general-purpose"},
	} {
		_, err := p.Choose(b)
		if !errors.Is(err, session.ErrNoKind) {
			t.Errorf("Choose(%+v) err = %v, want ErrNoKind", b, err)
			continue
		}
		for _, k := range []string{"design", "search", "verify"} {
			if !strings.Contains(err.Error(), k) {
				t.Errorf("error %q does not list %q", err, k)
			}
		}
	}
}

func TestProjectKindsHonoured(t *testing.T) {
	// Rejects: a selector that reads the built-in defaults instead of the project's table.
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"make validate\"\n" +
		"[models.kinds]\nreview = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := f.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := declared.Policy{Kinds: cfg.Models.Kinds}
	for _, b := range []session.Brief{{Kind: "review"}, {AgentType: "ness-review"}} {
		if v, err := p.Choose(b); err != nil || v.Offset != 1 {
			t.Errorf("Choose(%+v) = %+v, %v; want offset 1", b, v, err)
		}
	}
}

func TestNamedKinds(t *testing.T) {
	// Rejects: a named agent mapped to the wrong kind or an extra/missing agent.
	want := map[string]string{"ness-sweep": "search", "ness-review": "review", "ness-verify": "verify", "ness-design": "design"}
	got := declared.NamedKinds()
	if !maps.Equal(got, want) {
		t.Fatalf("NamedKinds = %v", got)
	}
	// Rejects: a shared map that a caller can corrupt for everyone.
	got["ness-sweep"] = "design"
	if declared.NamedKinds()["ness-sweep"] != "search" {
		t.Fatal("NamedKinds returned a shared map")
	}
}
