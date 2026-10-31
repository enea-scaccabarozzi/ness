package cli

import (
	"context"
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/atom/systemd"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

func newWorkerCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "The project's worker: run it here, or install it as a systemd user unit",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	var once bool
	run := &cobra.Command{
		Use:   "run",
		Short: "Run the worker in this terminal: one process per project, a pass every 5 s",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runWorker(d, once) },
	}
	run.Flags().BoolVar(&once, "once", false, "run one pass and exit")
	install := &cobra.Command{
		Use:   "install",
		Short: "Write and start the systemd user unit ness-<project>.service",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return installWorker(d) },
	}
	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Stop, disable and remove the unit",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return uninstallWorker(d) },
	}
	cmd.AddCommand(run, install, uninstall)
	return cmd
}

// workerEnv is what every worker subcommand needs: the project, its config, its primary checkout.
type workerEnv struct {
	p       *project
	cfg     *config.Config
	primary string
}

func loadWorkerEnv(d Deps) (workerEnv, error) {
	p, err := loadProject(d)
	if err != nil {
		return workerEnv{}, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return workerEnv{}, err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return workerEnv{}, err
	}
	return workerEnv{p: p, cfg: cfg, primary: primary}, nil
}

// installWorker refuses without a systemd user session (R-WRK-004) and writes the unit with this
// binary's real path (R-WRK-002, R-WRK-003), which doctor's worker-binary check reads back, and
// the PATH of this shell, so the unit finds herdr and the client.
func installWorker(d Deps) error {
	env, err := loadWorkerEnv(d)
	if err != nil {
		return err
	}
	sup := systemd.Supervisor{Exec: d.exec(), Home: userHome()}
	if aerr := sup.Available(); aerr != nil {
		return refuse("%v; run ness worker run in a terminal instead", aerr)
	}
	bin := realPath(d.Executable)
	path, err := sup.Install(env.cfg.Project, bin, env.primary, os.Getenv("PATH"))
	if errors.Is(err, systemd.ErrForeign) {
		return refuse("%v", err)
	}
	if err != nil {
		return err
	}
	done(d, "installed %s (ExecStart=%s worker run); check with ness doctor or systemctl --user status %s",
		path, bin, systemd.UnitName(env.cfg.Project))
	return nil
}

// repointWorker points the project's unit at bin when it runs another binary (by real path),
// keeping the unit's PATH (the caller's only when the unit has none):
// Install rewrites it, reloads systemd and restarts the worker. No unit, or the same binary, is
// false, nil; a caller that needs fresh code anyway calls restartWorker.
func repointWorker(d Deps, project, primary, bin string) (changed bool, err error) {
	sup := systemd.Supervisor{Exec: d.exec(), Home: userHome()}
	cur, err := sup.ExecBinary(project)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	bin = realPath(bin)
	if realPath(cur) == bin {
		return false, nil
	}
	path, err := sup.EnvPATH(project)
	if err != nil {
		return false, err
	}
	if path == "" {
		path = os.Getenv("PATH")
	}
	if _, err = sup.Install(project, bin, primary, path); err != nil {
		return false, err
	}
	return true, nil
}

// restartWorker restarts the project's unit so it runs the code now on disk; without a unit it
// does nothing and reports false.
func restartWorker(d Deps, project string) (restarted bool, err error) {
	sup := systemd.Supervisor{Exec: d.exec(), Home: userHome()}
	if _, serr := os.Stat(sup.UnitPath(project)); errors.Is(serr, fs.ErrNotExist) {
		return false, nil
	}
	_, err = d.exec()("", "systemctl", "--user", "restart", systemd.UnitName(project)+".service")
	return err == nil, err
}

