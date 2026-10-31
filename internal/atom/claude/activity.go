package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// pendingMax is how long a launch counts as running without any record of it: a notification lost
// to a client restart would otherwise keep the session excused from every report. The longest run
// seen in the field is under two hours.
// ponytail: a flat 3 hours; read the client's task list instead if an agent ever runs longer.
const pendingMax = 3 * time.Hour

// idleFile is how long a transcript may go unasked before Transcripts forgets what it read of it.
const idleFile = time.Hour

var (
	notificationRe = regexp.MustCompile(`(?s)^\s*<task-notification>.*?</task-notification>`)
	taskIDRe       = regexp.MustCompile(`<task-id>([\w-]+)</task-id>`)
	// remoteRe is a Remote Control URL the alert can put in a Markdown link as it is: no character
	// that ends a link target or opens markup (parentheses, brackets, "*", backtick, "|", quotes).
	remoteRe = regexp.MustCompile(`^https://[A-Za-z0-9._~:/?#@!$&'+,;=%-]+$`)
	// stillRunningRe is the notice an agent sends when it stops with background work of its own:
	// that is not a completion.
	stillRunningRe = regexp.MustCompile(`is waiting on its own background work|stopped with background work of its own`)
	// recordTypes are the record types fold reads, in both JSON spacings, so a line of any other
	// type (the API request blobs are the big ones) is skipped before it is parsed.
	recordTypes = [][]byte{
		[]byte(`"type":"user"`), []byte(`"type": "user"`),
		[]byte(`"type":"assistant"`), []byte(`"type": "assistant"`),
		[]byte(`"type":"queue-operation"`), []byte(`"type": "queue-operation"`),
		[]byte(`"type":"attachment"`), []byte(`"type": "attachment"`),
		[]byte(`"type":"system"`), []byte(`"type": "system"`),
	}
)

// record is the part of a transcript line Activity reads.
type record struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype"`
	Timestamp string          `json:"timestamp"`
	Sidechain bool            `json:"isSidechain"`       //nolint:tagliatelle // Claude Code's field names
	APIError  bool            `json:"isApiErrorMessage"` //nolint:tagliatelle // Claude Code's field names
	Compact   bool            `json:"isCompactSummary"`  //nolint:tagliatelle // Claude Code's field names
	Content   json.RawMessage `json:"content"`           // queue-operation: the queued text
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Attachment struct {
		Prompt json.RawMessage `json:"prompt"` // a queued command: the queued text
		Type   string          `json:"type"`
		URL    string          `json:"url"` // remote_session_change: the Remote Control URL
	} `json:"attachment"`
	ToolUseResult json.RawMessage `json:"toolUseResult"` //nolint:tagliatelle // Claude Code's field names
}

// toolResult is the structured result a tool leaves on its user record. Its ids start and stop
// background work; the text of a result is never read, because a tool result can quote anything.
type toolResult struct {
	Status       string `json:"status"`
	AgentID      string `json:"agentId"`          //nolint:tagliatelle // an agent launched async
	TaskID       string `json:"taskId"`           //nolint:tagliatelle // a workflow launched async
	Background   string `json:"backgroundTaskId"` //nolint:tagliatelle // a command moved to the background
	Resumed      string `json:"resumedAgentId"`   //nolint:tagliatelle // an agent sent another message
	StoppedTask  string `json:"task_id"`          //nolint:tagliatelle // TaskStop
	StoppedShell string `json:"shell_id"`         //nolint:tagliatelle // KillShell
}

// scan is what Transcripts remembers of one file: how far it was read and what the records so far say.
type scan struct {
	off     int64
	asked   time.Time            // the last Activity call that named this file
	pending map[string]time.Time // task id → when it was launched or last resumed
	turn    time.Time
	limit   string
	limitAt time.Time
	remote  string // the last Remote Control URL
}

// Transcripts reads the activity of Claude Code conversations from their transcripts and keeps each
// file's state between reads, so a pass of the worker costs the bytes appended since the last one.
// A file that is not asked for in an hour is forgotten. Transcripts only grow: one that shrinks is
// read again from the start, but one rewritten longer than it was between two reads is not noticed.
// It is not safe for concurrent use: the worker is one goroutine. The zero value is ready.
type Transcripts struct{ files map[string]*scan }

// Activity folds the transcript at path into what the worker acts on (R-WRK-096): the background
// work launched and not finished, the last real assistant turn, and a usage-limit error that is the
// last thing the client said, and the last Remote Control URL. false when the file cannot be read.
func (t *Transcripts) Activity(path string, now time.Time) (session.Activity, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return session.Activity{}, false
	}
	if t.files == nil {
		t.files = map[string]*scan{}
	}
	t.forget(now)
	sc := t.files[path]
	if sc == nil || info.Size() < sc.off { // new, or rewritten shorter: read it all again
		sc = &scan{pending: map[string]time.Time{}}
		t.files[path] = sc
	}
	sc.asked = now
	if info.Size() > sc.off {
		if err = sc.read(path); err != nil {
			return session.Activity{}, false
		}
	}
	return session.Activity{
		Pending: sc.running(now), Turn: sc.turn, Limit: sc.limit, LimitAt: sc.limitAt, Remote: sc.remote,
	}, true
}

