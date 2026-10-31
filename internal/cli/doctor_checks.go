package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/atom/systemd"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

// checkBinary: the running binary against the project pin, with its path (R-DOC-004).
func checkBinary(e *doctorEnv) check {
	pin := e.p.file.Pin()
	detail := fmt.Sprintf("%s at %s, pin %s", e.d.Version, executable(), pin)
	if err := config.CheckPin(pin, e.d.Version); err != nil {
		return fail("binary", detail+": "+err.Error(),
			"install ness "+pin+".x (mise use ness@"+pin+") or change the pin in .ness/ness.toml")
	}
	return ok("binary", detail)
}

// checkHooksBinary: the ness the client hooks resolve through PATH (R-DOC-005).
func checkHooksBinary(_ *doctorEnv) check {
	me := realPath(executable())
	found, err := exec.LookPath("ness")
	if err != nil {
		return fail("hooks-binary", "ness is not on PATH; the client hooks resolve nothing", "put "+me+" on PATH")
	}
	if realPath(found) != me {
		return fail("hooks-binary", "hooks resolve "+found+", this binary is "+me, "put "+me+" first on PATH")
	}
	return ok("hooks-binary", "hooks resolve "+found)
}

// checkWorkerBinary: the worker unit's ExecStart against this binary (R-DOC-005, §16 #3).
func checkWorkerBinary(e *doctorEnv) check {
	unit := unitPath(e.cfg.Project)
	f, err := os.Open(unit) //nolint:gosec // the unit of this project
	if err != nil {
		return warn("worker-binary", "no unit at "+unit+" (worker not installed)", "ness worker install")
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // read-only file
	me := realPath(executable())
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		start, found := strings.CutPrefix(sc.Text(), "ExecStart=")
		if !found {
			continue
		}
		prog := systemd.FirstWord(start)
		if prog == "" {
			break
		}
		if realPath(prog) != me {
			return fail("worker-binary", "unit ExecStart runs "+prog+", this binary is "+me, "ness worker install")
		}
		return ok("worker-binary", "unit runs "+prog)
	}
	return fail("worker-binary", "unit "+unit+" has no ExecStart", "ness worker install")
}

