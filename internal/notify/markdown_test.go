package notify_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

func construct(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "constructs", name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseBlocks(t *testing.T) {
	// Blocks are headings, paragraphs, fences, lists and tables, separated by blank lines; a
	// fence keeps its language and its raw lines (R-NTF-010 is per channel; this is the model).
	// covers: R-NTF-010
	bs := notify.Parse(construct(t, "heading") + "\n" + construct(t, "paragraph") + "\n" +
		construct(t, "fence") + "\n" + construct(t, "list") + "\n" + construct(t, "table"))
	want := []notify.Kind{notify.Heading, notify.Heading, notify.Paragraph, notify.Paragraph, notify.Fence,
		notify.List, notify.List, notify.Table}
	if len(bs) != len(want) {
		t.Fatalf("got %d blocks, want %d: %+v", len(bs), len(want), bs)
	}
	for i, k := range want {
		if bs[i].Kind != k {
			t.Errorf("block %d kind %v, want %v", i, bs[i].Kind, k)
		}
	}
	if bs[0].Level != 1 || bs[0].Text != "Title" || bs[1].Level != 2 {
		t.Errorf("headings: %+v %+v", bs[0], bs[1])
	}
	if bs[2].Text != "One line.\nSecond line of the same paragraph." {
		t.Errorf("paragraph text %q", bs[2].Text)
	}
	if bs[4].Lang != "sh" || bs[4].Text != "make validate\necho \"a & b < c\"" || bs[4].Raw != construct(t, "fence")[:len(construct(t, "fence"))-1] {
		t.Errorf("fence: %+v", bs[4])
	}
	if len(bs[5].Items) != 2 || bs[5].Items[1] != "two with `code`" || len(bs[6].Items) != 2 {
		t.Errorf("lists: %+v %+v", bs[5], bs[6])
	}
	if len(bs[7].Rows) != 3 || bs[7].Rows[0][1] != "state" || bs[7].Rows[2][1] != "RED" {
		t.Errorf("table rows: %v", bs[7].Rows)
	}
}

func TestUnterminatedFenceIsOneBlock(t *testing.T) {
	// Review Focus 3: a fence without its closing line runs to the end of the note as one block,
	// so the splitter can never cut inside it.
	// covers: R-NTF-011
	bs := notify.Parse("intro\n\n```\ncode\n\nmore code\n")
	if len(bs) != 2 || bs[1].Kind != notify.Fence || bs[1].Text != "code\n\nmore code" {
		t.Fatalf("blocks = %+v", bs)
	}
}

func TestFenceClosesOnlyOnABareFence(t *testing.T) {
	// A fence line with an info string inside a fence is content (CommonMark): only a bare
	// fence closes it.
	// covers: R-NTF-011
	bs := notify.Parse("```md\n```go\nx\n```\n\ny")
	if len(bs) != 2 || bs[0].Kind != notify.Fence || bs[0].Text != "```go\nx" || bs[1].Text != "y" {
		t.Fatalf("blocks = %+v", bs)
	}
}

func TestUnterminatedFenceTrimsTrailingBlankLines(t *testing.T) {
	// The block's Raw is its source: the blank lines after an open fence are not part of it.
	// covers: R-NTF-011
	bs := notify.Parse("```\ncode\n\n\n")
	if len(bs) != 1 || bs[0].Raw != "```\ncode" || bs[0].Text != "code" {
		t.Fatalf("blocks = %+v", bs)
	}
}

func TestHeadingKeepsTrailingHashWithoutSpace(t *testing.T) {
	// A closing # run needs a space before it (CommonMark): "Learn C#" keeps its C#.
	// covers: R-NTF-010
	for in, want := range map[string]string{"# Learn C#": "Learn C#", "## Done ##": "Done", "# a #b": "a #b", "# C# #": "C#"} {
		if bs := notify.Parse(in); len(bs) != 1 || bs[0].Text != want {
			t.Errorf("%q -> %+v, want text %q", in, bs, want)
		}
	}
}

func TestIndentedFenceIsAFence(t *testing.T) {
	// Up to three spaces still open a fence, and a blank line inside one is not a boundary.
	// covers: R-NTF-011
	bs := notify.Parse("   ```go\nx\n\ny\n   ```\n\nafter")
	if len(bs) != 2 || bs[0].Kind != notify.Fence || bs[0].Lang != "go" || bs[0].Text != "x\n\ny" {
		t.Fatalf("blocks = %+v", bs)
	}
}

func TestFenceInsideListItemStaysWhole(t *testing.T) {
	// covers: R-NTF-011
	md := "- step\n  ```sh\n  one\n\n  two\n  ```\n- next\n\ntail"
	bs := notify.Parse(md)
	if len(bs) != 2 || bs[0].Kind != notify.List || strings.Count(bs[0].Raw, "```") != 2 {
		t.Fatalf("blocks = %+v", bs)
	}
	parts, err := notify.Split(plain{limit: len(md) - 6}, bs)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range parts {
		if strings.Count(p.Text(), "```")%2 != 0 {
			t.Errorf("part cuts the fence: %q", p.Text())
		}
	}
}

func TestIntraWordUnderscoreIsText(t *testing.T) {
	// covers: R-NTF-010
	for _, s := range []string{"snake_case_name", "a snake_case_name b", "x_1_y"} {
		if got := notify.Inlines(s); len(got) != 1 || got[0].Kind != notify.Text || got[0].Text != s {
			t.Errorf("%q -> %+v", s, got)
		}
	}
	if got := notify.Inlines("a _em_ b"); len(got) != 3 || got[1].Kind != notify.Italic {
		t.Errorf("standalone underscore em: %+v", got)
	}
}

func TestInlines(t *testing.T) {
	// covers: R-NTF-010
	in := notify.Inlines("a **b** _c_ `d **x**` [e](http://u) *f*")
	kinds := make([]notify.InlineKind, 0, len(in))
	for _, i := range in {
		kinds = append(kinds, i.Kind)
	}
	want := []notify.InlineKind{notify.Text, notify.Bold, notify.Text, notify.Italic, notify.Text, notify.Code,
		notify.Text, notify.Link, notify.Text, notify.Italic}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
	if in[5].Text != "d **x**" || in[7].Text != "e" || in[7].URL != "http://u" {
		t.Errorf("code and link: %+v %+v", in[5], in[7])
	}
	if got := notify.Inlines("a * b _ c"); len(got) != 1 || got[0].Text != "a * b _ c" {
		t.Errorf("unpaired markers stay text: %+v", got)
	}
	if got := notify.Inlines(`\*a\* \_b\_ \[c\]`); len(got) != 1 || got[0].Text != "*a* _b_ [c]" {
		t.Errorf("escaped markers are text: %+v", got)
	}
}

// markers are the runes Parse and Inlines may consume as syntax: whitespace aside, the heading,
// fence, emphasis, code, link, table and list markers (digits and the dot of "1." included).
const markers = "#`*_|-+[]()<>.:0123456789"

func content(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune(markers, r) {
			return -1
		}
		return r
	}, s)
}

