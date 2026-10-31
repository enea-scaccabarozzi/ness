package notify

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrBlockTooLong is returned when one block alone exceeds the channel's limit (R-NTF-012).
var ErrBlockTooLong = errors.New("a block exceeds the channel's limit")

// Renderer is what the splitter needs from a channel: one block to platform text, the measure
// the platform counts in, the per-message limit (0: none) and the blocks per message (0: none).
type Renderer interface {
	Render(Block) string
	Measure(string) int
	Limit() int
	MaxBlocks() int
}

// Part is one message: its rendered blocks, joined by a blank line.
type Part struct{ Blocks []string }

// Text joins the part's blocks.
func (p Part) Text() string { return strings.Join(p.Blocks, "\n\n") }

// Split renders the blocks and packs them into parts within Limit and MaxBlocks, cutting only
// between blocks (R-NTF-011), so never inside a code fence or a multi-byte rune; a block over
// the limit by itself is ErrBlockTooLong naming it, before anything is sent (R-NTF-012).
func Split(r Renderer, blocks []Block) ([]Part, error) {
	var parts []Part
	var cur Part
	size := 0
	flush := func() {
		if len(cur.Blocks) > 0 {
			parts = append(parts, cur)
			cur, size = Part{}, 0
		}
	}
	for _, b := range blocks {
		text := r.Render(b)
		n := r.Measure(text)
		if r.Limit() > 0 && n > r.Limit() {
			return nil, fmt.Errorf("%w: %s %q (%d > %d)", ErrBlockTooLong, kindName(b.Kind), head(b.Raw), n, r.Limit())
		}
		sep := 0
		if len(cur.Blocks) > 0 {
			sep = r.Measure("\n\n")
		}
		if (r.Limit() > 0 && size+sep+n > r.Limit()) || (r.MaxBlocks() > 0 && len(cur.Blocks) == r.MaxBlocks()) {
			flush()
			sep = 0
		}
		cur.Blocks = append(cur.Blocks, text)
		size += sep + n
	}
	flush()
	return parts, nil
}

func kindName(k Kind) string {
	return [...]string{"paragraph", "heading", "code fence", "list", "table"}[k]
}

// head is the first 40 characters of s on one line, for an error message.
func head(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if utf8.RuneCountInString(s) > 40 {
		s = string([]rune(s)[:40]) + "…"
	}
	return s
}
