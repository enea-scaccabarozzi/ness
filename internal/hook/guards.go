package hook

import (
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

const waitingRule = "pull once with ness inbox, then end your turn; the worker wakes you when something arrives"

// readRule is the verb for the finite read a loop over items was standing in for.
const readRule = "; to read several items use ness inbox --show A B C, or ness inbox --unread for all of them"

var (
	// pollingTools re-run their input on a timer; with a ness read inside they are polling.
	pollingTools = []string{"Monitor", "ScheduleWakeup", "CronCreate"}
	pollRead     = regexp.MustCompile(`\bness\s+(inbox|status)\b`)
	// shellLoop matches loop syntax: while/until, `for NAME in`, `for ((`, and watch in command
	// position (watch re-runs its command on a timer); prose like "ready for review" passes.
	// ponytail: a word match, so `echo while` with a ness read in the same command is blocked
	// too; parse the shell if that ever bites.
	loopOpen = regexp.MustCompile(`(^|[\s;&|('"])(while|until)\b|` +
		`(^|[\s;&|('"])for\s+(\w+\s+in\b|\(\()`)
	watchCmd = regexp.MustCompile(`(^|[;&|('"]|\bsudo|\benv)\s*watch\b`)
	loopDone = regexp.MustCompile(`(^|[;&|\n(])[ \t]*done\b`)
	// shellDashC matches a shell's -c just before a quoted script (bash -c, sh -lc); a script handed
	// to another interpreter (python3 -c) is not shell.
	shellDashC = regexp.MustCompile(`\b(ba|z|da|k)?sh\b[^;&|'"]*\s-\w*c\s+$`)
	// forIn opens a loop over a finite word list: without a sleep in it, it reads and ends.
	forIn     = regexp.MustCompile(`^[\s;&|('"]?for\s+\w+\s+in\b`)
	sleepWord = regexp.MustCompile(`\bsleep\b`)
	nessAct   = regexp.MustCompile(`\bness\s+(integrate|send|open)\b`)
	// toolResult is the shape of a saved tool-result file (Claude Code: …/<session>/tool-results/<id>.txt).
	// ponytail: path shape only; add the Codex location when known.
	toolResult = regexp.MustCompile(`/tool-results/[^\s"'|;&<>]+`)
	wholeCat   = regexp.MustCompile(`(^|[\s;&|(])cat\s`)
	sedCmd     = regexp.MustCompile(`(^|[\s;&|(])sed\s`)
)

// toolInput are the fields the guards read from any tool's input.
type toolInput struct {
	Command         string `json:"command"`
	FilePath        string `json:"file_path"`
	Offset          *int   `json:"offset"`
	Limit           *int   `json:"limit"`
	RunInBackground bool   `json:"run_in_background"`
	SubagentType    string `json:"subagent_type"`
	Model           string `json:"model"`
	Effort          string `json:"effort"`
	Prompt          string `json:"prompt"`
}

// PreTool decides a pre-tool call for a bound session: the polling guard (R-HOOK-011,
// R-HOOK-012) and the output guard (R-HOOK-013). It returns the refusal reason, or "" to allow.
func PreTool(p Payload) string {
	var in toolInput
	// A tool_input that is not an object has no command or path to guard.
	if err := json.Unmarshal(p.ToolInput, &in); err != nil {
		in = toolInput{}
	}
	if reason := polling(p.ToolName, in.Command, inputText(p.ToolInput)); reason != "" {
		return reason
	}
	if reason := explore(p.ToolName, in.SubagentType); reason != "" {
		return reason
	}
	if reason := background(p.ToolName, in); reason != "" {
		return reason
	}
	return outputGuard(p.ToolName, in)
}

// inputText joins every decoded string value of a tool input, so a polling tool's prompt or
// command is matched as text, not as JSON with escaped tabs and newlines.
func inputText(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	var b strings.Builder
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case string:
			b.WriteString(v)
			b.WriteByte('\n')
		case []any:
			for _, e := range v {
				walk(e)
			}
		case map[string]any:
			for _, k := range slices.Sorted(maps.Keys(v)) {
				walk(v[k])
			}
		}
	}
	walk(v)
	return b.String()
}

// loop is one shell loop of a command: its text with prose blanked (code) and as written (raw).
type loop struct{ code, raw string }

