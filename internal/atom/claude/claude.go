// Package claude is the client atom for Claude Code: launch arguments, model and effort rules,
// the transcript reader. Hooks are wired by sync into .claude/settings.json (R-HOOK-001).
package claude

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

const (
	// Tested is the Claude Code version this binary was built against (R-SESS-016).
	Tested = "2.1.294"
	// EnterpriseSkills is the enterprise skills directory on Linux (R-SESS-014, D40).
	EnterpriseSkills = "/etc/claude-code/skills"
)

// Efforts are the values --effort accepts (R-SESS-017); Aliases the model aliases the client
// resolves itself (R-MOD-031).
var (
	Efforts = config.ClaudeEfforts
	Aliases = []string{"fable", "opus", "sonnet", "haiku"}
)

// Client implements session.Client for Claude Code.
type Client struct{ Exec Exec }

// Name is the client's config and --client value and its herdr kind.
func (Client) Name() string { return "claude" }

// Tested returns the tested version.
func (Client) Tested() string { return Tested }

// Efforts returns the accepted effort values.
func (Client) Efforts() []string { return Efforts }

// Version runs `claude --version` ("2.1.294 (Claude Code)").
func (c Client) Version() (string, error) {
	out, err := c.Exec("", "claude", "--version")
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("claude: %w", session.ErrNotInstalled)
	}
	if err != nil {
		return "", err
	}
	v, _, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
	if v == "" {
		return "", fmt.Errorf("claude --version printed %q", strings.TrimSpace(string(out)))
	}
	return v, nil
}

// ResolveModel accepts the aliases and full ids; the client has no dry resolver, so the launch
// is the real check (R-MOD-031).
func (Client) ResolveModel(id string) error {
	if slices.Contains(Aliases, id) || (strings.HasPrefix(id, "claude-") && len(id) > len("claude-")) {
		return nil
	}
	return fmt.Errorf("model %q is neither an alias (%s) nor a full claude-… id", id, strings.Join(Aliases, ", "))
}

// NewSessionID returns a random UUID v4 for --session-id (R-SESS-027).
func (Client) NewSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand cannot fail on supported platforms
	}
	b[6], b[8] = (b[6]&0x0f)|0x40, (b[8]&0x3f)|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Args builds the command line after `claude` (R-SESS-025, R-SESS-027, R-SESS-028, R-SESS-047,
// R-MOD-044, R-SESS-120). No prompt: the launch submits it once the client is ready (R-SESS-025).
// --remote-control takes its optional name in the same argv element (--remote-control=<name>), so it
// never swallows the next one; the name is the session's client name, the bare flag without one.
func (Client) Args(s session.LaunchSpec) []string {
	args := []string{"--session-id", s.SessionID}
	if s.Resume {
		args = []string{"--resume", s.SessionID}
	}
	rc := "--remote-control"
	if s.Name != "" {
		rc += "=" + s.Name
	}
	args = append(args, rc, "--model", s.Model, "--effort", s.Effort)
	if s.FallbackModel != "" {
		args = append(args, "--fallback-model", s.FallbackModel)
	}
	if s.Name != "" { // an explicit name replaces the client's own title (R-SESS-120)
		args = append(args, "--name", s.Name)
	}
	if s.SkipPermissions { // the pre-tool hook still runs and refuses (R-SESS-125)
		args = append(args, "--dangerously-skip-permissions")
	}
	return args
}

// CurrentSession is the running conversation's id, which Claude Code exports to its hooks and
// shells (used by open --root --here, R-SESS-038).
func (Client) CurrentSession() string { return os.Getenv("CLAUDE_CODE_SESSION_ID") }

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// ProjectDir is where Claude Code keeps the transcripts of dir: every rune that is not an ASCII
// letter or digit becomes "-".
func ProjectDir(home, dir string) string {
	enc := nonAlnum.ReplaceAllString(dir, "-")
	return filepath.Join(home, ".claude", "projects", enc)
}

// TranscriptPath is the transcript of conversation id run in dir under home.
func TranscriptPath(home, dir, id string) string {
	return filepath.Join(ProjectDir(home, dir), id+".jsonl")
}

type usageLine struct {
	Type             string `json:"type"`
	IsMeta           bool   `json:"isMeta"`           //nolint:tagliatelle // Claude Code's field names
	IsCompactSummary bool   `json:"isCompactSummary"` //nolint:tagliatelle // Claude Code's field names
	Timestamp        string `json:"timestamp"`
	Message          struct {
		Usage struct {
			Input         int `json:"input_tokens"`
			CacheCreation int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// workTime is the time of a real work entry: a user or assistant line that is neither meta nor
// a compaction summary and has a parseable timestamp.
func (u usageLine) workTime() (time.Time, bool) {
	if (u.Type != "user" && u.Type != "assistant") || u.IsMeta || u.IsCompactSummary {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, u.Timestamp)
	return at, err == nil
}

// Transcript reads <home>/.claude/projects/<dir>/<id>.jsonl: the context size is the prompt of
// the last assistant turn (input + cache read + cache creation); lines that do not parse are
// skipped; LastActive is the last real work entry, else the file's mtime (R-SESS-080, R-STAT-010).
func (Client) Transcript(home, dir, id string) (tr session.Transcript, err error) {
	path := TranscriptPath(home, dir, id)
	f, err := os.Open(path) //nolint:gosec // the client's transcript of this session
	if err != nil {
		return tr, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return tr, err
	}
	var last time.Time
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var u usageLine
		if json.Unmarshal(sc.Bytes(), &u) != nil {
			continue
		}
		if at, ok := u.workTime(); ok && at.After(last) {
			last = at
		}
		if n := u.Message.Usage.Input + u.Message.Usage.CacheRead + u.Message.Usage.CacheCreation; n > 0 {
			tr.Context = n
		}
	}
	tr.LastActive = info.ModTime()
	if !last.IsZero() {
		tr.LastActive = last
	}
	return tr, sc.Err()
}
