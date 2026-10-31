package cli

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// goldens maps each --json command to the script that pins its field schema byte for byte.
var goldens = map[string]string{
	"status":    "status_json.txtar",
	"inbox":     "json_goldens.txtar",
	"doctor":    "doctor.txtar",
	"stats":     "stats_json.txtar",
	"probe run": "json_goldens.txtar",
	"sync":      "json_goldens.txtar",
	"pick":      "pick.txtar",
}

func jsonCommands(c *cobra.Command, path string, out *[]string) {
	for _, sub := range c.Commands() {
		p := strings.TrimSpace(path + " " + sub.Name())
		if sub.Flags().Lookup("json") != nil {
			*out = append(*out, p)
		}
		jsonCommands(sub, p, out)
	}
}

// TestEveryJSONCommandHasAGolden enforces R-CFG-037: every command with a json flag is
// exercised by json_surface.txtar and has a golden; a new one fails until both exist.
func TestEveryJSONCommandHasAGolden(t *testing.T) {
	var found []string
	jsonCommands(newRoot(Deps{}), "", &found)
	slices.Sort(found)
	var want []string
	for p := range goldens {
		want = append(want, p)
	}
	slices.Sort(want)
	if !slices.Equal(found, want) {
		t.Fatalf("--json commands = %v, want %v: add the new one to json_surface.txtar, a golden script and goldens", found, want)
	}
	surface := read(t, "json_surface.txtar")
	for _, p := range found {
		if !regexp.MustCompile(`(?m)^(exec|exits \d) ness ` + regexp.QuoteMeta(p) + `( .*)? --json$`).MatchString(surface) {
			t.Errorf("json_surface.txtar has no `ness %s ... --json` line", p)
		}
		if g := read(t, goldens[p]); !strings.Contains(g, "ness "+p) || !strings.Contains(g, "--json") {
			t.Errorf("%s does not exercise `ness %s --json`", goldens[p], p)
		}
	}
}

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/script/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
