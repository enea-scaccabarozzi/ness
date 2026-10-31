package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

// check is one doctor line (R-DOC-020): Status is ok, warn or fail; Fix is a command, set on
// every fail and never run (R-DOC-002).
type check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

func ok(name, detail string) check { return check{Name: name, Status: "ok", Detail: detail} }

func warn(name, detail, fix string) check {
	return check{Name: name, Status: "warn", Detail: detail, Fix: fix}
}

func fail(name, detail, fix string) check {
	return check{Name: name, Status: "fail", Detail: detail, Fix: fix}
}

// doctorEnv is what the checks read. Nothing in it writes.
type doctorEnv struct {
	d       Deps
	p       *project
	cfg     *config.Config
	primary string
	now     time.Time
	// s is the store opened read-only (R-DOC-001); nil when there is none yet (storeErr is then
	// fs.ErrNotExist) or when it cannot be read (a newer schema).
	s        *store.Store
	storeErr error
}

// viaStore runs a check that reads the store: no store yet is ok, an unreadable one fails.
func (e *doctorEnv) viaStore(name string, fn func(*store.Store) check) check {
	switch {
	case errors.Is(e.storeErr, fs.ErrNotExist):
		return ok(name, "no store yet")
	case errors.Is(e.storeErr, store.ErrOlder):
		return warn(name, e.storeErr.Error(), "")
	case e.storeErr != nil:
		return fail(name, e.storeErr.Error(), "upgrade ness, or run ness doctor from a matching binary")
	}
	return fn(e.s)
}

type checker func(*doctorEnv) check

// checks run in this order; each one completes whatever the others found (R-DOC-017).
var checks = []checker{
	checkBinary, checkHooksBinary, checkWorkerBinary, checkHerdrVersion, checkClientVersion,
	checkTierModels, checkShadowing, checkProfileTools,
	checkWorker, checkWorkerClients, checkSystemd, checkSyncDrift, checkDeadRoutes, checkGonePanes,
	checkStrayWorktrees, checkProbes, checkOperatorNotify, checkSecrets, checkJev,
}

type doctorJSON struct {
	Schema  int     `json:"schema"`
	Version string  `json:"version"`
	Checks  []check `json:"checks"`
}

func newDoctorCmd(d Deps) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "doctor",
		Example: "  # every check, a fix under each failure\n  ness doctor\n" +
			"  # the same, for scripts\n  ness doctor --json",
		Short: "Check this installation and project, read-only; print a fix for every failure",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runDoctor(d, asJSON) },
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print one JSON object")
	return cmd
}

// runDoctor prints the version line first, so a refusal outside a project still says which
// binary answered (R-DOC-023); --json carries the version in the object and prints the line
// only when it refuses. It runs every check (R-DOC-001: nothing here writes; the store is opened
// read-only, never created). The pin is not checked on load: checkBinary reports a mismatch
// (R-DOC-004).
func runDoctor(d Deps, asJSON bool) error {
	rich := d.rich() && !asJSON
	switch {
	case rich:
		t := d.theme()
		say(d.out(), "%s %s %s", t.Badge("ness "+d.fullVersion(), "accent"), t.Dim.Render("at"), executable())
	case !asJSON:
		say(d.Stdout, "ness %s at %s", d.fullVersion(), executable())
	}
	env, err := newDoctorEnv(d)
	if err != nil {
		if asJSON {
			say(d.Stdout, "ness %s at %s", d.fullVersion(), executable())
		}
		return err
	}
	if env.s != nil {
		defer func() { _ = env.s.Close() }() //nolint:errcheck // read-only handle
	}
	results, err := checkAndPrint(d, env, rich, asJSON)
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Status == "fail" {
			return exitCode(ExitProblems)
		}
	}
	return nil
}

// checkAndPrint runs the checks, behind a spinner when rich, and prints them.
func checkAndPrint(d Deps, env *doctorEnv, rich, asJSON bool) ([]check, error) {
	if !rich {
		results := runChecks(env)
		return results, printResults(d, asJSON, results)
	}
	var results []check
	if err := d.theme().Spin(d.Stderr, fmt.Sprintf("running %d checks", len(checks)),
		func() { results = runChecks(env) }); err != nil {
		return nil, &exitError{code: ExitProblems, err: fmt.Errorf("doctor interrupted: %w", err)}
	}
	printRichResults(d, results)
	return results, nil
}

func newDoctorEnv(d Deps) (*doctorEnv, error) {
	p, err := locateProject()
	if err != nil {
		return nil, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return nil, err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return nil, err
	}
	env := &doctorEnv{d: d, p: p, cfg: cfg, primary: primary, now: d.Now()}
	path, err := store.Locate(p.root)
	if err != nil {
		return nil, refuse("%v", err)
	}
	env.s, env.storeErr = store.OpenReadOnly(path)
	return env, nil
}

func runChecks(env *doctorEnv) []check {
	results := make([]check, 0, len(checks))
	for _, c := range checks {
		results = append(results, c(env))
	}
	return results
}

func printResults(d Deps, asJSON bool, results []check) error {
	if asJSON {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(doctorJSON{Schema: 1, Version: d.fullVersion(), Checks: results})
	}
	for _, r := range results {
		line := r.Status + "\t" + r.Name + "\t" + r.Detail
		if r.Fix != "" {
			line += "\tfix: " + r.Fix
		}
		say(d.Stdout, "%s", line)
	}
	return nil
}

// printRichResults is doctor's rich rendering: a glyph per check, the fix under it, and the
// counts (R-UI-022).
func printRichResults(d Deps, results []check) {
	t, w := d.theme(), d.out()
	pad, counts := 0, map[string]int{}
	for _, r := range results {
		pad = max(pad, len(r.Name))
	}
	for _, r := range results {
		counts[r.Status]++
		glyph := map[string]string{
			"ok":   t.Glyph(ui.GlyphOK, "ok"),
			"warn": t.Glyph(ui.GlyphWarn, "warn"),
			"fail": t.Glyph(ui.GlyphFail, "fail"),
		}[r.Status]
		say(w, "%s %s  %s", glyph, t.Bold.Render(fmt.Sprintf("%-*s", pad, r.Name)), r.Detail)
		if r.Fix != "" {
			say(w, "%s%s", strings.Repeat(" ", pad+2), t.Dim.Render("↳ fix: ")+t.Accent.Render(r.Fix))
		}
	}
	say(w, "")
	say(w, "%s · %s · %s", t.OK.Render(fmt.Sprintf("%d ok", counts["ok"])),
		t.Warn.Render(fmt.Sprintf("%d warn", counts["warn"])), t.Fail.Render(fmt.Sprintf("%d fail", counts["fail"])))
}
