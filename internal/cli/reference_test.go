package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
	"github.com/spf13/cobra"
)

const refDir = "../../docs/reference/cli"

func TestReferenceIsFresh(t *testing.T) {
	// The checked-in CLI reference equals fresh output, file for file; -update rewrites it.
	// covers: R-QA-047
	want := cli.Reference()
	entries, err := os.ReadDir(refDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, ok := want[e.Name()]; !ok {
			t.Errorf("docs/reference/cli/%s is not a command page any more: delete it", e.Name())
		}
	}
	for name, data := range want {
		testkit.Golden(t, filepath.Join(refDir, name), string(data))
	}
}

func TestReferenceShape(t *testing.T) {
	// Every visible command has a page with its frontmatter, defaults and resolvable links;
	// meta.json lists exactly the pages, index first; hidden commands stay out.
	ref := cli.Reference()
	var meta struct{ Pages []string }
	if err := json.Unmarshal(ref["meta.json"], &meta); err != nil {
		t.Fatal(err)
	}
	var pages []string
	for name := range ref {
		if n, ok := strings.CutSuffix(name, ".md"); ok {
			pages = append(pages, n)
		}
	}
	slices.Sort(pages)
	listed := slices.Sorted(slices.Values(meta.Pages))
	if meta.Pages[0] != "index" || !slices.Equal(listed, pages) {
		t.Errorf("meta pages %v, files %v", meta.Pages, pages)
	}
	if _, ok := ref["help.md"]; ok {
		t.Error("the hidden help command has a page")
	}
	for _, p := range []string{"worker", "worker-install", "probe-run", "integrate"} {
		if !slices.Contains(pages, p) {
			t.Errorf("no page %q", p)
		}
	}
	page := string(ref["integrate.md"])
	for _, s := range []string{
		"---\ntitle: \"ness integrate\"\ndescription: \"Fold a branch",
		"```text\nness integrate [flags]\n```",
		"| `--subject SUBJECT` |  |",
		"| `--ui string` | `auto` |",
	} {
		if !strings.Contains(page, s) {
			t.Errorf("integrate.md lacks %q", s)
		}
	}
	if !strings.Contains(string(ref["worker.md"]), "ness-\\<project>.service") {
		t.Error("a raw <tag> reaches a table cell")
	}
	link := regexp.MustCompile(`\]\(\./([^)]+)\)`)
	for name, data := range ref {
		for _, m := range link.FindAllStringSubmatch(string(data), -1) {
			if _, ok := ref[m[1]]; !ok {
				t.Errorf("%s links to missing %s", name, m[1])
			}
		}
	}
	if !strings.Contains(string(ref["worker.md"]), "](./worker-run.md)") {
		t.Error("worker.md does not link its subcommands")
	}
}

func TestReferenceEscapesProse(t *testing.T) {
	// A <word> or {name} in a long text is escaped for MDX outside code, kept inside code
	// spans and fences; a bool flag that defaults to false shows no default.
	// covers: R-QA-047
	root := &cobra.Command{Use: "tool"}
	sub := &cobra.Command{
		Use:   "run",
		Short: "Run it",
		Long:  "Writes <name>.txt and {id}; see `a <b> {c}`.\n```sh\ntool run < in.txt\n```",
		Run:   func(*cobra.Command, []string) {},
	}
	sub.Flags().Bool("dry", false, "only print")
	sub.Flags().Bool("on", true, "turn on")
	root.AddCommand(sub)
	page := string(cli.ReferenceOf(root)["run.md"])
	for _, s := range []string{
		"Writes \\<name>.txt and \\{id}; see `a <b> {c}`.\n```sh\ntool run < in.txt\n```",
		"| `--dry` |  | only print |",
		"| `--on` | `true` | turn on |",
	} {
		if !strings.Contains(page, s) {
			t.Errorf("run.md lacks %q:\n%s", s, page)
		}
	}
}
