// Package notify is the notify feature (design §9): one Markdown note rendered by a channel
// atom, split on block boundaries, posted in a thread by key, each part recorded.
package notify

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind is a block's kind.
type Kind int

// The block kinds: everything a renderer treats differently.
const (
	Paragraph Kind = iota
	Heading
	Fence
	List
	Table
)

// Block is one block of the note. Raw is its source (no trailing newline), Text the heading or
// paragraph text with inline markup kept (the fence's body for a Fence), Items a list's items,
// Rows a table's cells with the header first.
type Block struct {
	Kind  Kind
	Level int
	Lang  string
	Text  string
	Items []string
	Rows  [][]string
	Raw   string
}

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)(?:\s+#+)?\s*$`)
	itemRe    = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*)$`)
	tableSep  = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)
)

// Parse splits md into blocks. It never panics (R-NTF-013): every input is some sequence of
// paragraphs. An unterminated fence runs to the end.
func Parse(md string) []Block {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []Block
	for i := 0; i < len(lines); {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
			i++
		case opensFence(line):
			var b Block
			b, i = fence(lines, i)
			out = append(out, b)
		case headingRe.MatchString(line):
			m := headingRe.FindStringSubmatch(line)
			out = append(out, Block{Kind: Heading, Level: len(m[1]), Text: m[2], Raw: line})
			i++
		default:
			var b Block
			b, i = span(lines, i)
			out = append(out, b)
		}
	}
	return out
}

// fenceLine reports whether line, ignoring its indent, starts with a code fence marker.
func fenceLine(line string) bool { return strings.HasPrefix(strings.TrimSpace(line), "```") }

// closesFence reports whether line closes a fence: the marker alone, no info string (CommonMark).
func closesFence(line string) bool {
	return fenceLine(line) && strings.Trim(strings.TrimSpace(line), "`") == ""
}

// opensFence reports whether line opens a top-level fence: up to three spaces, then the marker.
func opensFence(line string) bool {
	return fenceLine(line) && len(line)-len(strings.TrimLeft(line, " ")) <= 3
}

// fence reads from the opening line to the closing line or the end. A blank line inside it is
// part of it.
func fence(lines []string, i int) (b Block, next int) {
	lang := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "```"))
	j := i + 1
	for j < len(lines) && !closesFence(lines[j]) {
		j++
	}
	var end int
	if j < len(lines) {
		end = j + 1
	} else {
		// Unterminated: the fence keeps its blank lines inside but not the ones that trail it.
		j = trimBlank(lines, i+1, j)
		end = j
	}
	body := strings.Join(lines[i+1:j], "\n")
	return Block{Kind: Fence, Lang: lang, Text: body, Raw: strings.Join(lines[i:end], "\n")}, end
}

// spanEnd is the index after the run of lines starting at i: it ends at a blank line, a
// top-level fence or a heading, except that a fence opened inside the run (a list item's code)
// holds the run together until its closing line, blank lines included.
func spanEnd(lines []string, i int) int {
	open := false
	j := i
	for ; j < len(lines); j++ {
		l := lines[j]
		switch {
		case fenceLine(l):
			if !open && j > i && strings.HasPrefix(l, "```") {
				return j
			}
			open = !open
		case open:
		case strings.TrimSpace(l) == "" || headingRe.MatchString(l):
			return j
		}
	}
	return trimBlank(lines, i, j)
}

