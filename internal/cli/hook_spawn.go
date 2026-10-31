package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/atom/declared"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/hook"
	"github.com/enea-scaccabarozzi/ness/internal/ladder"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// spawnGuard judges a subagent call against the selector's rung and records the decision
// (R-HOOK-034..037). A failed record is reported on stderr and never changes the verdict.
func spawnGuard(d Deps, b bound, cfg *config.Config, p hook.Payload) error {
	sp, ok := hook.ParseSpawn(p)
	if !ok {
		return nil
	}
	var in struct {
		Prompt string `json:"prompt"`
	}
	if json.Unmarshal(p.ToolInput, &in) != nil {
		in.Prompt = "" // a prompt that is not a string is no task text
	}
	rec, reason := judgeSpawn(sp, in.Prompt, cfg, buildPolicy(d, cfg, b.p.root))
	rec.Session, rec.At = b.c.id, d.Now()
	if err := b.s.AddSpawn(rec); err != nil {
		printErr(d.Stderr, err)
	}
	if reason != "" {
		return block(reason)
	}
	return nil
}

// judgeSpawn decides one subagent call: the record to store and the refusal, "" to allow.
func judgeSpawn(
	sp hook.Spawn, prompt string, cfg *config.Config, pol session.Policy,
) (out store.Spawn, refusal string) {
	out = store.Spawn{AgentType: sp.AgentType, Kind: sp.Kind}
	if sp.AgentType == "fork" {
		out.Verdict = "fork"
		return out, ""
	}
	enforce := cfg.Models.Enforce
	refuse := func(msg string) (store.Spawn, string) {
		out.Verdict, out.Reason = "refused", msg
		if enforce == "log" {
			out.Verdict = "would-refuse"
			return out, ""
		}
		return out, msg + rule.Ref(rule.ModelEffort)
	}
	l := ladder.New(cfg, "claude") // only Claude sends Agent/Task, whatever the project's client
	req, found := requestedRung(sp, cfg, l)
	if !found {
		out.Requested = pairName(sp)
		return refuse(offLadder(sp, l))
	}
	out.Requested = l.Rungs[req].Name
	if req > l.Ceiling {
		c := l.Rungs[l.Ceiling]
		return refuse(fmt.Sprintf("subagent rung %s is above the ceiling %s (%s/%s); no escalate goes past it",
			out.Requested, c.Name, c.Model, c.Effort))
	}
	brief := session.Brief{Text: prompt, Role: "subagent", Client: "claude", Kind: sp.Kind, AgentType: sp.AgentType}
	ch, err := session.Pick(pol, brief, l, cfg.Models.RouterThreshold)
	if errors.Is(err, session.ErrNoKind) {
		return refuse(fmt.Sprintf("start the subagent prompt with a line kind: <name>; %v", err))
	}
	out.Recommended = ch.Tier
	verdict, block := hook.Judge(req, ch.Index, sp.Escalate, enforce)
	out.Verdict, out.Reason = verdict, ch.Reason
	if verdict == "escalated" {
		out.Reason = sp.Escalate
	}
	if !block {
		return out, ""
	}
	r := l.Rungs[req]
	msg := fmt.Sprintf("subagent rung %s (%s/%s) is above the recommended %s (%s/%s, %s): pass model %s "+
		"and effort %s, or add a line `escalate: <reason>` to go one rung above it",
		r.Name, r.Model, r.Effort, ch.Tier, ch.Model, ch.Effort, ch.Reason, ch.Model, ch.Effort)
	if sp.Escalate != "" {
		msg = "escalate buys one rung only; " + msg
	}
	return out, msg + rule.Ref(rule.ModelEffort)
}

// requestedRung is the rung the call asks for: its explicit pair, or a named agent's own rung.
// A model without an effort takes the effort of the kind's rung when that rung has the model
// (R-HOOK-034): agents copied pick's effort into the prompt instead of the call.
func requestedRung(sp hook.Spawn, cfg *config.Config, l ladder.Ladder) (int, bool) {
	if sp.Model != "" && sp.Effort == "" {
		if i, ok := kindRung(sp, cfg, l); ok && l.Rungs[i].Model == sp.Model {
			return i, true
		}
	}
	if sp.Model != "" || sp.Effort != "" {
		return l.Find(sp.Model, sp.Effort)
	}
	return namedRung(sp.AgentType, cfg, l)
}

// kindRung is the rung the call's kind, else its named agent's kind, maps to; false for neither.
func kindRung(sp hook.Spawn, cfg *config.Config, l ladder.Ladder) (int, bool) {
	k := sp.Kind
	if k == "" {
		k = declared.NamedKinds()[sp.AgentType]
	}
	off, ok := cfg.Models.Kinds[k]
	if !ok {
		return -1, false
	}
	r, _ := l.At(off)
	return l.Index(r.Name)
}

// namedRung is the rung a named ness agent is rendered at; false for any other agent type.
func namedRung(agentType string, cfg *config.Config, l ladder.Ladder) (int, bool) {
	if _, ok := declared.NamedKinds()[agentType]; !ok {
		return -1, false
	}
	return kindRung(hook.Spawn{AgentType: agentType}, cfg, l)
}

// pairName is the call's explicit model/effort, "" when it gives neither.
func pairName(sp hook.Spawn) string {
	if sp.Model == "" && sp.Effort == "" {
		return ""
	}
	return sp.Model + "/" + sp.Effort
}

// offLadder is the refusal for a call whose model and effort are not one rung of the ladder.
func offLadder(sp hook.Spawn, l ladder.Ladder) string {
	var b strings.Builder
	if sp.Model == "" && sp.Effort == "" {
		b.WriteString("a subagent that is not a named ness agent needs a model and effort from the ladder")
	} else {
		fmt.Fprintf(&b, "model %q with effort %q is not a rung of the ladder", sp.Model, sp.Effort)
	}
	b.WriteString("; pass both, as ness pick --kind NAME prints them:")
	for _, r := range l.Rungs {
		fmt.Fprintf(&b, "\n  %-9s %s/%s", r.Name, r.Model, r.Effort)
	}
	return b.String()
}
