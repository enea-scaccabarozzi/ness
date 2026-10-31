// Package sync renders core, features and profiles into a repository: whole files, managed
// blocks in shared files, owned JSON keys and symlinks, all recorded in .ness/lock.json. It
// never overwrites or deletes content it did not record (D4).
package sync

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Style is the comment syntax of block markers in one kind of file (R-SYNC-040).
type Style struct {
	begin, end, close string
}

var (
	markdownStyle = Style{begin: "<!-- ness:begin ", end: "<!-- ness:end ", close: " -->"}
	hashStyle     = Style{begin: "# ness:begin ", end: "# ness:end ", close: ""}
	blockNameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$`)
)

// styleFor picks Markdown markers for .md files and # markers for everything else.
func styleFor(path string) Style {
	if strings.HasSuffix(path, ".md") {
		return markdownStyle
	}
	return hashStyle
}

func (s Style) beginLine(name string) string { return s.begin + name + s.close + "\n" }
func (s Style) endLine(name string) string   { return s.end + name + s.close + "\n" }

// marker classifies one line as "begin", "end" or "" and returns the name it carries.
// ponytail: a marker line inside a fenced code block still counts; quote markers indented in prose.
func (s Style) marker(line string) (kind, name string) {
	t := strings.TrimSpace(line)
	if rest, ok := strings.CutPrefix(t, s.begin); ok {
		name, _ = strings.CutSuffix(rest, s.close)
		return "begin", strings.TrimSpace(name)
	}
	if rest, ok := strings.CutPrefix(t, s.end); ok {
		name, _ = strings.CutSuffix(rest, s.close)
		return "end", strings.TrimSpace(name)
	}
	return "", ""
}

// block is one managed block; offsets are byte positions in the file.
type block struct {
	name                        string
	line                        int
	start, inner, innerEnd, end int
}

// parseBlocks finds every managed block. Malformed markers are an error naming the line.
func parseBlocks(data []byte, s Style) ([]block, error) {
	var (
		out  []block
		open *block
		off  int
	)
	seen := map[string]bool{}
	for i, raw := range bytes.SplitAfter(data, []byte("\n")) {
		kind, name := s.marker(string(raw))
		switch kind {
		case "begin":
			if err := checkBegin(name, open, seen, i+1); err != nil {
				return nil, err
			}
			open = &block{name: name, line: i + 1, start: off, inner: off + len(raw)}
		case "end":
			if open == nil || open.name != name {
				return nil, fmt.Errorf("line %d: end marker for %q without its begin", i+1, name)
			}
			open.innerEnd, open.end = off, off+len(raw)
			out = append(out, *open)
			seen[name] = true
			open = nil
		}
		off += len(raw)
	}
	if open != nil {
		return nil, fmt.Errorf("line %d: block %q has no end marker", open.line, open.name)
	}
	return out, nil
}

func checkBegin(name string, open *block, seen map[string]bool, line int) error {
	switch {
	case !blockNameRe.MatchString(name):
		return fmt.Errorf("line %d: malformed block name %q", line, name)
	case open != nil:
		return fmt.Errorf("line %d: block %q begins inside block %q", line, name, open.name)
	case seen[name]:
		return fmt.Errorf("line %d: duplicate block %q", line, name)
	}
	return nil
}

func findBlock(data []byte, s Style, name string) (block, bool, error) {
	blocks, err := parseBlocks(data, s)
	if err != nil {
		return block{}, false, err
	}
	for _, b := range blocks {
		if b.name == name {
			return b, true, nil
		}
	}
	return block{}, false, nil
}

// normInner makes non-empty block content end with a newline.
func normInner(inner []byte) []byte {
	if len(inner) > 0 && !bytes.HasSuffix(inner, []byte("\n")) {
		return slices.Concat(inner, []byte("\n"))
	}
	return inner
}

// getBlock returns the content between block name's markers and whether the block exists.
func getBlock(data []byte, s Style, name string) (inner []byte, found bool, err error) {
	b, ok, err := findBlock(data, s, name)
	if err != nil || !ok {
		return nil, false, err
	}
	return data[b.inner:b.innerEnd], true, nil
}

// setBlock replaces block name's content in place wherever the block sits (R-SYNC-041), or
// appends the block at the end of the file (R-SYNC-042).
func setBlock(data []byte, s Style, name string, inner []byte) ([]byte, error) {
	b, ok, err := findBlock(data, s, name)
	if err != nil {
		return nil, err
	}
	inner = normInner(inner)
	if ok {
		return slices.Concat(data[:b.inner], inner, data[b.innerEnd:]), nil
	}
	var out bytes.Buffer
	out.Write(data)
	if len(data) > 0 {
		if !bytes.HasSuffix(data, []byte("\n")) {
			out.WriteByte('\n')
		}
		out.WriteByte('\n')
	}
	out.WriteString(s.beginLine(name))
	out.Write(inner)
	out.WriteString(s.endLine(name))
	return out.Bytes(), nil
}

// removeBlock deletes block name with its markers, and the blank line setBlock put before it.
func removeBlock(data []byte, s Style, name string) ([]byte, error) {
	b, ok, err := findBlock(data, s, name)
	if err != nil || !ok {
		return data, err
	}
	head := data[:b.start]
	if bytes.HasSuffix(head, []byte("\n\n")) {
		head = head[:len(head)-1]
	}
	return slices.Concat(head, data[b.end:]), nil
}