func unitPath(project string) string {
	return systemd.Supervisor{Home: userHome()}.UnitPath(project)
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// checkHerdrVersion (R-DOC-006).
func checkHerdrVersion(e *doctorEnv) check {
	v, err := buildRuntime(e.d).Version()
	if errors.Is(err, session.ErrNotInstalled) {
		return fail("herdr-version", "herdr is not installed",
			"install herdr "+session.TestedHerdr+" (https://herdr.dev)")
	}
	if err != nil {
		return fail("herdr-version", err.Error(), "herdr --version")
	}
	if verr := session.CheckVersion("herdr", v, session.TestedHerdr); verr != nil {
		return fail("herdr-version", v+" is not supported; tested "+session.TestedHerdr, "herdr update")
	}
	return ok("herdr-version", v)
}

// checkClientVersion (R-DOC-006).
func checkClientVersion(e *doctorEnv) check {
	c := buildClient(e.d, e.cfg.Client)
	v, err := c.Version()
	if errors.Is(err, session.ErrNotInstalled) {
		return fail("client-version", c.Name()+" is not installed", "install "+c.Name()+" "+c.Tested())
	}
	if err != nil {
		return fail("client-version", err.Error(), c.Name()+" --version")
	}
	if verr := session.CheckVersion(c.Name(), v, c.Tested()); verr != nil {
		return fail("client-version", c.Name()+" "+v+" is not supported; tested "+c.Tested(), c.Name()+" update")
	}
	return ok("client-version", c.Name()+" "+v)
}

// checkTierModels: every tier row resolves for the configured client (R-DOC-007).
func checkTierModels(e *doctorEnv) check {
	c := buildClient(e.d, e.cfg.Client)
	var bad []string
	for _, name := range slices.Sorted(maps.Keys(e.cfg.Tiers)) {
		model, _ := session.TierEffort(e.cfg.Tiers[name], c.Name())
		if err := c.ResolveModel(model); err != nil {
			bad = append(bad, fmt.Sprintf("tier %s (%s = %q): %v", name, c.Name(), model, err))
		}
	}
	if len(bad) > 0 {
		return fail("tier-models", strings.Join(bad, "; "), "edit [tiers.<name>] in .ness/ness.toml")
	}
	return ok("tier-models", fmt.Sprintf("%d tiers resolve for %s", len(e.cfg.Tiers), c.Name()))
}

// checkShadowing lists every shadowing pair, not just the first (R-DOC-008).
func checkShadowing(e *doctorEnv) check {
	found, err := session.Shadowed(filepath.Join(e.primary, ".agents", "skills"), skillRoots())
	if err != nil {
		return fail("skill-shadowing", err.Error(), "fix the permissions of .agents/skills")
	}
	if len(found) == 0 {
		return ok("skill-shadowing", "no global skill shadows a project skill")
	}
	pairs := make([]string, len(found))
	for i, s := range found {
		pairs[i] = s.Global + " shadows " + s.Project
	}
	return fail("skill-shadowing", strings.Join(pairs, "; "), "rename or remove the global skill")
}

// checkProfileTools: requires_tools of the applied profiles on PATH (R-DOC-009).
func checkProfileTools(e *doctorEnv) check {
	res, err := loadProfiles(e.d, e.p, e.cfg.Profiles)
	if err != nil {
		return fail("profile-tools", err.Error(), "fix the profile, then ness sync")
	}
	var missing []string
	for _, tool := range slices.Sorted(maps.Keys(res.Tools)) {
		if _, lerr := exec.LookPath(tool); lerr != nil {
			missing = append(missing, tool+" (profile "+res.Tools[tool]+")")
		}
	}
	if len(missing) > 0 {
		return fail("profile-tools", "not on PATH: "+strings.Join(missing, ", "), "install them (mise use ...)")
	}
	return ok("profile-tools", fmt.Sprintf("%d required tools present", len(res.Tools)))
}

// checkWorker: running, same version, heartbeat age with the 60 s line (R-DOC-010, R-DOC-022).
// A project without a store has no heartbeat either.
func checkWorker(e *doctorEnv) check {
	restart := "systemctl --user restart ness-" + e.cfg.Project
	var hb store.Heartbeat
	err := e.storeErr
	if err == nil {
		hb, err = e.s.Heartbeat()
	}
	switch {
	case errors.Is(err, store.ErrOlder):
		return warn("worker", err.Error(), "")
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, store.ErrNoHeartbeat):
		return fail("worker", "not running (no heartbeat)", "ness worker install")
	case err != nil:
		return fail("worker", err.Error(), restart)
	}
	age := int(e.now.Sub(hb.At).Seconds())
	if age > 60 {
		return fail("worker", fmt.Sprintf("heartbeat %ds ago (older than 60 s): not running", age), restart)
	}
	if hb.Version != e.d.fullVersion() {
		return fail("worker", fmt.Sprintf("running %s, this binary is %s", hb.Version, e.d.fullVersion()), restart)
	}
	return ok("worker", fmt.Sprintf("running %s, heartbeat %ds ago", hb.Version, age))
}

// checkWorkerClients: a client the worker cannot deliver to (no screen reader), when it is the
// configured client or an open session runs on it, is a warning naming the client and the
// sessions it leaves unwoken; their mail waits until they read it (R-WRK-091).
func checkWorkerClients(e *doctorEnv) check {
	return e.viaStore("worker-clients", func(s *store.Store) check {
		open, err := s.Sessions(false)
		if err != nil {
			return fail("worker-clients", err.Error(), "ness status")
		}
		supported := workerClients(e.d)
		names := strings.Join(slices.Sorted(maps.Keys(supported)), ", ")
		unwoken := map[string][]string{}
		if _, ok := supported[e.cfg.Client]; !ok {
			unwoken[e.cfg.Client] = nil
		}
		for _, sess := range open {
			if _, ok := supported[sess.Client]; !ok {
				unwoken[sess.Client] = append(unwoken[sess.Client], sess.ID)
			}
		}
		if len(unwoken) == 0 {
			return ok("worker-clients", "the worker delivers to "+names)
		}
		var parts []string
		for _, name := range slices.Sorted(maps.Keys(unwoken)) {
			part := name + " has no worker support"
			if ids := unwoken[name]; len(ids) > 0 {
				part += ", not woken: " + strings.Join(ids, ", ")
			}
			parts = append(parts, part)
		}
		return warn("worker-clients", strings.Join(parts, "; "),
			"run sessions on "+names+", or read their mail with ness inbox")
	})
}

