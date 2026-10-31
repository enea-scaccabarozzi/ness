package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/internal/update"
)

// probeTimeout bounds the new binary's --version run before it is renamed into place.
const probeTimeout = 10 * time.Second

func newSelfUpdateCmd(d Deps) *cobra.Command {
	var version, channel string
	var check bool
	cmd := &cobra.Command{
		Use: "self-update",
		Example: "  # the newest release on the project's channel (stable outside a project)\n  ness self-update\n" +
			"  # only say whether a newer release exists\n  ness self-update --check\n" +
			"  # install a specific release\n  ness self-update --version 0.2.0",
		Short: "Install a newer ness release in place of this binary",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runSelfUpdate(d, version, channel, check) },
	}
	cmd.Flags().StringVar(&version, "version", "", "install release `X.Y.Z` instead of the newest")
	cmd.Flags().StringVar(&channel, "channel", "",
		"release `channel`: stable or rc (default: the project's [update] channel)")
	cmd.Flags().BoolVar(&check, "check", false, "report whether a newer release exists; install nothing")
	return cmd
}

// runSelfUpdate resolves the target on the channel (the flag, else the project's, else stable)
// from one release fetch; --check reports it and records the cache, otherwise selfInstall does.
func runSelfUpdate(d Deps, want, channel string, check bool) error {
	if d.Releases == nil {
		return refuse("no release source")
	}
	flag := channel
	channel, err := pickChannel(channel)
	if err != nil {
		return err
	}
	rels, err := d.Releases.Releases(context.Background())
	if err != nil {
		return refuse("release list: %v", err)
	}
	if check {
		writeCheckCache(d, rels)
	}
	rel, err := update.Resolve(rels, channel, want)
	if err != nil {
		return refuse("%v", err)
	}
	if (want == "" || check) && !newerThan(rel.Version, d.Version) {
		say(d.Stdout, "ness %s is the latest on channel %s", d.Version, channel)
		return nil
	}
	if check {
		say(d.Stdout, "ness %s available (installed %s): %s", rel.Version, d.Version, checkFix(flag))
		return nil
	}
	return selfInstall(d, rel)
}

// checkFix is the command --check offers: with --channel, that channel, not the project's, so
// it installs what was reported.
func checkFix(flag string) string {
	if flag == "" {
		return "ness self-update"
	}
	return "ness self-update --channel " + flag
}

// selfInstall installs rel unless another tool manages the binary or a registered checkout cannot
// run it (R-UPD-045, R-UPD-046).
func selfInstall(d Deps, rel update.Release) error {
	// The ceiling first: a managed binary's tool command must not name a version it refuses.
	pins, err := machinePins(d)
	if err != nil {
		return err
	}
	if cerr := update.Ceiling(rel.Version, pins); cerr != nil {
		return refuse("%v", cerr)
	}
	bin := realPath(d.Executable)
	if command, ok := update.ManagedBy(bin, rel.Version, os.Getenv, realPath(userHome())); ok {
		return refuse("ness at %s is managed by another tool; run: %s", bin, command)
	}
	prev, err := installVersion(d, rel)
	if prev != "" {
		done(d, "ness %s installed at %s (previous kept at %s); workers pick it up within a minute",
			rel.Version, bin, prev)
	}
	return err
}

// pickChannel is the --channel flag, else the project's [update] channel.
func pickChannel(flag string) (string, error) {
	if flag == "" {
		return projectChannel()
	}
	if !slices.Contains(config.UpdateChannels, flag) {
		return "", usage("--channel %s: want stable or rc", flag)
	}
	return flag, nil
}

// projectChannel is the current project's [update] channel, "stable" outside a project. It reads
// only the file's head: self-update skips the pin check (R-CFG-011) and must run under a ness.toml
// a newer binary wrote.
func projectChannel() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := repoRoot(wd)
	if err != nil {
		return "stable", nil //nolint:nilerr // outside a repository: the default channel (R-CFG-053)
	}
	data, err := os.ReadFile(filepath.Join(root, config.Path)) //nolint:gosec // fixed path under the repo
	if errors.Is(err, fs.ErrNotExist) {
		return "stable", nil
	}
	if err != nil {
		return "", err
	}
	h, err := config.ReadHead(data)
	if err != nil {
		return "", refuse("%s: %v", filepath.Join(root, config.Path), err)
	}
	return h.Update.Channel, nil
}

