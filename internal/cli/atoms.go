package cli

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/atom/cmdpolicy"
	"github.com/enea-scaccabarozzi/ness/internal/atom/codex"
	"github.com/enea-scaccabarozzi/ness/internal/atom/command"
	"github.com/enea-scaccabarozzi/ness/internal/atom/declared"
	"github.com/enea-scaccabarozzi/ness/internal/atom/github"
	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/atom/jev"
	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/atom/pr"
	"github.com/enea-scaccabarozzi/ness/internal/atom/rebase"
	"github.com/enea-scaccabarozzi/ness/internal/atom/slack"
	"github.com/enea-scaccabarozzi/ness/internal/atom/squash"
	"github.com/enea-scaccabarozzi/ness/internal/atom/telegram"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// The atoms are built here, the only place in cli that names an implementation; the commands
// see the seams.

func buildRuntime(d Deps) session.Runtime {
	return herdr.Runtime{Exec: d.exec(), Workspace: os.Getenv("HERDR_WORKSPACE_ID")}
}

func buildClient(d Deps, name string) session.Client {
	if name == "codex" {
		return &codex.Client{Exec: d.exec()}
	}
	return claude.Client{Exec: d.exec()}
}

// buildUsage reads when the client's usage limit resets, for the worker (R-WRK-043).
func buildUsage() func(now time.Time) (time.Time, bool) {
	return func(now time.Time) (time.Time, bool) { return claude.Usage(userHome(), now) }
}

// buildPolicy is the selector models.policy names; d.Policy wins when set (tests).
// The command selector runs in root.
func buildPolicy(d Deps, cfg *config.Config, root string) session.Policy {
	if d.Policy != nil {
		return d.Policy
	}
	if cfg.Models.Policy == "command" {
		m := cfg.Models
		return cmdpolicy.Policy{Command: m.Command, Dir: root, Timeout: m.CommandTimeout.Duration, Run: d.shell()}
	}
	fallback := declared.Policy{Kinds: cfg.Models.Kinds}
	if cfg.Models.Policy == "jev" {
		j := cfg.Models.Jev
		key, _, err := secretResolver(cfg.Project).Resolve(j.Key)
		if err != nil {
			key = "" // the policy reports "no key"; the error may name the reference, never the value
		}
		return jev.Policy{
			Endpoint: j.Endpoint, Model: j.Model, Key: key, Timeout: j.Timeout.Duration,
			Client: &http.Client{Transport: d.transport()},
			Kinds:  cfg.Models.Kinds, Weights: jev.DefaultWeights, Threshold: cfg.Models.RouterThreshold,
			Fallback: fallback,
		}
	}
	return fallback
}

// userHome is the user's home, where the clients keep their transcripts; "" when unknown, which
// means a relaunch never resumes a transcript (it starts fresh under the same id).
func userHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// skillRoots are the global skill directories of this user (R-SESS-006).
func skillRoots() []string {
	home := userHome()
	if home == "" {
		home = "/nonexistent"
	}
	return session.SkillRoots(home)
}

// buildForge is the gh CLI unless a test injected a forge.
func buildForge(d Deps) session.Forge {
	if d.Forge != nil {
		return d.Forge
	}
	return pr.GH{Exec: d.exec()}
}

// buildIntegrator is the integrator of mode; subject is the squash commit's (R-SESS-140).
func buildIntegrator(d Deps, mode, subject string) session.Integrator {
	switch mode {
	case "squash":
		return squash.Integrator{Subject: subject}
	case "rebase":
		return rebase.Integrator{}
	case "pr":
		return pr.Integrator{Forge: buildForge(d)}
	case "merge":
		return merge.Integrator{}
	default: // the config's default (R-CFG-060)
		return squash.Integrator{Subject: subject}
	}
}

