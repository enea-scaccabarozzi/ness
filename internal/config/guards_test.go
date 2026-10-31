package config_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

const guardEntry = `
[[guards]]
tool    = "Bash"
match   = '^pytest\s*$'
message = "Run the one test you changed."
skill   = "ness-entry"
section = "Tests"
`

func TestGuardsReadBackInOrder(t *testing.T) {
	// covers: R-CFG-086
	body := minimal + guardEntry + `
[[guards]]
tool    = "Edit|Write"
match   = '\.py$'
content = 'print\('
message = "No debug prints."
skill   = "ness-entry"
section = "Tests"
`
	c, err := load(t, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []config.Guard{
		{Tool: "Bash", Match: `^pytest\s*$`, Message: "Run the one test you changed.", Skill: "ness-entry", Section: "Tests"},
		{Tool: "Edit|Write", Match: `\.py$`, Content: `print\(`, Message: "No debug prints.", Skill: "ness-entry", Section: "Tests"},
	}
	if len(c.Guards) != len(want) {
		t.Fatalf("guards = %+v, want %+v", c.Guards, want)
	}
	for i := range want {
		if c.Guards[i] != want[i] {
			t.Errorf("guards[%d] = %+v, want %+v", i, c.Guards[i], want[i])
		}
	}
}

func TestNoGuardsByDefault(t *testing.T) {
	// covers: R-CFG-086
	c, err := load(t, minimal, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Guards) != 0 {
		t.Errorf("guards = %+v, want none", c.Guards)
	}
}

func TestGuardRefusals(t *testing.T) {
	// covers: R-CFG-087
	good := func(drop, replace string) string {
		return strings.Replace(minimal+guardEntry, drop, replace, 1)
	}
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"invalid tool", good(`tool    = "Bash"`, `tool = "(Bash"`), []string{"guards[0]", "tool"}},
		{"invalid match", good(`'^pytest\s*$'`, `'[pytest'`), []string{"guards[0]", "match"}},
		{"invalid content", good(`section = "Tests"`, "section = \"Tests\"\ncontent = '(x'"), []string{"guards[0]", "content"}},
		{"missing tool", good(`tool    = "Bash"`, ``), []string{"guards[0]", "tool"}},
		{"missing match", good(`match   = '^pytest\s*$'`, ``), []string{"guards[0]", "match"}},
		{"missing message", good(`message = "Run the one test you changed."`, ``), []string{"guards[0]", "message"}},
		{"missing skill", good(`skill   = "ness-entry"`, ``), []string{"guards[0]", "skill"}},
		{"missing section", good(`section = "Tests"`, ``), []string{"guards[0]", "section"}},
		{"empty section", good(`section = "Tests"`, `section = ""`), []string{"guards[0]", "section"}},
		{"second entry named", minimal + guardEntry + guardEntry[:strings.Index(guardEntry, "skill")], []string{"guards[1]", "skill"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustFail(t, tc.body, tc.want...)
		})
	}
}

func TestGuardRefusedAtParse(t *testing.T) {
	// covers: R-CFG-087
	if _, err := config.Parse([]byte(strings.Replace(minimal+guardEntry, `"Bash"`, `"(Bash"`, 1))); err == nil {
		t.Fatal("Parse accepted an invalid guard regexp")
	}
}
