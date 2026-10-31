package cli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/atom/jev"
	"github.com/enea-scaccabarozzi/ness/internal/hook"
	"github.com/enea-scaccabarozzi/ness/internal/ladder"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

type pickOpts struct {
	kind, role, agent    string
	json, explain, stdin bool
}

func newPickCmd(d Deps) *cobra.Command {
	var o pickOpts
	cmd := &cobra.Command{
		Use: "pick",
		Example: "  # the rung for a search subagent\n  ness pick --kind search\n" +
			"  # a named agent's rung, as JSON\n  ness pick --agent ness-design --json\n" +
			"  # the ladder, base and ceiling\n  ness pick\n" +
			"  # judge a task text (policy jev or command)\n  ness pick --stdin --kind implement < task.md",
		Short: "Show the rung the selector picks for a task, or the ladder",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runPick(d, o) },
	}
	f := cmd.Flags()
	f.StringVar(&o.kind, "kind", "", "work `NAME` (search, implement, design, ...)")
	f.StringVar(&o.role, "role", "subagent", "who runs the task: subagent or child")
	f.StringVar(&o.agent, "agent", "", "named subagent `TYPE` (implies its kind)")
	f.BoolVar(&o.json, "json", false, "print the pick as JSON")
	f.BoolVar(&o.explain, "explain", false, "also print the Jev answers (policy jev)")
	f.BoolVar(&o.stdin, "stdin", false, "read the task text from stdin")
	return cmd
}

// pickJSON is the --json shape of a pick (R-MOD-046).
type pickJSON struct {
	Schema     int          `json:"schema"`
	Rung       string       `json:"rung"`
	Model      string       `json:"model"`
	Effort     string       `json:"effort"`
	Index      int          `json:"index"`
	Confidence float64      `json:"confidence"`
	Reason     string       `json:"reason"`
	Fallback   *bool        `json:"fallback,omitempty"`
	Answers    *answersJSON `json:"answers,omitempty"`
}

// answersJSON is jev.Answers in snake_case, for --explain (R-MOD-064).
type answersJSON struct {
	Kind           string  `json:"kind"`
	KindConfidence float64 `json:"kind_confidence"`
	Depth          float64 `json:"depth"`
	Ambiguity      float64 `json:"ambiguity"`
	Blast          float64 `json:"blast"`
	Novelty        float64 `json:"novelty"`
	Breadth        float64 `json:"breadth"`
	Check          float64 `json:"check"`
	Mechanical     float64 `json:"mechanical"`
	Design         float64 `json:"design"`
	Judge          float64 `json:"judge"`
	Chain          float64 `json:"chain"`
}

// explainer is a selector that also returns its answers (the jev one).
type explainer interface {
	Explain(session.Brief) (jev.Answers, session.Verdict, error)
}

// capturing runs an explainer as a policy, keeping the answers of its last call.
type capturing struct {
	e explainer
	a *jev.Answers
}

// explaining wraps pol to capture its answers when explain is set and pol is an explainer;
// the answers are nil otherwise.
func explaining(pol session.Policy, explain bool) (session.Policy, *jev.Answers) {
	e, isJev := pol.(explainer)
	if !explain || !isJev {
		return pol, nil
	}
	a := new(jev.Answers)
	return capturing{e, a}, a
}

// Choose implements session.Policy.
func (c capturing) Choose(b session.Brief) (session.Verdict, error) {
	a, v, err := c.e.Explain(b)
	*c.a = a
	return v, err
}

// runPick runs the configured selector on the task text read from stdin under --stdin
// (R-MOD-046); with no kind, agent or text it prints the ladder.
func runPick(d Deps, o pickOpts) error {
	if o.role != "subagent" && o.role != "child" {
		return usage("--role %s: accepted values are subagent, child", o.role)
	}
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return err
	}
	o.kind = hook.CanonicalKind(o.kind)
	text, err := pickText(d, o.stdin)
	if err != nil {
		return err
	}
	if o.kind == "" && o.agent == "" && strings.TrimSpace(text) == "" {
		return printLadder(d, o.json, ladder.New(cfg, "claude"), ladder.New(cfg, "codex"))
	}
	b := session.Brief{Text: text, Role: o.role, Client: cfg.Client, Kind: o.kind, AgentType: o.agent}
	pol, answers := explaining(buildPolicy(d, cfg, p.root), o.explain)
	ch, err := session.Pick(pol, b, ladder.New(cfg, cfg.Client), cfg.Models.RouterThreshold)
	if errors.Is(err, session.ErrNoKind) {
		return refuse("%v", err)
	}
	return printPick(d, o, cfg.Client, ch, answers)
}

