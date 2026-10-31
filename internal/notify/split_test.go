package notify_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// plain renders the raw block, counts runes, and takes limit and maxBlocks from the test.
type plain struct{ limit, maxBlocks int }

func (p plain) Render(b notify.Block) string { return b.Raw }
func (plain) Measure(s string) int           { return utf8.RuneCountInString(s) }
func (p plain) Limit() int                   { return p.limit }
func (p plain) MaxBlocks() int               { return p.maxBlocks }

func TestSplitOnBlockBoundaries(t *testing.T) {
	// Parts are whole blocks joined by blank lines, each within the limit; concatenated they
	// equal the full render; a block alone over the limit fails naming it, nothing else.
	// covers: R-NTF-011, R-NTF-012
	md := strings.Repeat("para one two three\n\n", 10) + "```\nfence line\nfence line two\n```\n\nlast"
	blocks := notify.Parse(md)
	parts, err := notify.Split(plain{limit: 60}, blocks)
	if err != nil {
		t.Fatal(err)
	}
	full := make([]string, 0, len(blocks))
	for _, b := range blocks {
		full = append(full, b.Raw)
	}
	var texts []string
	for _, p := range parts {
		if n := utf8.RuneCountInString(p.Text()); n > 60 {
			t.Errorf("part %q has %d runes", p.Text(), n)
		}
		for _, b := range p.Blocks {
			if strings.Count(b, "```")%2 != 0 {
				t.Errorf("part cuts a fence: %q", b)
			}
		}
		texts = append(texts, p.Text())
	}
	if strings.Join(texts, "\n\n") != strings.Join(full, "\n\n") {
		t.Fatalf("parts do not add up to the full render")
	}
	if len(parts) < 4 {
		t.Fatalf("only %d parts for a 60-rune limit", len(parts))
	}
	_, err = notify.Split(plain{limit: 10}, notify.Parse("short\n\n"+strings.Repeat("x", 11)))
	if !errors.Is(err, notify.ErrBlockTooLong) || !strings.Contains(err.Error(), "paragraph \"xxxxxxxxxxx\"") {
		t.Fatalf("over-long block: %v", err)
	}
}

func TestSplitHonoursMaxBlocksAndNoLimit(t *testing.T) {
	// covers: R-NTF-021, R-NTF-029
	blocks := notify.Parse(strings.Repeat("p\n\n", 51))
	parts, err := notify.Split(plain{limit: 0, maxBlocks: 50}, blocks)
	if err != nil || len(parts) != 2 || len(parts[0].Blocks) != 50 || len(parts[1].Blocks) != 1 {
		t.Fatalf("51 blocks, 50 per part: %d parts, %v", len(parts), err)
	}
	parts, err = notify.Split(plain{}, blocks)
	if err != nil || len(parts) != 1 {
		t.Fatalf("no limits: %d parts, %v", len(parts), err)
	}
	if parts, err = notify.Split(plain{}, nil); err != nil || len(parts) != 0 {
		t.Fatalf("no blocks: %v, %v", parts, err)
	}
}

func TestSplitKeepsRunesAndFencesWhole(t *testing.T) {
	// A multi-byte block is measured in runes and never cut; a fence stays in one part.
	// covers: R-NTF-011
	md := "ééééé\n\n```\nüü\n\nöö\n```\n\nçççç"
	parts, err := notify.Split(plain{limit: 14}, notify.Parse(md))
	if err != nil || len(parts) != 3 {
		t.Fatalf("parts = %d, %v", len(parts), err)
	}
	for _, p := range parts {
		if !utf8.ValidString(p.Text()) || strings.Count(p.Text(), "```")%2 != 0 {
			t.Errorf("part %q", p.Text())
		}
	}
}

func noSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func all(parts []notify.Part) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p.Text())
	}
	return out
}

func FuzzSplit(f *testing.F) {
	// The parts are the input's blocks, in order and whole (no block lost, duplicated or cut, so
	// no fence unbalanced beyond what the input had), each under the limit, each valid UTF-8 when
	// the input is.
	// covers: R-NTF-011, R-NTF-013
	f.Add("a\n\nb\n\n```\nc\n```\n", 8)
	f.Add("| a | b |\n|---|---|\n| 1 | 2 |\n", 3)
	f.Add("é\n\n```\nü\n\n\n", 5)
	f.Fuzz(func(t *testing.T, md string, limit int) {
		if limit < 0 {
			limit = -limit
		}
		blocks := notify.Parse(md)
		parts, err := notify.Split(plain{limit: limit}, blocks)
		if err != nil {
			if !errors.Is(err, notify.ErrBlockTooLong) {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		var got []string
		for _, p := range parts {
			if limit > 0 && utf8.RuneCountInString(p.Text()) > limit {
				t.Fatalf("part over the limit: %q", p.Text())
			}
			if utf8.ValidString(md) && !utf8.ValidString(p.Text()) {
				t.Fatalf("part is not UTF-8: %q", p.Text())
			}
			got = append(got, p.Blocks...)
		}
		want := make([]string, 0, len(blocks))
		for _, b := range blocks {
			want = append(want, b.Raw)
		}
		// Against the input itself, not only Parse's output: nothing but whitespace may differ.
		if a, b := noSpace(strings.Join(all(parts), "")), noSpace(md); utf8.ValidString(md) && a != b {
			t.Fatalf("parts %q do not hold the input %q", a, b)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("parts %q != blocks %q", got, want)
		}
	})
}