// buildChannel is the atom of [notify.channels.NAME] with every reference of its keys resolved
// (R-NTF-060, R-CFG-080). An unknown name lists the configured ones (R-NTF-002). needToken false
// (a dry run, which makes no request) leaves a missing reference empty instead of refusing.
func buildChannel(d Deps, cfg *config.Config, root, name string, needToken bool) (notify.Channel, error) {
	ch, ok := cfg.Notify.Channels[name]
	if !ok {
		names := strings.Join(slices.Sorted(maps.Keys(cfg.Notify.Channels)), ", ")
		return nil, fmt.Errorf("%w %s; configured: %s", notify.ErrNoChannel, name, cmp.Or(names, "none"))
	}
	r := secretResolver(cfg.Project)
	var secrets []string
	ch, err := ch.Resolve(func(v string) (string, error) {
		val, ref, err := r.Resolve(v)
		if ref && val != "" {
			secrets = append(secrets, val)
		}
		if !needToken {
			err = nil
		}
		return val, err
	})
	if err != nil {
		return nil, refuse("channel %s: %v", name, err)
	}
	return masked{Channel: kindChannel(d, root, ch), secrets: secrets}, nil
}

func kindChannel(d Deps, root string, ch config.Channel) notify.Channel {
	switch ch.Kind {
	case "slack":
		return slack.Channel{Token: ch.Token, ChannelID: ch.ChannelID, HTTP: d.httpClient()}
	case "telegram":
		return telegram.Channel{Token: ch.Token, ChatID: ch.ChatID, HTTP: d.httpClient()}
	case "github":
		return github.Channel{Token: ch.Token, Repo: ch.Repo, HTTP: d.httpClient()}
	}
	return command.Channel{Command: ch.Command, Dir: root, Run: d.shell(), Secret: ch.Token}
}

// masked is a channel whose post errors never carry a value it got from a reference (R-NTF-061).
type masked struct {
	notify.Channel
	secrets []string
}

// Post implements notify.Channel.
func (m masked) Post(p notify.Part, thread string) (string, error) {
	id, err := m.Channel.Post(p, thread)
	if err != nil {
		err = errors.New(probe.Redact(err.Error(), m.secrets))
	}
	return id, err
}

// secretResolver resolves the references of the project's configuration: env: from the
// process environment, secret: from the user's secrets file of the project (R-CFG-082).
func secretResolver(project string) *config.SecretResolver {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "/nonexistent"
	}
	return &config.SecretResolver{Path: config.SecretsPath(dir, project), Getenv: os.Getenv}
}

// buildPoster composes the poster over the project's channels and store.
func buildPoster(d Deps, cfg *config.Config, root string, s *store.Store, needToken bool) notify.Poster {
	return notify.Poster{
		Channel: func(name string) (notify.Channel, error) { return buildChannel(d, cfg, root, name, needToken) },
		Store:   s, Now: d.Now,
	}
}

// transcriptPath is where Claude Code keeps the transcript of a session: the project directory
// of its worktree (or the primary checkout) under the user's home (R-STAT-002).
func transcriptPath(primary string, sess store.Session) string {
	return claude.TranscriptPath(userHome(), transcriptDir(primary, sess), sess.ClientSession)
}

// transcriptDir is the directory the session's client conversation runs in: its worktree, or, for a
// root adopted with --here, whichever of the primary checkout and its worktree holds the
// conversation's transcript, since the adopted conversation may have started in either
// (R-SESS-114). Every transcript reader (stats, status, worker activity, close --authorized)
// routes through here.
// ponytail: assumes the adopted conversation started at the checkout or worktree root, not in a
// subdirectory; record the start directory at adoption if that case shows up.
func transcriptDir(primary string, sess store.Session) string {
	if sess.Worktree == "" {
		return primary
	}
	wt := filepath.Join(primary, sess.Worktree)
	if !sess.Adopted {
		return wt
	}
	if _, err := os.Stat(claude.TranscriptPath(userHome(), wt, sess.ClientSession)); err == nil {
		return wt
	}
	return primary
}