// checkSystemd: a systemd user session for the worker unit; "degraded" still runs units, so it
// is a warning, not a failure (R-DOC-016, R-DOC-021).
func checkSystemd(e *doctorEnv) check {
	out, err := e.d.exec()("", "systemctl", "--user", "is-system-running")
	state := strings.TrimSpace(string(out))
	if state == "degraded" {
		return warn("systemd", "user session degraded (a unit failed)", "systemctl --user --failed")
	}
	if err != nil {
		msg := strings.Join(strings.Fields(err.Error()), " ")
		return fail("systemd", "no systemd user session: "+msg, "ness worker run (in a terminal, without systemd)")
	}
	return ok("systemd", "user session "+state)
}

// checkSyncDrift: what ness sync --check would report, without writing (R-DOC-011).
func checkSyncDrift(e *doctorEnv) check {
	root, err := os.OpenRoot(e.primary)
	if err != nil {
		return fail("sync-drift", err.Error(), "ness sync --check")
	}
	defer func() { _ = root.Close() }() //nolint:errcheck // read-only root
	plan, err := planSync(e.d, e.p, root)
	if err != nil {
		return fail("sync-drift", err.Error(), "ness sync")
	}
	changed, edited := driftOf(plan)
	switch {
	case len(changed) > 0:
		return fail("sync-drift", "would change "+strings.Join(changed, ", "), "ness sync")
	case len(edited) > 0:
		return warn("sync-drift", "edited since the last sync: nothing to do to keep it; to take ness's version: rm "+
			strings.Join(edited, " ")+" && ness sync", "")
	}
	return ok("sync-drift", "generated files are current")
}

// driftOf splits the plan into paths sync would change and paths the user edited, which sync
// keeps but which still differ from the render. A lock-only change counts as a change.
func driftOf(plan *nsync.Plan) (changed, edited []string) {
	for i := range plan.Files {
		fp := &plan.Files[i]
		if fp.Conflict || fp.Err != nil || fp.Writes() {
			changed = append(changed, fp.Path)
		} else if slices.ContainsFunc(fp.Steps, func(st nsync.Step) bool { return st.Action == nsync.ActKeep }) {
			edited = append(edited, fp.Path)
		}
	}
	if len(changed) == 0 {
		if lockChanged, err := plan.Changed(); err != nil || lockChanged {
			changed = append(changed, "the lock")
		}
	}
	return changed, edited
}

// checkDeadRoutes: open items addressed to closed or unknown sessions, deferred ones included (R-DOC-012).
func checkDeadRoutes(e *doctorEnv) check {
	return e.viaStore("dead-routes", func(s *store.Store) check {
		dead, err := s.DeadRoutes()
		if err != nil {
			return fail("dead-routes", err.Error(), "ness inbox")
		}
		if len(dead) > 0 {
			return fail("dead-routes", strings.Join(dead, "; "),
				"ness worker run (it retires them) or ness reply ITEM --as declined")
		}
		return ok("dead-routes", "every open item has a live recipient")
	})
}

// gonePanes lists the sessions whose pane the runtime reports gone; any other answer error is
// returned too, because an unanswered question is not a pane that exists.
func gonePanes(rt session.Runtime, open []store.Session) (gone []string, first string, askErr error) {
	for _, sess := range open {
		if sess.Pane == "" {
			continue
		}
		_, err := rt.State(sess.Pane)
		switch {
		case errors.Is(err, session.ErrGone):
			gone = append(gone, sess.ID+" (pane "+sess.Pane+")")
			if first == "" {
				first = sess.ID
			}
		case err != nil:
			askErr = err
		}
	}
	return gone, first, askErr
}

