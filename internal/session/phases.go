package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Phases are the launch phases in order (R-SESS-044), persisted before each step's side effect
// (R-SESS-031). Names are frozen by TestPhasesGolden.
var Phases = []string{
	store.PhaseRegistered, "creating-worktree", "linking-files", "writing-brief", "opening-tab",
	"starting-agent", "sending-brief", "waiting-ready", store.PhaseReady,
}

// NextPhases returns the phases to run after the stored phase: the stored step is re-run (its
// side effect may be half done; every step is idempotent) and the ones after it. An unknown
// phase is treated as registered.
func NextPhases(phase string) []string {
	i := slices.Index(Phases, phase)
	if i <= 0 {
		return Phases[1:]
	}
	return Phases[i:]
}

// PromptPending reports a launch whose client is up but has not been given its first prompt: a
// ready mark written then (the hook runs at the client's start, or once a trust dialog is
// accepted after a stall) does not make the session ready (R-SESS-025).
func PromptPending(s store.Session) bool {
	i := slices.Index(Phases, s.Phase)
	return i >= slices.Index(Phases, "starting-agent") && i <= slices.Index(Phases, "sending-brief")
}

// Done reports a quick root whose work is integrated: nothing is left for it to do, so it shows
// done and the worker stops asking the operator to move; it still closes only with the operator's
// words (R-SESS-157).
func Done(s store.Session) bool { return s.Size == "quick" && !s.IntegratedAt.IsZero() }

// Shown is the displayed phase (R-SESS-033, R-SESS-085): ready once the mark exists and the
// prompt went in (done for an integrated quick root), stalled launch when the phase is older than
// the ready timeout, else the phase.
func Shown(s store.Session, now time.Time, timeout time.Duration) string {
	if !s.ReadyAt.IsZero() && !PromptPending(s) || s.Phase == store.PhaseReady {
		if Done(s) {
			return "done"
		}
		return store.PhaseReady
	}
	if now.Sub(s.PhaseAt) >= timeout {
		return "stalled launch"
	}
	return s.Phase
}

// ScopeRe is the scope syntax (R-SESS-043).
var ScopeRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// IDRe is the session id syntax (R-SESS-042).
var IDRe = regexp.MustCompile(`^s-[0-9a-f]{12}$`)

// NewID returns "s-" plus 12 hex characters (R-SESS-042).
func NewID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand cannot fail on supported platforms
	}
	return "s-" + hex.EncodeToString(b[:])
}

// Prompt is the one-line prompt a launch submits once the client is ready (R-SESS-025).
func Prompt(id, scope, record string) string {
	return fmt.Sprintf("You are ness session %s (%s). Read %s/brief.md, then load ness-entry.", id, scope, record)
}