// uninstallWorker stops and removes the unit install wrote (R-WRK-006).
func uninstallWorker(d Deps) error {
	env, err := loadWorkerEnv(d)
	if err != nil {
		return err
	}
	sup := systemd.Supervisor{Exec: d.exec(), Home: userHome()}
	err = sup.Uninstall(env.cfg.Project, env.primary)
	if errors.Is(err, systemd.ErrForeign) {
		return refuse("%v", err)
	}
	if err != nil {
		return err
	}
	done(d, "uninstalled %s", sup.UnitPath(env.cfg.Project))
	return nil
}

// runWorker refuses without herdr (R-WRK-007) or beside another worker (R-WRK-001), then passes
// every 5 s until a signal or a binary change (R-WRK-009, R-WRK-024); --once runs one pass. It
// needs no systemd (R-WRK-005).
//
// ponytail: a hung herdr call stalls the loop (SystemExec has no timeout, M8); doctor flags the
// stale heartbeat. Upgrade: a per-call timeout in the exec seam.
func runWorker(d Deps, once bool) error {
	env, err := loadWorkerEnv(d)
	if err != nil {
		return err
	}
	w, lg, closeAll, err := openWorker(d, env)
	if err != nil {
		return err
	}
	defer closeAll()
	if once {
		defer w.Wait() // the floors the pass started finish and are booked (R-PRB-089)
		return w.Pass()
	}
	loop := &worker.Loop{
		Pass: w.Pass, Binary: func() (string, error) { return binaryVersion(d) },
		Version: d.fullVersion(), Log: lg, Interval: worker.Interval, Check: worker.CheckEvery,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// A stop signal kills the floors with the worker: their results are not booked, they run again
	// after the restart (R-PRB-089). Stop must be set before they end, so it follows the signal.
	defer context.AfterFunc(ctx, w.Stop)()
	defer w.Stop()
	if err = loop.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// deliverer is a client the worker can deliver to: it reads the client's screen and knows the
// texts that resume a cut-short turn and rename a running conversation (R-WRK-090).
type deliverer interface {
	ReadScreen(ansi string, now time.Time) session.Screen
	ResumeText() string
	RenameText(name string) string
}

// workerClients are the clients that are deliverers; a session of any other client is logged as
// not delivered to and reported by doctor (R-WRK-090).
func workerClients(d Deps) map[string]worker.Client {
	out := map[string]worker.Client{}
	for _, name := range config.Clients {
		if c, ok := buildClient(d, name).(deliverer); ok {
			out[name] = worker.Client{Read: c.ReadScreen, Resume: c.ResumeText(), Rename: c.RenameText}
		}
	}
	return out
}

// withActivity gives the claude client its transcript reading (R-WRK-096): the conversation of a
// session lives under the user's home, in the project directory of the session's worktree. One
// reader serves the worker, so each transcript is read once and then only for what was appended.
func withActivity(clients map[string]worker.Client, primary string) map[string]worker.Client {
	c, ok := clients["claude"]
	if !ok {
		return clients
	}
	tr := &claude.Transcripts{}
	c.Activity = func(sess store.Session, now time.Time) (session.Activity, bool) {
		if sess.ClientSession == "" {
			return session.Activity{}, false
		}
		return tr.Activity(claude.TranscriptPath(userHome(), transcriptDir(primary, sess), sess.ClientSession), now)
	}
	clients["claude"] = c
	return clients
}

// openWorker checks herdr, takes the single-instance lock, and only then opens the store and the
// log, so a refused second worker touches neither (R-WRK-001).
func openWorker(d Deps, env workerEnv) (w *worker.Worker, lg *worker.Log, closeAll func(), err error) {
	rt := buildRuntime(d)
	if herr := session.CheckHerdr(rt); herr != nil {
		return nil, nil, nil, refuse("%v", herr)
	}
	path, err := store.Locate(env.p.root)
	if err != nil {
		return nil, nil, nil, refuse("%v", err)
	}
	dir := filepath.Dir(path)
	if merr := os.MkdirAll(dir, 0o750); merr != nil {
		return nil, nil, nil, merr
	}
	release, err := worker.Lock(dir)
	if errors.Is(err, worker.ErrLocked) {
		return nil, nil, nil, refuse("%v", err)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	s, err := store.Open(path)
	if err != nil {
		release()
		return nil, nil, nil, refuse("%v", err)
	}
	// A live project is registered even before its first writing sync, so the ceiling and the
	// install veto see its pin (R-UPD-031).
	record(d, env.p)
	lg = &worker.Log{Path: filepath.Join(dir, "worker.log"), Now: d.Now, Err: d.Stderr}
	w = &worker.Worker{
		Store: s, Runtime: rt,
		Clients: withActivity(workerClients(d), env.primary),
		Project: env.cfg.Project,
		Usage:   buildUsage(),
		Version: d.fullVersion(), Now: d.Now, Log: lg,
		Stall: time.Duration(env.cfg.Worker.StallMinutes) * time.Minute,
		Quiet: time.Duration(env.cfg.Worker.QuietSeconds) * time.Second,
	}
	bases := map[string]baseLock{}
	w.Stale = func(sess store.Session) (worker.Stale, bool, error) { return staleLock(d, env.primary, sess, bases) }
	w.Update = unattended(d, env, s)
	wireProbes(d, env, s, w)
	wireNotify(d, env, s, w)
	return w, lg, func() { _ = s.Close(); release() }, nil //nolint:errcheck // the process ends here
}

// wireNotify fits the operator channel (CX-16) when [notify] operator names one and the notify
// feature is on; like the probes, a change needs a worker restart. The channel is built at each
// post, so a token exported later is picked up without one.
func wireNotify(d Deps, env workerEnv, s *store.Store, w *worker.Worker) {
	op := env.cfg.Notify.Operator
	if op == "" || !env.cfg.On("notify") {
		return
	}
	poster := buildPoster(d, env.cfg, env.primary, s, true)
	w.Notify = alertSender(op, poster.Post)
}

// alertSender renders an alert to Markdown and posts it to channel in the thread of its root, so a
// program's alerts group together (R-WRK-082). post is the poster's Post.
func alertSender(channel string, post func(notify.Request) (int, error)) func(worker.Alert) error {
	return func(a worker.Alert) error {
		_, err := post(notify.Request{Channel: channel, Markdown: a.Markdown(), Thread: "alerts:" + a.Root})
		return err
	}
}

// wireProbes fits the probe slot when the probes feature is on. The worker decides this once, at
// start: enabling the feature in ness.toml needs a worker restart (P-32). So does every other change
// to the configuration Check reads: turning notify off or removing a channel makes a notify probe a
// file error (no floor runs) until the worker restarts, and restoring it does not help before then.
func wireProbes(d Deps, env workerEnv, s *store.Store, w *worker.Worker) {
	if !env.cfg.On("probes") {
		return
	}
	w.Probes = &worker.Probes{
		Primary: env.primary, Redact: env.cfg.Probes.RedactEnv, Getenv: os.Getenv, Run: d.shell(),
		Rand:  rand.New(rand.NewPCG(uint64(d.Now().UnixNano()), 1)), //nolint:gosec // jitter and sample
		Check: func(pf probe.Probe) error { return checkNotify(env.cfg, pf) },
	}
	if env.cfg.On("notify") {
		poster := buildPoster(d, env.cfg, env.primary, s, true)
		w.Probes.Post = func(channel, markdown, thread string) error {
			_, err := poster.Post(notify.Request{Channel: channel, Markdown: markdown, Thread: thread})
			return err
		}
	}
}

// binaryVersion runs the binary at this process's real path (the unit's ExecStart) with
// --version, which prints "ness version X", and returns X (R-WRK-009).
func binaryVersion(d Deps) (string, error) {
	out, err := d.exec()("", realPath(d.Executable), "--version")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", errors.New("--version printed nothing")
	}
	return fields[len(fields)-1], nil
}