// writeCheckCache records rels in the check cache; a failure is printed, never fatal.
func writeCheckCache(d Deps, rels []update.Release) {
	path, err := update.CachePath(d.CacheDir)
	if err == nil {
		err = update.WriteCache(path, update.CacheFrom(rels, d.Now()))
	}
	if err != nil {
		printErr(d.Stderr, fmt.Errorf("update cache: %w", err))
	}
}

// newerThan reports that release a is newer than b; an unparsable version is never newer.
func newerThan(a, b string) bool {
	c, err := nsync.Compare(a, b)
	return err == nil && c > 0
}

// machinePins is the pin of every registered project's base checkout and of each of its
// worktrees holding a ness.toml; Primary is the registered root.
func machinePins(d Deps) ([]update.Pin, error) {
	roots, err := d.Registry.List()
	if err != nil {
		return nil, err
	}
	var pins []update.Pin
	for _, root := range roots {
		trees, err := worktrees(root)
		if err != nil {
			return nil, err
		}
		for _, t := range trees {
			path := filepath.Join(t, config.Path)
			h, err := readHeadAt(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			pins = append(pins, update.Pin{Path: path, Pin: h.Ness, Primary: root})
		}
	}
	return pins, nil
}

// worktrees lists the checkouts of the repository at root, the main one first.
func worktrees(root string) ([]string, error) {
	args := []string{"-C", root, "worktree", "list", "--porcelain"}
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed git subcommand
	cmd.Env = store.CleanGitEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list in %s: %w", root, err)
	}
	var trees []string
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			trees = append(trees, p)
		}
	}
	return trees, nil
}

func readHeadAt(path string) (config.Head, error) {
	data, err := os.ReadFile(path) //nolint:gosec // a registered project's ness.toml
	if err != nil {
		return config.Head{}, err
	}
	h, err := config.ReadHead(data)
	if err != nil {
		return config.Head{}, fmt.Errorf("%s: %w", path, err)
	}
	return h, nil
}

// installVersion downloads rel for this platform, verifies its checksum before extracting it,
// installs it over the real file behind d.Executable (keeping .prev) and re-points every
// registered worker unit that runs another binary. It checks neither the managed binary nor the
// ceiling: its callers do. A failure before the rename is a refusal and prev is ""; a failure
// re-pointing a worker comes after the install, with prev set.
func installVersion(d Deps, rel update.Release) (prev string, err error) {
	ctx := context.Background()
	asset, err := rel.Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", refuse("%v", err)
	}
	sumsURL, ok := rel.Assets[update.Checksums]
	if !ok {
		return "", refuse("ness %s has no %s", rel.Version, update.Checksums)
	}
	archive, err := fetchAll(ctx, d.Releases, asset)
	if err != nil {
		return "", refuse("download %s: %v", asset, err)
	}
	sums, err := fetchAll(ctx, d.Releases, sumsURL)
	if err != nil {
		return "", refuse("download %s: %v", update.Checksums, err)
	}
	if verr := update.Verify(sums, update.AssetName(rel.Version, runtime.GOOS, runtime.GOARCH), archive); verr != nil {
		return "", refuse("%v", verr)
	}
	data, err := update.Extract(archive)
	if err != nil {
		return "", refuse("%v", err)
	}
	bin := realPath(d.Executable)
	prev, err = update.Install(bin, data, rel.Version, probeVersion)
	if err != nil {
		return "", refuse("install %s: %v", bin, err)
	}
	return prev, repointWorkers(d, bin)
}

// repointWorkers points every registered project's worker unit at bin.
func repointWorkers(d Deps, bin string) error {
	roots, err := d.Registry.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, root := range roots {
		h, err := readHeadAt(filepath.Join(root, config.Path))
		if err == nil {
			_, err = repointWorker(d, h.Project, root, bin)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("worker of %s: %w", root, err))
		}
	}
	return errors.Join(errs...)
}

func fetchAll(ctx context.Context, src update.Source, url string) ([]byte, error) {
	rc, err := src.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(rc)
	return data, errors.Join(err, rc.Close())
}

// probeVersion runs the just-written binary's --version, bounded by probeTimeout. It runs the
// real file on purpose: the file self-update wrote is not an external program to fake.
func probeVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version") //nolint:gosec // the binary just written
	cmd.Env = store.CleanGitEnv(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
