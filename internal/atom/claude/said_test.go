package claude_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
)

func TestOperatorSaidReadsOnlyTypedLines(t *testing.T) {
	// The operator's words count only from a line they typed: not a tool result, a meta, system
	// or sidechain line, a compaction summary, or a text ness types itself (CX-19).
	// covers: R-SESS-121
	typed := func(text string) string {
		return `{"type":"user","message":{"role":"user","content":` + text + `},"promptSource":"typed","origin":{"kind":"human"}}`
	}
	cases := []struct {
		name, line string
		said       bool
	}{
		{"typed", typed(`"ok, Close  IT now"`), true},
		{"queued while the agent worked",
			`{"type":"user","message":{"role":"user","content":"Perfect, close it"},"promptSource":"queued","origin":{"kind":"human"}}`, true},
		{"queued hint", `{"type":"user","message":{"role":"user","content":"ness: close it"},"promptSource":"queued","origin":{"kind":"human"}}`, false},
		{"typed without the new fields", `{"type":"user","message":{"role":"user","content":"close it"}}`, false},
		{"slash command as the client records it",
			typed(`"<command-name>/compact</command-name>\n<command-args>close it</command-args>"`), false},
		{"local command output", typed(`"<local-command-stdout>close it</local-command-stdout>"`), false},
		{"words inside other words", typed(`"disclose items"`), false},
		{"tool result", `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"close it"}]}}`, false},
		{"meta", `{"type":"user","isMeta":true,"message":{"role":"user","content":"close it"}}`, false},
		{"system prompt", `{"type":"user","promptSource":"system","message":{"role":"user","content":"close it"}}`, false},
		{"sidechain", `{"type":"user","isSidechain":true,"message":{"role":"user","content":"close it"}}`, false},
		{"compact summary", `{"type":"user","isCompactSummary":true,"message":{"role":"user","content":"close it"}}`, false},
		{"assistant", `{"type":"assistant","message":{"role":"assistant","content":"close it"}}`, false},
		{"hint", typed(`"ness: 1 unread (close it); run ness inbox"`), false},
		{"compact", typed(`"/compact close it"`), false},
		{"rename", typed(`"/rename close it"`), false},
		{"launch prompt", typed(`"You are ness session s-1 (x). close it"`), false},
		{"other words", typed(`"close the window"`), false},
		{"not json", `close it`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "s.jsonl")
			if err := os.WriteFile(path, []byte(`{"type":"assistant"}`+"\n"+c.line+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			said, err := claude.OperatorSaid(path, "close it")
			if err != nil || said != c.said {
				t.Fatalf("said = %v, %v; want %v", said, err, c.said)
			}
		})
	}
	if _, err := claude.OperatorSaid(filepath.Join(t.TempDir(), "none.jsonl"), "close it"); !os.IsNotExist(err) {
		t.Fatalf("missing transcript: %v", err)
	}
	// Fewer than two words never authorize: a blank or a common word would match anything.
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, []byte(typed(`"please close it"`)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, words := range []string{"", " ", "close", "e"} {
		if said, err := claude.OperatorSaid(path, words); said || err != nil {
			t.Fatalf("%q: said = %v, %v", words, said, err)
		}
	}
}
