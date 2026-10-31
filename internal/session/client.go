package session

import "time"

// LaunchSpec is one client launch: the worktree, the resolved model and effort, the client
// session id (R-SESS-027) and, on resume, the stored one (R-SESS-047). The first prompt is not
// in it: the launch submits it once the client is ready (R-SESS-025). Ness is the ness
// executable the Codex hooks run; "" means "ness".
type LaunchSpec struct {
	Dir, Model, Effort, SessionID, FallbackModel, Ness string
	// Name is the session's name in the client (R-SESS-120): <project>/<title>.
	Name   string
	Resume bool
	// SkipPermissions starts the client in its skip-permissions mode (R-SESS-125).
	SkipPermissions bool
}

// ClientName is a session's name in its client: the project, then the tab title (R-SESS-120).
func ClientName(project, title string) string { return project + "/" + title }

// Transcript is what a client's transcript tells status: the context size of the last turn and
// when the session last wrote.
type Transcript struct {
	Context    int
	LastActive time.Time // the last real work entry; the file's mtime when the transcript has none
}

// Client is the coding-agent seam (R-SESS-111): Claude Code, Codex (experimental) and the
// contract they share. Args is pure; Version, ResolveModel and Transcript touch the outside.
type Client interface {
	Name() string
	Tested() string
	Version() (string, error)
	Efforts() []string
	ResolveModel(id string) error
	NewSessionID() string
	Args(LaunchSpec) []string
	CurrentSession() string
	Transcript(home, dir, id string) (Transcript, error)
}