// loops returns every shell loop in command, from its opener to the matching `done` (the rest
// of the command when unterminated), nested loops counted; `watch ...` is a loop over the rest of
// its command.
func loops(command string) []loop {
	blanked := code(command)
	type event struct {
		pos, end int
		open     bool
	}
	evs := make([]event, 0, 8)
	for _, m := range loopOpen.FindAllStringIndex(blanked, -1) {
		evs = append(evs, event{m[0], m[1], true})
	}
	for _, m := range loopDone.FindAllStringIndex(blanked, -1) {
		evs = append(evs, event{m[0], m[1], false})
	}
	slices.SortFunc(evs, func(a, b event) int { return a.pos - b.pos })
	var out []loop
	for i, e := range evs {
		if !e.open {
			continue
		}
		end, depth := len(blanked), 1
		for _, n := range evs[i+1:] {
			if n.open {
				depth++
				continue
			}
			depth--
			if depth == 0 {
				end = n.end
				break
			}
		}
		out = append(out, loop{blanked[e.pos:end], command[e.pos:end]})
	}
	if m := watchCmd.FindStringIndex(blanked); m != nil {
		out = append(out, loop{blanked[m[0]:], command[m[0]:]})
	}
	return out
}

// waitLoops returns the loops of command that can wait without end: while/until, `for ((`,
// watch, and any loop with a sleep in it. A `for NAME in` without a sleep runs over a finite list
// and ends, so it is not one.
func waitLoops(command string) []loop {
	var out []loop
	for _, l := range loops(command) {
		if !forIn.MatchString(l.code) || sleepWord.MatchString(l.code) {
			out = append(out, l)
		}
	}
	return out
}

func polling(tool, command, raw string) string {
	if slices.Contains(pollingTools, tool) && pollRead.MatchString(raw) {
		return "polling with " + tool + " on ness inbox/status is refused: " + waitingRule + rule.Ref(rule.HookPolling)
	}
	if tool != "Bash" {
		return ""
	}
	for _, l := range waitLoops(command) {
		switch body := l.code; {
		case pollRead.MatchString(body):
			return "a waiting loop over ness inbox/status is refused: " + waitingRule + readRule +
				rule.Ref(rule.HookPolling)
		case sleepWord.MatchString(body) && nessAct.MatchString(body):
			return "a sleep loop around ness integrate, send or open is refused: run the command once, " +
				"read its refusal and act on it, or end your turn to wait" + rule.Ref(rule.HookSleepLoop)
		}
	}
	return ""
}

// explore refuses the Explore subagent: ness-sweep is the read-only search agent here.
func explore(tool, subagent string) string {
	if (tool == "Agent" || tool == "Task") && subagent == "Explore" {
		return "the Explore subagent is refused: use the ness-sweep subagent for searches" + rule.Ref(rule.Explore)
	}
	return ""
}

func outputGuard(tool string, in toolInput) string {
	switch tool {
	case "Read":
		if toolResult.MatchString(in.FilePath) && in.Offset == nil && in.Limit == nil {
			return "reading a whole saved tool result is refused: pass offset and limit, or grep it" +
				rule.Ref(rule.HookOutput)
		}
	case "Bash":
		if toolResult.MatchString(in.Command) && (wholeCat.MatchString(in.Command) ||
			sedCmd.MatchString(in.Command) && !strings.Contains(in.Command, " -n")) {
			return "cat or sed over a whole saved tool result is refused: use head, grep or sed -n with a range" +
				rule.Ref(rule.HookOutput)
		}
	}
	return ""
}

// code blanks the prose in command, keeping its length: here-doc bodies and quoted strings, except
// a string handed to a shell's `-c` (bash -c '...'), which is code; another interpreter's script
// (python3 -c "...") is not shell and is blanked.
// ponytail: a command substitution inside a double-quoted string is blanked with it.
func code(command string) string {
	b := []byte(command)
	for i := 0; i < len(command); i++ {
		switch c := command[i]; {
		case c == '\\':
			i++
		case c == '\'' || c == '"':
			end := quoteEnd(command, i)
			if !shellDashC.MatchString(command[:i]) {
				blank(b, i+1, end)
			}
			i = end
		case strings.HasPrefix(command[i:], "<<") && !strings.HasPrefix(command[i:], "<<<"):
			if last, bg := skipHeredoc(command, i+2); !bg {
				blank(b, i+lineLen(command[i:]), last+1)
				i = last
			}
		}
	}
	return string(b)
}

// quoteEnd is the index of the quote that closes the one at command[i], len(command) when none.
func quoteEnd(command string, i int) int {
	q := command[i]
	end := i + 1
	for end < len(command) && command[end] != q {
		if q == '"' && command[end] == '\\' {
			end++
		}
		end++
	}
	return min(end, len(command))
}

// blank overwrites b[from:to] with spaces, keeping newlines.
func blank(b []byte, from, to int) {
	for i := from; i < to && i < len(b); i++ {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
}
