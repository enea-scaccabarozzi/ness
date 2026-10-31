// Package stats reads the Claude Code transcripts bound to ness sessions and adds them up:
// tokens per price class, the context size of each turn, compactions, tool-result bytes per
// tool, and the cost from an embedded dated price table (design §5, STAT).
package stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"time"
)

// Usage is a token count per price class.
// CacheWrite counts every cache write; CacheWrite1h is the 1-hour-TTL part of it (billed higher).
type Usage struct {
	Input, Output, CacheWrite, CacheRead, CacheWrite1h int
}

// Add sums u and o.
func (u Usage) Add(o Usage) Usage {
	return Usage{
		u.Input + o.Input, u.Output + o.Output, u.CacheWrite + o.CacheWrite, u.CacheRead + o.CacheRead,
		u.CacheWrite1h + o.CacheWrite1h,
	}
}

// Transcript is what one transcript file adds up to.
type Transcript struct {
	Model       string           // the model of the last priced call
	Usage       Usage            // every call with prompt tokens, sidechain calls in the file included
	Models      map[string]Usage // Usage per model: each call is priced at its own model (Cost)
	Turns       []int            // the prompt tokens of the last API call of each turn (R-STAT-010)
	Compactions int              // compaction markers (R-STAT-010)
	Skipped     int              // lines that did not parse (R-STAT-007)
	ToolBytes   map[string]int   // tool_result bytes per tool (R-STAT-011)
}

// Cost is the transcript's cost in cents, each model's usage priced at its own rates (R-STAT-001).
// unpriced is true when some usage is on a model the table does not know: its tokens are in Usage
// but not in cents, so the figure is a lower bound.
func (t Transcript) Cost() (cents int, unpriced bool) {
	for model, u := range t.Models {
		c, ok := Cost(model, u)
		cents += c
		unpriced = unpriced || !ok
	}
	return cents, unpriced
}

// Context is the context size: the prompt tokens of the last turn's last call.
func (t Transcript) Context() int {
	if len(t.Turns) == 0 {
		return 0
	}
	return t.Turns[len(t.Turns)-1]
}

type block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

type line struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Compact   bool   `json:"isCompactSummary"` //nolint:tagliatelle // Claude Code's field names
	Sidechain bool   `json:"isSidechain"`      //nolint:tagliatelle // Claude Code's field names
	Message   struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   struct {
			Input         int `json:"input_tokens"`
			Output        int `json:"output_tokens"`
			CacheWrite    int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
			CacheCreation struct {
				OneHour int `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

type call struct {
	model string
	usage Usage
}

// reader carries the state of one Read.
type reader struct {
	t     Transcript
	tools map[string]string // tool_use id -> tool name
	calls map[string]call   // message.id -> the last line seen for it
	anon  []call            // calls whose line carries no message.id: each line counts once
	cur   int               // prompt tokens of the last call of the open turn
}

// Read sums the JSONL transcript r, keeping entries at or after since (zero: all). A line that
// is not a JSON object is skipped and counted; nothing panics (R-STAT-007).
func Read(r io.Reader, since time.Time) (Transcript, error) {
	rd := reader{t: Transcript{ToolBytes: map[string]int{}}, tools: map[string]string{}, calls: map[string]call{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue // a blank line is not a bad line
		}
		var l line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil { // JSON null decodes to an empty line: ignored
			rd.t.Skipped++
			continue
		}
		if !inRange(l.Timestamp, since) {
			continue
		}
		switch l.Type {
		case "assistant":
			rd.assistant(l)
		case "user":
			rd.user(l)
		}
	}
	rd.closeTurn()
	rd.t.Models = map[string]Usage{}
	for _, c := range append(rd.anon, slices.Collect(maps.Values(rd.calls))...) {
		rd.t.Models[c.model] = rd.t.Models[c.model].Add(c.usage)
		rd.t.Usage = rd.t.Usage.Add(c.usage)
	}
	return rd.t, sc.Err()
}

func inRange(stamp string, since time.Time) bool {
	if since.IsZero() {
		return true
	}
	ts, err := time.Parse(time.RFC3339Nano, stamp)
	return err == nil && !ts.Before(since)
}

func blocks(raw json.RawMessage) []block {
	var out []block
	if json.Unmarshal(raw, &out) != nil {
		return nil // a plain string, or absent
	}
	return out
}

// assistant records a priced call's usage and remembers its tool uses; the call's prompt is the
// open turn's context. Claude Code writes one line per content block, each repeating the
// message.id and usage of its API call, so a call is keyed by that id and the last line wins
// (P-02). A call without prompt tokens (synthetic) is neither. Sidechain (subagent) calls are
// billed but are not turns.
func (rd *reader) assistant(l line) {
	for _, b := range blocks(l.Message.Content) {
		if b.Type == "tool_use" {
			rd.tools[b.ID] = b.Name
		}
	}
	m := l.Message.Usage
	prompt := m.Input + m.CacheWrite + m.CacheRead
	if prompt == 0 {
		return
	}
	u := Usage{
		Input: m.Input, Output: m.Output, CacheWrite: m.CacheWrite, CacheRead: m.CacheRead,
		CacheWrite1h: min(max(m.CacheCreation.OneHour, 0), max(m.CacheWrite, 0)), // absent split: all 5-minute
	}
	if c := (call{l.Message.Model, u}); l.Message.ID == "" {
		rd.anon = append(rd.anon, c)
	} else {
		rd.calls[l.Message.ID] = c
	}
	rd.t.Model = l.Message.Model
	if !l.Sidechain {
		rd.cur = prompt
	}
}

// user counts a compaction marker, attributes tool_result bytes to their tools, and closes the
// open turn on a human message (one without tool results, outside a sidechain).
func (rd *reader) user(l line) {
	if l.Compact {
		rd.t.Compactions++
	}
	results := 0
	for _, b := range blocks(l.Message.Content) {
		if b.Type != "tool_result" {
			continue
		}
		results++
		name := rd.tools[b.ToolUseID]
		if name == "" {
			name = "unknown"
		}
		rd.t.ToolBytes[name] += resultBytes(b.Content)
	}
	if results == 0 && !l.Sidechain {
		rd.closeTurn()
	}
}

func (rd *reader) closeTurn() {
	if rd.cur > 0 {
		rd.t.Turns = append(rd.t.Turns, rd.cur)
	}
	rd.cur = 0
}

// resultBytes is the size of a tool result: the text of a string, else its JSON.
func resultBytes(raw json.RawMessage) int {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return len(s)
	}
	return len(raw)
}
