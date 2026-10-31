package profile_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

// mk builds a profile from a manifest and files; every file named in the manifest must be given.
func mk(t *testing.T, name, manifest string, files map[string]string, local bool) *profile.Profile {
	t.Helper()
	fsys := fstest.MapFS{}
	for k, v := range files {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	p, err := profile.Parse(name, []byte("name = \""+name+"\"\n"+manifest), fsys)
	if err != nil {
		t.Fatal(err)
	}
	p.Local = local
	return p
}

func none(string) bool { return false }

func resolveErr(t *testing.T, ps []*profile.Profile, defined func(string) bool, wants ...string) {
	t.Helper()
	_, err := profile.Resolve(ps, defined)
	if err == nil {
		t.Fatal("no conflict reported")
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not contain %q", err, w)
		}
	}
}

func appender(t *testing.T, name, text string, local bool) *profile.Profile {
	t.Helper()
	return mk(t, name, "[slots]\n\"ness-entry#sizes\" = { append = \"s.md\" }\n", map[string]string{"s.md": text}, local)
}

func TestListSlotOrder(t *testing.T) {
	// covers: R-PROF-012, R-PROF-020, R-PROF-030, R-PROF-038, R-SYNC-001
	a, b := appender(t, "a", "A\n", false), appender(t, "b", "B\n", false)
	for _, order := range [][]*profile.Profile{{a, b}, {b, a}} {
		r, err := profile.Resolve(order, none)
		if err != nil {
			t.Fatal(err)
		}
		parts := r.Lists["ness-entry#sizes"]
		if len(parts) != 2 || parts[0].Owner != order[0].Name || parts[1].Owner != order[1].Name {
			t.Fatalf("parts %+v for order %s,%s", parts, order[0].Name, order[1].Name)
		}
	}
}

func TestLocalAppliedLast(t *testing.T) {
	// covers: R-PROF-021
	local, a := appender(t, "local", "L\n", true), appender(t, "a", "A\n", false)
	r, err := profile.Resolve([]*profile.Profile{local, a}, none)
	if err != nil {
		t.Fatal(err)
	}
	if parts := r.Lists["ness-entry#sizes"]; parts[1].Owner != "local" || r.Profiles[1].Name != "local" {
		t.Fatalf("local not last: %+v", parts)
	}
}

func setter(t *testing.T, name string, local bool) *profile.Profile {
	t.Helper()
	return mk(t, name, "[slots]\n\"x#y\" = { set = \"v.md\" }\n", map[string]string{"v.md": name + "\n"}, local)
}

func TestSingleSlot(t *testing.T) {
	// covers: R-PROF-013, R-PROF-022, R-PROF-031, R-PROF-034
	r, err := profile.Resolve([]*profile.Profile{setter(t, "a", false)}, none)
	if err != nil || string(r.Singles["x#y"].Text) != "a\n" {
		t.Fatalf("single = %+v %v", r.Singles, err)
	}
	resolveErr(t, []*profile.Profile{setter(t, "a", false), setter(t, "b", false)}, none, "a", "b", "x#y")
	resolveErr(t, []*profile.Profile{setter(t, "b", false), setter(t, "a", false)}, none, "a", "b", "x#y")
	r, err = profile.Resolve([]*profile.Profile{setter(t, "a", false), setter(t, "b", false), setter(t, "local", true)}, none)
	if err != nil || string(r.Singles["x#y"].Text) != "local\n" {
		t.Fatalf("local override = %+v %v", r.Singles, err)
	}
}

func TestSeamConflict(t *testing.T) {
	// covers: R-PROF-032, R-PROF-023, R-PROF-034
	squash := mk(t, "a", "[atoms]\nintegrator = \"squash\"\n", nil, false)
	pr := mk(t, "local", "[atoms]\nintegrator = \"pr\"\n", nil, true)
	resolveErr(t, []*profile.Profile{squash, pr}, none, "a", "local", "integrator")
	r, err := profile.Resolve([]*profile.Profile{squash, pr}, func(k string) bool { return k == "integrate.mode" })
	if err != nil {
		t.Fatalf("ness.toml did not settle the seam: %v", err)
	}
	if _, set := r.Defaults["integrate.mode"]; set {
		t.Fatal("a key the file sets must not get a profile default")
	}
}

func TestConfigConflict(t *testing.T) {
	// covers: R-PROF-033
	a := mk(t, "a", "[config]\n\"integrate.push\" = true\n", nil, false)
	b := mk(t, "b", "[config]\n\"integrate.push\" = false\n", nil, false)
	resolveErr(t, []*profile.Profile{a, b}, none, "a", "b", "integrate.push")
	same := mk(t, "c", "[config]\n\"integrate.push\" = true\n", nil, false)
	r, err := profile.Resolve([]*profile.Profile{a, same}, none)
	if err != nil || r.Defaults["integrate.push"] != true {
		t.Fatalf("equal values = %+v %v", r.Defaults, err)
	}
}

