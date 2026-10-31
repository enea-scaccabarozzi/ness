package profile_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

const example = `name    = "local"
summary = "project policy"
requires_tools = ["python3"]

[slots]
"ness-entry#stop-and-ask"   = { append = "stop-and-ask.md" }
"program-roadmap#autonomy"  = { set = "autonomy-grant.md" }
"AGENTS.md#local"           = { block = "agents-local.md" }

[files]
"docs/workflow/ops-policy.md" = { seed = "ops-policy.md" }
".golangci.yml"               = { own = "golangci.yml" }

[config]
"integrate.push" = true

[features]
enable = ["contracts"]

[atoms]
integrator = "squash"
`

func exampleFS() fstest.MapFS {
	return fstest.MapFS{
		"stop-and-ask.md": {Data: []byte("ask\n")}, "autonomy-grant.md": {Data: []byte("grant\n")},
		"agents-local.md": {Data: []byte("local\n")}, "ops-policy.md": {Data: []byte("ops\n")},
		"golangci.yml": {Data: []byte("version: 2\n")},
	}
}

func TestParseExample(t *testing.T) {
	// covers: R-PROF-001, R-PROF-017
	p, err := profile.Parse("local", []byte(example), exampleFS())
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "local" || p.RequiresTools[0] != "python3" || p.Slots["program-roadmap#autonomy"].Set != "autonomy-grant.md" ||
		p.Files["docs/workflow/ops-policy.md"].Seed != "ops-policy.md" || p.Config["integrate.push"] != true ||
		p.Enable[0] != "contracts" || p.Atoms["integrator"] != "squash" {
		t.Fatalf("parsed %+v", p)
	}
}

func TestParseRejects(t *testing.T) {
	// covers: R-PROF-005, R-PROF-006, R-PROF-007, R-PROF-008, R-PROF-009, R-PROF-017, R-PROF-044
	cases := map[string]struct{ body, want string }{
		"syntax":        {"name = \n", "profile.toml:1:"},
		"name mismatch": {"name = \"b\"\n", `name "b" differs from its directory "local"`},
		"slot no hash":  {"name = \"local\"\n[slots]\n\"ness-entry\" = { append = \"stop-and-ask.md\" }\n", "<skill-or-file>#<slot>"},
		"slot two":      {"name = \"local\"\n[slots]\n\"a#b\" = { append = \"stop-and-ask.md\", set = \"stop-and-ask.md\" }\n", "exactly one"},
		"missing file":  {"name = \"local\"\n[slots]\n\"a#b\" = { append = \"missing.md\" }\n", "missing.md"},
		"copy file":     {"name = \"local\"\n[files]\n\"x\" = { copy = \"ops-policy.md\" }\n", "copy"},
		"escape":        {"name = \"local\"\n[files]\n\"../x\" = { own = \"ops-policy.md\" }\n", "clean relative path"},
		"feature":       {"name = \"local\"\n[features]\nenable = [\"x\"]\n", `"x" is not a feature`},
		"atom":          {"name = \"local\"\n[atoms]\nintegrator = \"octopus\"\n", "merge, squash, rebase, pr"},
		"seam":          {"name = \"local\"\n[atoms]\nrouter = \"x\"\n", `unknown seam "router"`},
		"config key":    {"name = \"local\"\n[config]\n\"integrate.pushh\" = true\n", "integrate.pushh"},
		"block target":  {"name = \"local\"\n[slots]\n\"x#y\" = { block = \"agents-local.md\" }\n", "block"},
	}
	for name, c := range cases {
		_, err := profile.Parse("local", []byte(c.body), exampleFS())
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "profile local") {
			t.Errorf("%s: err = %v, want it to name the profile and contain %q", name, err, c.want)
		}
	}
}

func TestLoadSources(t *testing.T) {
	// covers: R-PROF-003, R-PROF-042
	builtin := fstest.MapFS{"program/profile.toml": {Data: []byte("name = \"program\"\n")}}
	local := fstest.MapFS{
		"mine/profile.toml":    {Data: []byte("name = \"mine\"\n")},
		"program/profile.toml": {Data: []byte("name = \"program\"\n")},
	}
	src := profile.Source{Builtin: builtin, Local: local, Version: "0.1.0"}
	p, err := src.Load("mine")
	if err != nil || !p.Local || p.Version == "" || p.Version == "0.1.0" {
		t.Fatalf("local load = %+v %v", p, err)
	}
	if _, err := src.Load("program"); err == nil || !strings.Contains(err.Error(), "program") {
		t.Fatalf("shadowing a built-in accepted: %v", err)
	}
	if _, err := src.Load("nope"); err == nil || !strings.Contains(err.Error(), `unknown profile "nope"`) {
		t.Fatalf("unknown profile: %v", err)
	}
	src.Local = nil
	p, err = src.Load("program")
	if err != nil || p.Local || p.Version != "0.1.0" {
		t.Fatalf("built-in load = %+v %v", p, err)
	}
}

func TestLocalVersionIsContentHash(t *testing.T) {
	// covers: R-PROF-043
	fsys := fstest.MapFS{"mine/profile.toml": {Data: []byte("name = \"mine\"\n")}, "mine/a.md": {Data: []byte("a")}}
	src := profile.Source{Builtin: fstest.MapFS{}, Local: fsys}
	p1, _ := src.Load("mine")
	p2, _ := src.Load("mine")
	fsys["mine/a.md"] = &fstest.MapFile{Data: []byte("b")}
	p3, _ := src.Load("mine")
	if p1.Version != p2.Version || p1.Version == p3.Version {
		t.Fatalf("versions %s %s %s", p1.Version, p2.Version, p3.Version)
	}
}

func FuzzParse(f *testing.F) {
	// covers: R-PROF-005
	f.Add([]byte(example))
	f.Add([]byte("name = \"local\"\n[slots]\n\"a#b\" = 3\n"))
	f.Add([]byte("name = \"local\"\n[[checks]]\nid = \"c\"\non = \"open\"\nskill = \"s\"\nsection = \"S\"\nmessage = \"m\"\nkind = \"section-matches\"\nfile = \"a.md\"\nheading = \"H\"\nrequire = [\"(\"]\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := profile.Parse("local", data, exampleFS())
		if err != nil && !strings.Contains(err.Error(), "profile local") {
			t.Fatalf("error does not name the profile: %v", err)
		}
	})
}