// forget drops the files that were not asked for within idleFile.
func (t *Transcripts) forget(now time.Time) {
	for p, old := range t.files {
		if now.Sub(old.asked) > idleFile {
			delete(t.files, p)
		}
	}
}

// running reports whether any launch is younger than pendingMax.
func (sc *scan) running(now time.Time) bool {
	for _, at := range sc.pending {
		if now.Sub(at) < pendingMax {
			return true
		}
	}
	return false
}

// read folds the complete lines after sc.off into sc; a last line without its newline is still
// being written and waits for the next read.
func (sc *scan) read(path string) (err error) {
	f, err := os.Open(path) //nolint:gosec // the client's transcript of this session
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if _, err = f.Seek(sc.off, io.SeekStart); err != nil {
		return err
	}
	rd := bufio.NewReader(f)
	for {
		line, rerr := rd.ReadBytes('\n')
		if rerr != nil { // io.EOF: what is left is a partial line, read again next time
			if errors.Is(rerr, io.EOF) {
				return nil
			}
			return rerr
		}
		sc.off += int64(len(line))
		sc.fold(line)
	}
}

// fold applies one transcript line. Only user, assistant, attachment, queue-operation and system
// records matter. A remote_session_change attachment carries the session's Remote Control URL.
// Background work comes from the ids a tool result carries as fields and from task notifications
// that are the whole queued or typed text; text inside a tool result, which can quote a launch or a
// notification, is never read.
func (sc *scan) fold(line []byte) {
	if !relevant(line) {
		return
	}
	var r record
	if json.Unmarshal(line, &r) != nil || r.Sidechain {
		return
	}
	at, _ := time.Parse(time.RFC3339Nano, r.Timestamp) //nolint:errcheck // a record without a time folds as zero
	switch r.Type {
	case "assistant":
		sc.assistant(r, at)
	case "user":
		sc.launched(r.ToolUseResult, at)
		sc.notified(text(r.Message.Content), at)
	case "queue-operation":
		sc.notified(text(r.Content), at)
	case "attachment":
		sc.attachment(r, at)
	case "system":
		if r.Subtype == "agents_killed" { // the client killed everything it had running
			clear(sc.pending)
		}
	}
}

// attachment applies an attachment record: a queued command's notification, or the session's
// Remote Control URL.
func (sc *scan) attachment(r record, at time.Time) {
	sc.notified(text(r.Attachment.Prompt), at)
	if r.Attachment.Type == "remote_session_change" && remoteRe.MatchString(r.Attachment.URL) {
		sc.remote = r.Attachment.URL
	}
}

// relevant reports whether line is of a type fold reads.
func relevant(line []byte) bool {
	for _, t := range recordTypes {
		if bytes.Contains(line, t) {
			return true
		}
	}
	return false
}

// text is raw as a JSON string, or "" when it is anything else (a list of blocks is a tool result).
func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

// launched applies a tool's structured result: ids it started enter the pending set, ids it
// stopped leave it.
func (sc *scan) launched(raw json.RawMessage, at time.Time) {
	var tr toolResult
	if json.Unmarshal(raw, &tr) != nil {
		return
	}
	for _, id := range []string{tr.Background, tr.Resumed} {
		if id != "" {
			sc.pending[id] = at
		}
	}
	if tr.Status == "async_launched" {
		for _, id := range []string{tr.AgentID, tr.TaskID} {
			if id != "" {
				sc.pending[id] = at
			}
		}
	}
	delete(sc.pending, tr.StoppedTask)
	delete(sc.pending, tr.StoppedShell)
}

// notified applies the task notification that is all of s: a finished task leaves the pending set,
// an agent that stopped with background work of its own stays in it.
func (sc *scan) notified(s string, at time.Time) {
	n := notificationRe.FindString(s)
	id := taskIDRe.FindStringSubmatch(n)
	switch {
	case id == nil:
	case stillRunningRe.MatchString(n):
		sc.pending[id[1]] = at
	default:
		delete(sc.pending, id[1])
	}
}

// assistant notes a turn: a usage-limit error is the client's last word until a real turn follows
// it. Other API errors are neither a turn nor a limit; the compaction summary is not a turn.
func (sc *scan) assistant(r record, at time.Time) {
	if r.Compact {
		return
	}
	if r.APIError {
		var blocks []struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(r.Message.Content, &blocks) == nil && len(blocks) > 0 {
			if lim, ok := ParseLimit(blocks[0].Text, at); ok {
				sc.limit, sc.limitAt = lim.Notice, at
			}
		}
		return
	}
	sc.turn, sc.limit = at, ""
}
