package hook

import (
	"regexp"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

const bgRule = "run it with the Bash tool's run_in_background option and end your turn; the task notification wakes you"

// pgrepSelf matches a pgrep with -f (alone, in a cluster, after other flags) or --full: it searches
// full command lines, which include the waiting loop's own bash -c text.
var pgrepSelf = regexp.MustCompile(`\bpgrep\b[^;|&\n]*\s(-\w*f\w*|--full)(\s|$)`)

// background decides the R-HOOK-020 and R-HOOK-021 refusals; it returns "" to allow.
func background(tool string, in toolInput) string {
	if tool != "Bash" {
		return ""
	}
	if waitsOnPgrep(in.Command) {
		return "a wait loop over pgrep -f is refused: the pattern matches the loop's own command line " +
			"and never ends; " + bgRule + rule.Ref(rule.HookBackground)
	}
	if !in.RunInBackground && backgrounded(in.Command) {
		return "backgrounding with & is refused: nothing wakes you when the job ends; " +
			bgRule + rule.Ref(rule.HookBackground)
	}
	return ""
}

// waitsOnPgrep reports a wait loop (waitLoops) that runs pgrep -f; a one-shot pgrep, or one in a
// `for` over a finite list, is a lookup and passes. It reads the loop as written, so a pgrep in a
// quoted $( ) still counts.
func waitsOnPgrep(command string) bool {
	for _, l := range waitLoops(command) {
		if pgrepSelf.MatchString(l.raw) {
			return true
		}
	}
	return false
}

// backgrounded reports an unquoted lone &. It tracks single and double quotes, backslashes,
// comments, here-doc bodies (also inside a $( ) in double quotes), $(( )) arithmetic and ${ }, and
// skips &&, &>, >&, <& and |&. An & with a wait later on its line runs in the foreground in effect.
// ponytail: ${ } ends at the first }, and a $(( that is really $( ( reads as arithmetic.
func backgrounded(cmd string) bool {
	_, bg := scan(cmd, 0, false)
	return bg
}

// waitRe matches the wait command in the rest of a line.
var waitRe = regexp.MustCompile(`(^|[\s;&|])wait(\s|;|$)`)

// scan reads unquoted shell from cmd[i]. In a command substitution (nested) it stops at the ) that
// closes it and returns that index; it returns early with bg true at a backgrounding &.
func scan(cmd string, i int, nested bool) (end int, bg bool) {
	for ; i < len(cmd); i++ {
		if next, ok := skipOpaque(cmd, i); ok {
			i = next
		} else if nested && cmd[i] == ')' {
			return i, false
		} else if i, bg = scanOp(cmd, i, nested); bg {
			return i, true
		}
	}
	return len(cmd), false
}

// scanOp reads the quote, group, here-doc or & at cmd[i], if there is one, and returns the index of
// its last byte (i for anything else) and whether it backgrounds.
func scanOp(cmd string, i int, nested bool) (last int, bg bool) {
	switch c := cmd[i]; {
	case c == '"' || strings.HasPrefix(cmd[i:], "$(") || (nested && c == '('):
		return scanGroup(cmd, i)
	case strings.HasPrefix(cmd[i:], "<<") && !strings.HasPrefix(cmd[i:], "<<<"):
		return skipHeredoc(cmd, i+2)
	case c == '&':
		bg = lone(cmd, i) && !waitRe.MatchString(cmd[i+1:i+1+lineLen(cmd[i+1:])])
		return i + strings.Count(cmd[i:min(i+2, len(cmd))], "&") - 1, bg
	}
	return i, false
}

// skipOpaque returns the index of the last byte of a backslash pair, a single-quoted string, a
// comment, $(( )) arithmetic or ${ } at cmd[i], where no & can background.
func skipOpaque(cmd string, i int) (last int, ok bool) {
	switch {
	case cmd[i] == '\\':
		return i + 1, true
	case cmd[i] == '\'':
		return skipTo(cmd, i+1, '\''), true
	case cmd[i] == '#' && (i == 0 || strings.IndexByte(" \t\n", cmd[i-1]) >= 0):
		return i + lineLen(cmd[i:]), true
	case strings.HasPrefix(cmd[i:], "$(("):
		return skipArith(cmd, i), true
	case strings.HasPrefix(cmd[i:], "${"):
		return skipTo(cmd, i+2, '}'), true
	}
	return i, false
}

// scanGroup reads the double-quoted string or the ( ) group that starts at cmd[i] and returns the
// index of its last byte.
func scanGroup(cmd string, i int) (end int, bg bool) {
	switch cmd[i] {
	case '"':
		return scanQuoted(cmd, i+1)
	case '$':
		return scan(cmd, i+2, true)
	}
	return scan(cmd, i+1, true)
}

// scanQuoted reads a double-quoted string from just past its opening quote and returns the index
// of the closing quote; a $( ) inside it is unquoted shell.
func scanQuoted(cmd string, i int) (end int, bg bool) {
	for ; i < len(cmd); i++ {
		switch {
		case cmd[i] == '\\':
			i++
		case cmd[i] == '"':
			return i, false
		case strings.HasPrefix(cmd[i:], "$(("):
			i = skipArith(cmd, i)
		case strings.HasPrefix(cmd[i:], "$("):
			if i, bg = scan(cmd, i+2, true); bg {
				return i, true
			}
		}
	}
	return len(cmd), false
}

// skipTo returns the index of the first q at or after i, len(cmd) when none.
func skipTo(cmd string, i int, q byte) int {
	if n := strings.IndexByte(cmd[min(i, len(cmd)):], q); n >= 0 {
		return i + n
	}
	return len(cmd)
}

// skipArith returns the index of the last ) of the $(( )) starting at cmd[i].
func skipArith(cmd string, i int) int {
	for depth, j := 0, i+1; j < len(cmd); j++ {
		switch cmd[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(cmd)
}

// lineLen is the length of s up to, not including, its first newline.
func lineLen(s string) int {
	if n := strings.IndexByte(s, '\n'); n >= 0 {
		return n
	}
	return len(s)
}

// lone reports whether the & at cmd[i] backgrounds, not part of &>, >&, <& or |&.
func lone(cmd string, i int) bool {
	if strings.HasPrefix(cmd[i:], "&>") || strings.HasPrefix(cmd[i:], "&&") {
		return false
	}
	return i == 0 || strings.IndexByte("<>|", cmd[i-1]) < 0
}

// skipHeredoc is called with at just past a <<. It returns the index of the last byte of the
// here-doc body (len-1 when unterminated), or bg true when the rest of the opening line
// backgrounds.
func skipHeredoc(cmd string, at int) (last int, bg bool) {
	rest := strings.TrimPrefix(cmd[at:], "-")
	strip := len(rest) < len(cmd[at:])
	rest = strings.TrimLeft(rest, " \t")
	n := strings.IndexAny(rest, " \t\n;&|<>()")
	if n < 0 {
		n = len(rest)
	}
	word := strings.Trim(rest[:n], `'"\`)
	open := len(cmd) - len(rest) + n
	eol := open + lineLen(cmd[open:])
	if backgrounded(cmd[open:eol]) {
		return 0, true
	}
	for pos := eol + 1; pos <= len(cmd); {
		end := pos + lineLen(cmd[pos:])
		line := cmd[pos:end]
		if strip {
			line = strings.TrimLeft(line, "\t")
		}
		if line == word {
			return end - 1, false
		}
		pos = end + 1
	}
	return len(cmd) - 1, false
}
