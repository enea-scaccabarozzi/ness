package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Reference renders the command tree as the CLI reference of docs/reference/cli, keyed by file
// name: one Markdown page per visible command below the root, index.md (the root and a table of
// every command) and meta.json (the Fumadocs page order). The output is deterministic: commands
// in tree order, flags sorted, no dates.
func Reference() map[string][]byte {
	root := newRoot(Deps{Version: "dev"}) // the version string never reaches a page
	root.InitDefaultVersionFlag()
	return ReferenceOf(root)
}

// ReferenceOf renders the reference pages of any command tree rooted at root, as Reference does
// for ness's own tree.
func ReferenceOf(root *cobra.Command) map[string][]byte {
	var cmds []*cobra.Command
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range available(c) {
			cmds = append(cmds, sub)
			walk(sub)
		}
	}
	walk(root)

	out := map[string][]byte{}
	pages := append(make([]string, 0, 1+len(cmds)), "index")
	var idx strings.Builder
	page(&idx, root)
	idx.WriteString("## Commands\n\n| Command | Description |\n|---|---|\n")
	for _, c := range cmds {
		fmt.Fprintf(&idx, "| [%s](./%s.md) | %s |\n", c.CommandPath(), refName(c), cell(c.Short))
		out[refName(c)+".md"] = []byte(pageText(c))
		pages = append(pages, refName(c))
	}
	out["index.md"] = []byte(idx.String())
	meta, err := json.MarshalIndent(struct {
		Title string   `json:"title"`
		Pages []string `json:"pages"`
	}{"CLI", pages}, "", "  ")
	if err != nil {
		panic(err) // a struct of strings always marshals
	}
	out["meta.json"] = append(meta, '\n')
	return out
}

// refName is the page name of c: its path below the root, joined with dashes ("worker-start").
func refName(c *cobra.Command) string {
	return strings.ReplaceAll(strings.TrimPrefix(c.CommandPath(), c.Root().Name()+" "), " ", "-")
}

func pageText(c *cobra.Command) string {
	var b strings.Builder
	page(&b, c)
	if subs := available(c); len(subs) > 0 {
		b.WriteString("## Subcommands\n\n| Command | Description |\n|---|---|\n")
		for _, s := range subs {
			fmt.Fprintf(&b, "| [%s](./%s.md) | %s |\n", s.CommandPath(), refName(s), cell(s.Short))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// page writes the frontmatter, usage, long text, examples and flags of c.
func page(b *strings.Builder, c *cobra.Command) {
	fmt.Fprintf(b, "---\ntitle: %s\ndescription: %s\n---\n\n", strconv.Quote(c.CommandPath()), strconv.Quote(c.Short))
	fmt.Fprintf(b, "## Usage\n\n```text\n%s\n```\n\n", c.UseLine())
	if c.Long != "" {
		b.WriteString(prose(strings.TrimSpace(c.Long)) + "\n\n")
	}
	if c.Example != "" {
		lines := strings.Split(strings.TrimRight(c.Example, "\n"), "\n")
		for i, l := range lines {
			lines[i] = strings.TrimPrefix(l, "  ") // cobra's help indent
		}
		fmt.Fprintf(b, "## Examples\n\n```sh\n%s\n```\n\n", strings.Join(lines, "\n"))
	}
	flags(b, "Flags", c.LocalFlags())
	flags(b, "Inherited flags", c.InheritedFlags())
}

func flags(b *strings.Builder, title string, fs *pflag.FlagSet) {
	var rows []string
	fs.VisitAll(func(f *pflag.Flag) { // VisitAll is sorted by name
		if f.Hidden || f.Name == "help" {
			return
		}
		name, usage := pflag.UnquoteUsage(f)
		flag := "--" + f.Name
		if f.Shorthand != "" {
			flag = "-" + f.Shorthand + ", " + flag
		}
		if name != "" {
			flag += " " + name
		}
		def := "`" + f.DefValue + "`"
		if f.DefValue == "" || f.DefValue == "[]" || f.Value.Type() == "bool" && f.DefValue == "false" {
			def = ""
		}
		rows = append(rows, fmt.Sprintf("| %#q | %s | %s |", flag, def, cell(usage)))
	})
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n| Flag | Default | Description |\n|---|---|---|\n%s\n\n", title, strings.Join(rows, "\n"))
}

func available(c *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	for _, s := range c.Commands() {
		if s.IsAvailableCommand() {
			out = append(out, s)
		}
	}
	return out
}

// cell makes s safe inside a GFM table cell: no pipe, no line break, no raw tag or expression.
func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ", "<", `\<`, "{", `\{`).Replace(s)
}

// prose makes Markdown text safe for MDX: outside code fences and code spans, < and { are
// escaped, so a <word> or {name} reads as text and not as a tag or an expression.
func prose(s string) string {
	esc := strings.NewReplacer("<", `\<`, "{", `\{`)
	lines := strings.Split(s, "\n")
	fenced := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		parts := strings.Split(l, "`")
		for j := 0; j < len(parts); j += 2 { // even parts are outside code spans
			parts[j] = esc.Replace(parts[j])
		}
		lines[i] = strings.Join(parts, "`")
	}
	return strings.Join(lines, "\n")
}