func FuzzParse(f *testing.F) {
	// Parse never panics and loses no content: every rune of the input that is neither
	// whitespace nor in markers appears, in order, in the blocks' Lang, Text, Items and Rows.
	// covers: R-NTF-013
	for _, n := range []string{"heading", "paragraph", "fence", "list", "table", "escaping", "link"} {
		b, err := os.ReadFile(filepath.Join("testdata", "constructs", n+".md"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(b))
	}
	f.Add("```\nopen")
	f.Add("|a|\n|-|")
	f.Add("- x\n  ```\n  y\n\n  z\n  ```\n")
	f.Fuzz(func(t *testing.T, md string) {
		var kept strings.Builder
		for _, b := range notify.Parse(md) {
			_ = notify.Inlines(b.Text)
			kept.WriteString(b.Lang + b.Text)
			for _, it := range b.Items {
				_ = notify.Inlines(it)
				kept.WriteString(it)
			}
			for _, row := range b.Rows {
				kept.WriteString(strings.Join(row, ""))
			}
		}
		// Joining pieces can fuse stray bytes of invalid UTF-8 into a rune, so compare valid text only.
		if got, want := content(kept.String()), content(md); utf8.ValidString(md) && got != want {
			t.Fatalf("content lost or changed:\n got %q\nwant %q", got, want)
		}
	})
}