func TestOwnedPathConflict(t *testing.T) {
	// covers: R-PROF-035, R-PROF-010
	own := func(name string, local bool) *profile.Profile {
		return mk(t, name, "[files]\n\"docs/x.md\" = { own = \"x.md\" }\n", map[string]string{"x.md": "x\n"}, local)
	}
	resolveErr(t, []*profile.Profile{own("a", false), own("local", true)}, none, "a", "local", "docs/x.md")
	r, err := profile.Resolve([]*profile.Profile{own("a", false)}, none)
	if err != nil || len(r.Files) != 1 || r.Files[0].Owner != "a" || r.Files[0].Seed {
		t.Fatalf("files = %+v %v", r.Files, err)
	}
}

func TestRelations(t *testing.T) {
	// covers: R-PROF-036, R-PROF-040
	needs := mk(t, "behavior", "requires = [\"strict\"]\n", nil, false)
	resolveErr(t, []*profile.Profile{needs}, none, "behavior", "strict")
	hates := mk(t, "a", "conflicts = [\"b\"]\n", nil, false)
	resolveErr(t, []*profile.Profile{hates, mk(t, "b", "", nil, false)}, none, "a", "b")
	resolveErr(t, []*profile.Profile{mk(t, "b", "", nil, false), mk(t, "b", "", nil, false)}, none, "twice")
}

func TestBlocksNamedByProfile(t *testing.T) {
	// covers: R-PROF-014, R-SYNC-082
	p := mk(t, "local", "[slots]\n\"AGENTS.md#rules\" = { block = \"r.md\" }\n", map[string]string{"r.md": "r\n"}, true)
	r, err := profile.Resolve([]*profile.Profile{p}, none)
	if err != nil || len(r.Blocks) != 1 || r.Blocks[0].Name != "local/rules" {
		t.Fatalf("blocks = %+v %v", r.Blocks, err)
	}
}

func sizer(t *testing.T, name, list string) *profile.Profile {
	t.Helper()
	return mk(t, name, "sizes = "+list+"\n", nil, false)
}

func TestResolveSizes(t *testing.T) {
	// covers: R-PROF-085
	r, err := profile.Resolve(nil, none)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.Sizes, " "); got != "quick full" {
		t.Fatalf("core sizes = %q, want %q", got, "quick full")
	}
	r, err = profile.Resolve([]*profile.Profile{sizer(t, "a", `["program"]`), sizer(t, "b", `["epic", "saga"]`)}, none)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.Sizes, " "); got != "quick full program epic saga" {
		t.Fatalf("sizes = %q, want core then each profile's in profile order", got)
	}
}

func TestResolveSizeConflicts(t *testing.T) {
	// covers: R-PROF-085
	resolveErr(t, []*profile.Profile{sizer(t, "a", `["program"]`), sizer(t, "b", `["program"]`)}, none,
		"program", "a", "b")
	resolveErr(t, []*profile.Profile{sizer(t, "a", `["quick"]`)}, none, "quick", "a")
	resolveErr(t, []*profile.Profile{sizer(t, "a", `["full"]`)}, none, "full", "a")
}

func TestParseRefusesABadSizeName(t *testing.T) {
	// covers: R-PROF-085
	_, err := profile.Parse("a", []byte("name = \"a\"\nsizes = [\"Big Deal\"]\n"), fstest.MapFS{})
	if err == nil || !strings.Contains(err.Error(), "sizes") {
		t.Fatalf("err = %v, want a refusal naming sizes", err)
	}
}

func TestResolveDefaultSize(t *testing.T) {
	// covers: R-PROF-085
	r, _ := profile.Resolve(nil, none)
	if got := r.DefaultSize(); got != "full" {
		t.Fatalf("core default = %q, want full", got)
	}
	r, _ = profile.Resolve([]*profile.Profile{sizer(t, "a", `["program"]`), sizer(t, "b", `["epic"]`)}, none)
	if got := r.DefaultSize(); got != "epic" {
		t.Fatalf("default = %q, want the last added size", got)
	}
}

func TestResolveRefusesACheckSizeNobodyDeclares(t *testing.T) {
	// covers: R-PROF-084
	p, err := profile.Parse("loc", []byte("name = \"loc\"\n[[checks]]\nid = \"c\"\non = \"open\"\n"+
		"skill = \"ness-entry\"\nsection = \"Communicate\"\nrun = \"true\"\nsize = [\"progam\"]\n"), fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	resolveErr(t, []*profile.Profile{p}, none, "loc", "c", "progam")
	if _, err := profile.Resolve([]*profile.Profile{sizer(t, "a", `["progam"]`), p}, none); err != nil {
		t.Fatalf("a declared size was refused: %v", err)
	}
}