// pickText is the task text on stdin, read only when asked: a stdin that never closes (a Bash
// tool call with a heredoc) would hang the pick (R-MOD-046).
func pickText(d Deps, asked bool) (string, error) {
	if !asked || d.Stdin == nil {
		return "", nil
	}
	b, err := io.ReadAll(d.Stdin)
	return string(b), err
}

// printPick prints ch as one line (and the Agent call line for a Claude subagent) or as JSON.
// a, set under --explain with policy jev, holds Jev's answers (Kind "" when Jev gave none).
func printPick(d Deps, o pickOpts, client string, ch session.Choice, a *jev.Answers) error {
	out := pickJSON{
		Schema: 1, Rung: ch.Tier, Model: ch.Model, Effort: ch.Effort, Index: ch.Index,
		Confidence: ch.Confidence, Reason: ch.Reason,
	}
	if a != nil {
		out.Fallback = &a.Fallback
		if a.Kind != "" { // kept on a low-confidence fallback for calibration (R-MOD-064)
			out.Answers = &answersJSON{
				a.Kind, a.KindConfidence, a.Depth, a.Ambiguity, a.Blast, a.Novelty,
				a.Breadth, a.Check, a.Mechanical, a.Design, a.Judge, a.Chain,
			}
		}
	}
	if o.json {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	say(d.Stdout, "%-9s %-14s %s", ch.Tier, ch.Model+"/"+ch.Effort, ch.Reason)
	if o.role == "subagent" && client == "claude" { // the line is Claude Code's Agent call
		say(d.Stdout, "model: %s  effort: %s", ch.Model, ch.Effort)
	}
	if x := out.Answers; x != nil {
		say(d.Stdout, "kind %s", x.Kind)
		names := []string{
			"kind_confidence", "depth", "ambiguity", "blast", "novelty", "breadth",
			"check", "mechanical", "design", "judge", "chain",
		}
		vals := []float64{
			x.KindConfidence, x.Depth, x.Ambiguity, x.Blast, x.Novelty, x.Breadth,
			x.Check, x.Mechanical, x.Design, x.Judge, x.Chain,
		}
		for i, n := range names {
			say(d.Stdout, "%s %g", n, vals[i])
		}
	}
	return nil
}

// ladderJSON is the --json shape of the ladder listing (R-MOD-046).
type ladderJSON struct {
	Schema  int        `json:"schema"`
	Base    string     `json:"base"`
	Ceiling string     `json:"ceiling"`
	Rungs   []rungJSON `json:"rungs"`
}

type rungJSON struct {
	Name   string `json:"name"`
	Claude string `json:"claude"`
	Codex  string `json:"codex"`
}

// printLadder lists every rung with its claude and codex model/effort, marking base and ceiling.
func printLadder(d Deps, asJSON bool, claude, codex ladder.Ladder) error {
	out := ladderJSON{Schema: 1, Base: claude.Rungs[claude.Base].Name, Ceiling: claude.Rungs[claude.Ceiling].Name}
	for i, r := range claude.Rungs {
		c := codex.Rungs[i]
		out.Rungs = append(out.Rungs, rungJSON{r.Name, r.Model + "/" + r.Effort, c.Model + "/" + c.Effort})
	}
	if asJSON {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	for i, r := range out.Rungs {
		mark := ""
		if i == claude.Base {
			mark += "  base"
		}
		if i == claude.Ceiling {
			mark += "  ceiling"
		}
		say(d.Stdout, "%-9s %-14s %-20s%s", r.Name, r.Claude, r.Codex, mark)
	}
	return nil
}
