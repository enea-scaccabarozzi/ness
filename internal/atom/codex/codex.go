// Package codex is the experimental client atom for Codex (R-CFG-044): launch arguments with
// the trust and update overrides that keep the launch free of dialogs (R-SESS-113), hooks on
// the command line (R-HOOK-018), model ids checked against `codex debug models` (R-MOD-030).
package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Tested is the Codex version this binary was built against (R-SESS-016).
const Tested = "0.162.1"

// Efforts are the reasoning efforts Codex accepts (R-MOD-033).
var Efforts = config.CodexEfforts

// Client implements session.Client for Codex. Model ids are fetched once per process.
type Client struct {
	Exec   Exec
	once   sync.Once
	models []string
	merr   error
}

// Name is the client's config and --client value and its herdr kind.
func (*Client) Name() string { return "codex" }

// Tested returns the tested version.
func (*Client) Tested() string { return Tested }

// Efforts returns the accepted effort values.
func (*Client) Efforts() []string { return Efforts }

// Version runs `codex --version` ("codex-cli 0.162.1").
func (c *Client) Version() (string, error) {
	out, err := c.Exec("", "codex", "--version")
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("codex: %w", session.ErrNotInstalled)
	}
	if err != nil {
		return "", err
	}
	_, v, ok := strings.Cut(strings.TrimSpace(string(out)), " ")
	if !ok || v == "" {
		return "", fmt.Errorf("codex --version printed %q", strings.TrimSpace(string(out)))
	}
	return v, nil
}

// ResolveModel accepts only a slug listed by `codex debug models` (R-MOD-030, R-MOD-032).
func (c *Client) ResolveModel(id string) error {
	c.once.Do(func() {
		out, err := c.Exec("", "codex", "debug", "models")
		if err != nil {
			c.merr = err
			return
		}
		var list struct {
			Models []struct {
				Slug string `json:"slug"`
			} `json:"models"`
		}
		if uerr := json.Unmarshal(out, &list); uerr != nil {
			c.merr = fmt.Errorf("codex debug models: %w", uerr)
			return
		}
		for _, m := range list.Models {
			c.models = append(c.models, m.Slug)
		}
	})
	if c.merr != nil {
		return c.merr
	}
	if !slices.Contains(c.models, id) {
		return fmt.Errorf("model %q is not listed by codex debug models (%s); upgrade Codex or check the "+
			"models your account offers, or set [tiers.NAME] codex in .ness/ness.toml",
			id, strings.Join(c.models, ", "))
	}
	return nil
}

// NewSessionID is empty: Codex has no --session-id (ruling); the transcript is not bound.
func (*Client) NewSessionID() string { return "" }

// CurrentSession is empty for the same reason.
func (*Client) CurrentSession() string { return "" }

// tomlString is s as a TOML basic string; control characters become \uXXXX escapes, which %q
// does not produce in valid TOML.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote leaves a plain word alone and single-quotes anything else for the shell that
// runs the hook command.
func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// HookOverrides are the -c entries that run the ness hooks (R-HOOK-018). -c values are parsed
// as TOML, so each is an inline array of tables in the shape of ~/.codex/hooks.json.
func HookOverrides(ness string) []string {
	if ness == "" {
		ness = "ness"
	}
	entry := func(event string) string {
		return "[{hooks=[{type=\"command\",command=" + tomlString(shellQuote(ness)+" hook "+event) + "}]}]"
	}
	return []string{"-c", "hooks.SessionStart=" + entry("session-start"), "-c", "hooks.PreToolUse=" + entry("pre-tool")}
}

// Args builds the command line after `codex`: model and effort, trust for the worktree and no
// update prompt (R-SESS-113) and the hooks; the launch submits the prompt (R-SESS-025).
func (*Client) Args(s session.LaunchSpec) []string {
	args := []string{
		"-m", s.Model, "-c", fmt.Sprintf("model_reasoning_effort=%q", s.Effort),
		// The key is the bare top-level `projects`: Codex may split a -c key on every dot, and
		// every worktree path has one (.worktrees). The value is an inline table, parsed as TOML.
		"-c", "projects={" + tomlString(s.Dir) + "={trust_level=\"trusted\"}}",
		"-c", "check_for_update_on_startup=false",
		"--enable", "hooks", "--dangerously-bypass-hook-trust",
	}
	if s.SkipPermissions { // the pre-tool hook still runs and refuses (R-SESS-125)
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	}
	return append(args, HookOverrides(s.Ness)...)
}

// Transcript is not available for Codex in v1 (R-STAT-013).
func (*Client) Transcript(string, string, string) (session.Transcript, error) {
	return session.Transcript{}, errors.New("codex transcripts are not read in v1")
}
