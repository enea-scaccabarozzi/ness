package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"slices"
	"strings"
)

// userLine is what OperatorSaid reads of a transcript line: who typed it and what.
type userLine struct {
	Type      string `json:"type"`
	Meta      bool   `json:"isMeta"`           //nolint:tagliatelle // Claude Code's field names
	Sidechain bool   `json:"isSidechain"`      //nolint:tagliatelle // Claude Code's field names
	Compact   bool   `json:"isCompactSummary"` //nolint:tagliatelle // Claude Code's field names
	Source    string `json:"promptSource"`     //nolint:tagliatelle // Claude Code's field names
	Origin    struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// nessTyped are the prefixes of texts ness itself types into a conversation (the worker's hint,
// the compaction and rename commands, the launch prompt): never the operator's words.
var nessTyped = []string{"ness: ", "/compact", "/rename ", "You are ness session "}

// OperatorSaid is the package's OperatorSaid over the transcript of conversation id run in dir
// under home (R-SESS-121).
func (Client) OperatorSaid(home, dir, id, words string) (bool, error) {
	return OperatorSaid(TranscriptPath(home, dir, id), words)
}

// OperatorSaid reports whether words (at least two of them) appear as whole consecutive words,
// case-insensitively, in a message the operator typed in the transcript at path (R-SESS-121): a
// user line marked typed by a human whose content is a string, not a tool result, not meta,
// sidechain or a compaction summary, and not a text ness or the client types itself (slash
// commands and local command output are recorded as `<command-name>`-style strings). Lines that
// do not parse are skipped; a missing file is os.IsNotExist.
func OperatorSaid(path, words string) (bool, error) {
	want := strings.Fields(strings.ToLower(words))
	if len(want) < 2 {
		return false, nil
	}
	f, err := os.Open(path) //nolint:gosec // the client's transcript of this session
	if err != nil {
		return false, err
	}
	defer f.Close() //nolint:errcheck // read only
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var l userLine
		if json.Unmarshal(sc.Bytes(), &l) != nil || !typedByOperator(l) {
			continue
		}
		var text string
		if json.Unmarshal(l.Message.Content, &text) != nil {
			continue // a content array: tool results
		}
		if isNessTyped(text) {
			continue
		}
		if containsWords(strings.Fields(strings.ToLower(text)), want) {
			return true, nil
		}
	}
	return false, sc.Err()
}

// typedByOperator is a human's line: typed at the prompt, or typed while the agent was busy
// and queued until its turn ended.
func typedByOperator(l userLine) bool {
	return l.Type == "user" && !l.Meta && !l.Sidechain && !l.Compact &&
		(l.Source == "typed" || l.Source == "queued") && l.Origin.Kind == "human"
}

// isNessTyped is true for the texts ness types and for what the client records of its own
// commands: a slash command, local command output or a bash prompt, as a string starting with
// a tag.
func isNessTyped(text string) bool {
	if text == ResumeText || strings.HasPrefix(text, "<") {
		return true
	}
	for _, p := range nessTyped {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

// containsWords reports whether want occurs in have as consecutive whole words.
func containsWords(have, want []string) bool {
	for i := 0; i+len(want) <= len(have); i++ {
		if slices.Equal(have[i:i+len(want)], want) {
			return true
		}
	}
	return false
}