// checkGonePanes: open sessions whose pane the runtime no longer has (R-DOC-013).
func checkGonePanes(e *doctorEnv) check {
	return e.viaStore("gone-panes", func(s *store.Store) check {
		open, err := s.Sessions(false)
		if err != nil {
			return fail("gone-panes", err.Error(), "ness status")
		}
		gone, first, askErr := gonePanes(buildRuntime(e.d), open)
		if len(gone) > 0 {
			return fail("gone-panes", strings.Join(gone, "; "), "ness open --resume "+first)
		}
		if askErr != nil {
			return warn("gone-panes", "could not ask herdr: "+askErr.Error(), "")
		}
		return ok("gone-panes", "every open session has its pane")
	})
}

// checkStrayWorktrees: directories under .worktrees/ no session owns; a closed session whose
// worktree was kept still owns it (R-DOC-014).
func checkStrayWorktrees(e *doctorEnv) check {
	entries, err := os.ReadDir(filepath.Join(e.primary, ".worktrees"))
	if err != nil {
		return ok("stray-worktrees", "no .worktrees directory")
	}
	return e.viaStore("stray-worktrees", func(s *store.Store) check {
		all, serr := s.Sessions(true)
		if serr != nil {
			return fail("stray-worktrees", serr.Error(), "ness status --all")
		}
		owned := ownedWorktrees(all)
		var stray []string
		for _, d := range entries {
			if d.IsDir() && !owned[d.Name()] {
				stray = append(stray, ".worktrees/"+d.Name())
			}
		}
		if len(stray) > 0 {
			return fail("stray-worktrees", strings.Join(stray, ", ")+" without a session",
				"git worktree remove "+stray[0])
		}
		return ok("stray-worktrees", "every worktree has a session")
	})
}

func ownedWorktrees(all []store.Session) map[string]bool {
	owned := map[string]bool{}
	for _, sess := range all {
		if sess.Worktree != "" && (!sess.Closed || sess.KeepReason != "") {
			owned[filepath.Base(sess.Worktree)] = true
		}
	}
	return owned
}

// checkProbes: every live probe has a live owner (R-DOC-015); a paused one names its owner and
// the start that resumes it.
func checkProbes(e *doctorEnv) check {
	if !e.cfg.On("probes") {
		return ok("probes", "feature off")
	}
	return e.viaStore("probes", func(s *store.Store) check {
		rows, err := s.Probes(false)
		if err != nil {
			return fail("probes", err.Error(), "ness status")
		}
		var paused []store.Probe
		for _, p := range rows {
			if p.State == store.ProbePaused {
				paused = append(paused, p)
			}
		}
		if len(paused) == 0 {
			return ok("probes", fmt.Sprintf("%d active probes, every one has a live owner", len(rows)))
		}
		list := make([]string, 0, len(paused))
		for _, p := range paused {
			list = append(list, p.Path+" (owner "+p.Owner+" closed)")
		}
		return fail("probes", strings.Join(list, "; "), "ness probe start "+paused[0].Path+" (from a live session)")
	})
}

// checkOperatorNotify reports the channel the worker tells the operator on (R-DOC-024): a
// warning when none is configured, ok naming channel and kind, a failure when the channel cannot
// be built (its token not exported). Nothing is posted.
func checkOperatorNotify(e *doctorEnv) check {
	const name = "operator-notify"
	op := e.cfg.Notify.Operator
	if op == "" {
		return warn(name, "none configured: the operator is not told when a session needs them",
			"set [notify] operator = NAME to a channel of [notify.channels] in .ness/ness.toml, "+
				"with notify in features")
	}
	ch, err := buildChannel(e.d, e.cfg, e.primary, op, true)
	if err != nil {
		return fail(name, op+": "+err.Error(), "see the secrets check")
	}
	return ok(name, op+" ("+ch.Kind()+")")
}