// trimBlank lowers end past blank lines that trail lines[start:end].
func trimBlank(lines []string, start, end int) int {
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

// span reads a run of lines that is not a fence or a heading: a table, a list or a paragraph.
func span(lines []string, i int) (b Block, next int) {
	j := spanEnd(lines, i)
	run := lines[i:j]
	raw := strings.Join(run, "\n")
	if len(run) >= 2 && strings.Contains(run[0], "|") && tableSep.MatchString(strings.TrimSpace(run[1])) {
		return Block{Kind: Table, Rows: rows(run), Raw: raw}, j
	}
	if itemRe.MatchString(run[0]) {
		return Block{Kind: List, Items: items(run), Raw: raw}, j
	}
	return Block{Kind: Paragraph, Text: raw, Raw: raw}, j
}

// items reads a list's items; a line that is no item continues the one before it.
func items(run []string) []string {
	out := make([]string, 0, len(run))
	for _, l := range run {
		if m := itemRe.FindStringSubmatch(l); m != nil {
			out = append(out, m[1])
		} else {
			out[len(out)-1] += " " + strings.TrimSpace(l)
		}
	}
	return out
}

// rows splits a table's lines into cells, skipping the separator line.
func rows(run []string) [][]string {
	out := make([][]string, 0, len(run)-1)
	for n, l := range run {
		if n == 1 {
			continue
		}
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(strings.TrimSuffix(l, "|"), "|")
		cells := strings.Split(l, "|")
		for c := range cells {
			cells[c] = strings.TrimSpace(cells[c])
		}
		out = append(out, cells)
	}
	return out
}

// InlineKind is an inline span's kind.
type InlineKind int

// The inline kinds a renderer tells apart.
const (
	Text InlineKind = iota
	Bold
	Italic
	Code
	Link
)

// Inline is one span of a text: Text (or the link's text) and the link's URL.
type Inline struct {
	Kind      InlineKind
	Text, URL string
}

// escRe is a backslash escape of a character inline markup reads: it stands for the character.
var escRe = regexp.MustCompile(`\\([\\` + "`" + `*_\[\]])`)

// inlineRe matches the first inline span; inside bold, italic and a link's text an escape is one
// unit, so an escaped marker neither closes nor opens one.
var inlineRe = regexp.MustCompile(escRe.String() + "|`([^`]+)`|" +
	`\*\*((?:\\.|[^\\\n])+?)\*\*|__((?:\\.|[^\\\n])+?)__|` +
	`\*((?:\\.|[^*\s\\])(?:(?:\\.|[^*\\])*(?:\\.|[^*\s\\]))?)\*|` +
	`_((?:\\.|[^_\s\\])(?:(?:\\.|[^_\\])*(?:\\.|[^_\s\\]))?)_|` +
	`\[((?:\\.|[^\]\\])+)\]\(([^)\s]+)\)|<(https?://[^>\s]+)>`)

// Inlines splits s into spans: code first (its content is literal), then bold, italic, links.
// A backslash before \, `, *, _, [ or ] is that character as text.
// An unpaired marker stays text, and so does an underscore inside a word (snake_case_name).
func Inlines(s string) []Inline {
	var out []Inline
	add := func(in Inline) {
		if n := len(out); in.Kind == Text && n > 0 && out[n-1].Kind == Text {
			out[n-1].Text += in.Text
			return
		}
		out = append(out, in)
	}
	prev := rune(0)
	for s != "" {
		loc := inlineRe.FindStringSubmatchIndex(s)
		if loc == nil {
			add(Inline{Kind: Text, Text: s})
			break
		}
		if loc[0] > 0 {
			prev, _ = utf8.DecodeLastRuneInString(s[:loc[0]])
		}
		if inWord(s, loc, prev) {
			// An underscore flanked by a letter or digit is text: keep it and look on after it.
			add(Inline{Kind: Text, Text: s[:loc[0]+1]})
			prev, s = '_', s[loc[0]+1:]
			continue
		}
		if loc[0] > 0 {
			add(Inline{Kind: Text, Text: s[:loc[0]]})
		}
		add(span2(s, loc))
		prev, _ = utf8.DecodeLastRuneInString(s[:loc[1]])
		s = s[loc[1]:]
	}
	return out
}

// inWord reports whether the match is an underscore span opened or closed inside a word.
func inWord(s string, loc []int, prev rune) bool {
	next, _ := utf8.DecodeRuneInString(s[loc[1]:])
	return s[loc[0]] == '_' && (isWord(prev) || isWord(next))
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// span2 builds the inline for the first group that matched.
func span2(s string, loc []int) Inline {
	group := func(n int) string {
		if loc[2*n] < 0 {
			return ""
		}
		return s[loc[2*n]:loc[2*n+1]]
	}
	switch {
	case loc[2] >= 0:
		return Inline{Kind: Text, Text: group(1)}
	case loc[4] >= 0:
		return Inline{Kind: Code, Text: group(2)}
	case loc[6] >= 0, loc[8] >= 0:
		return Inline{Kind: Bold, Text: group(3) + group(4)}
	case loc[10] >= 0, loc[12] >= 0:
		return Inline{Kind: Italic, Text: group(5) + group(6)}
	case loc[14] >= 0:
		return Inline{Kind: Link, Text: escRe.ReplaceAllString(group(7), "$1"), URL: group(8)}
	}
	return Inline{Kind: Link, Text: group(9), URL: group(9)}
}