// checkSecrets reports whether every reference of ness.toml resolves (R-CFG-085): the missing
// ones by key and reference, each with its fix, and a too-open secrets file. Never a value.
func checkSecrets(e *doctorEnv) check {
	const name = "secrets"
	r := secretResolver(e.cfg.Project)
	if _, err := config.ReadSecrets(r.Path); err != nil {
		return fail(name, err.Error(), "chmod 600 "+r.Path)
	}
	refs := 0
	var missing, fixes []string
	for _, cn := range slices.Sorted(maps.Keys(e.cfg.Notify.Channels)) {
		_, err := e.cfg.Notify.Channels[cn].Resolve(func(v string) (string, error) {
			source, ref, isRef := config.SecretRef(v)
			if !isRef {
				return "", nil
			}
			refs++
			if _, _, err := r.Resolve(v); err != nil {
				fixes = append(fixes, map[string]string{"env": "export ", "secret": "ness secret set "}[source]+ref)
				return "", fmt.Errorf("%s missing", v)
			}
			return "", nil
		})
		if err != nil {
			missing = append(missing, "notify.channels."+cn+"."+strings.ReplaceAll(err.Error(), "\n", ", "))
		}
	}
	if len(missing) > 0 {
		fixes = slices.Compact(slices.Sorted(slices.Values(fixes)))
		return fail(name, strings.Join(missing, "; "), strings.Join(fixes, "; "))
	}
	if refs == 0 {
		return ok(name, "no references in "+config.Path)
	}
	if refs == 1 {
		return ok(name, "1 reference resolves")
	}
	return ok(name, fmt.Sprintf("%d references resolve", refs))
}

// checkJev: under policy jev, whether the key and the pinned model answer one minimal POST
// /v1/systemone (R-DOC-025); GET /v1/models lists only aliases, so it cannot vouch for a pin.
// Never a failure: the selector falls back to declared. Never the key, never a response body.
func checkJev(e *doctorEnv) check {
	const name = "jev"
	if e.cfg.Models.Policy != "jev" {
		return ok(name, "policy "+e.cfg.Models.Policy)
	}
	j := e.cfg.Models.Jev
	key, _, err := secretResolver(e.cfg.Project).Resolve(j.Key)
	if err != nil || key == "" {
		return warn(name, "no key: models.jev.key = "+j.Key+" does not resolve; picks fall back to declared",
			"export the variable or ness secret set the reference")
	}
	body, err := json.Marshal(map[string]any{
		"model": j.Model,
		"state": map[string]string{"task": "ness doctor probe"},
		"questions": map[string]any{
			"probe": map[string]string{"type": "noul", "instructions": "Is `task` a probe?"},
		},
	})
	if err != nil {
		return warn(name, "encode probe: "+err.Error(), "")
	}
	url := strings.TrimSuffix(j.Endpoint, "/") + "/v1/systemone"
	ctx, cancel := context.WithTimeout(context.Background(), j.Timeout.Duration)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return warn(name, "endpoint "+j.Endpoint+": bad URL", "fix models.jev.endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.d.httpClient().Do(req)
	if err != nil {
		return warn(name, j.Endpoint+" unreachable (offline?); picks fall back to declared",
			"check the network or models.jev.endpoint")
	}
	_ = resp.Body.Close() //nolint:errcheck // the body is never read: it may echo the key
	switch resp.StatusCode {
	case http.StatusOK:
		return ok(name, j.Model+" answers at "+j.Endpoint)
	case http.StatusUnauthorized:
		return warn(name, "POST /v1/systemone: 401, key rejected; picks fall back to declared",
			"check the key models.jev.key names")
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return warn(name, fmt.Sprintf("model %s rejected (%d); picks fall back to declared", j.Model, resp.StatusCode),
			"pin an available model in models.jev.model")
	}
	return warn(name, fmt.Sprintf("POST /v1/systemone: %d; picks fall back to declared", resp.StatusCode),
		"check models.jev.endpoint")
}
