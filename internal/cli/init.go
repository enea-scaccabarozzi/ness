package cli

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/store"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/profiles"
)

type initOpts struct {
	profiles                    []string
	client, base, gate, project string
	yes                         bool
}

func newInitCmd(d Deps) *cobra.Command {
	var o initOpts
	cmd := &cobra.Command{
		Use: "init",
		Example: "  # asks for what it cannot infer\n  ness init\n" +
			"  # no questions\n  ness init --gate 'make validate' --profile go-strict --yes",
		Short: "Create .ness/ness.toml and render the harness into this repository",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runInit(d, o) },
	}
	f := cmd.Flags()
	f.StringArrayVar(&o.profiles, "profile", nil, "apply profile `NAME` (repeatable, applied in order)")
	f.StringVar(&o.client, "client", "", "client: claude or codex (default claude)")
	f.StringVar(&o.base, "base", "", "base branch (default: the current branch)")
	f.StringVar(&o.gate, "gate", "", "gate `CMD` that ness integrate runs")
	f.StringVar(&o.project, "project", "", "project name (default: the repository directory name)")
	f.BoolVar(&o.yes, "yes", false, "never prompt")
	return cmd
}

// runInit writes .ness/ness.toml and syncs. It plans the sync from the new file before writing
// anything, so an unknown profile or a conflict between profiles leaves the repository as it
// was (R-CFG-025).
func runInit(d Deps, o initOpts) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := repoRoot(wd)
	if err != nil {
		return err
	}
	if _, serr := os.Lstat(filepath.Join(root, config.Path)); !errors.Is(serr, fs.ErrNotExist) {
		return refuse("%s already exists; edit it and run ness sync", config.Path)
	}
	o, err = prepareInit(d, o, root)
	if err != nil {
		return err
	}
	pin, err := config.PinOf(d.Version)
	if err != nil {
		return refuse("%v", err)
	}
	data := initFile(o, pin)
	file, err := config.Parse(data)
	if err != nil {
		return refuse("%v", err)
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	plan, err := planSync(d, &project{root: root, file: file}, rootFS)
	if err == nil {
		err = writeInit(rootFS, data, plan)
	}
	if cerr := errors.Join(err, rootFS.Close()); cerr != nil {
		return cerr
	}
	record(d, &project{root: root, file: file})
	report(d, plan)
	err = syncExit(plan, false)
	nextSteps(d, o.project, err)
	return err
}

// nextSteps closes a rich init with what to do next; plain init prints only the sync report.
func nextSteps(d Deps, project string, err error) {
	if d.rich() && err == nil {
		done(d, "ness is set up in %s; next: commit the files, then run ness doctor", project)
	}
}

func writeInit(rootFS *os.Root, data []byte, plan *nsync.Plan) error {
	if err := rootFS.MkdirAll(".ness", 0o750); err != nil {
		return err
	}
	if err := rootFS.WriteFile(config.Path, data, 0o644); err != nil { //nolint:gosec // committed config
		return err
	}
	return nsync.Apply(rootFS, plan)
}

// prepareInit infers project and base, prompts on a terminal for the values no flag gave
// (R-CFG-051) and refuses what is still missing (R-CFG-052).
func prepareInit(d Deps, o initOpts, root string) (initOpts, error) {
	given := o
	if o.project == "" {
		o.project = filepath.Base(root)
	}
	if o.base == "" {
		o.base = currentBranch(root)
	}
	if err := askInit(d, &o, given); err != nil {
		return o, err
	}
	var missing []string
	if o.gate == "" {
		missing = append(missing, `--gate (the command ness integrate runs, e.g. --gate "make validate")`)
	}
	if o.base == "" {
		missing = append(missing, "--base (HEAD is detached, so the base branch cannot be inferred)")
	}
	if len(missing) > 0 {
		return o, refuse("missing %s", strings.Join(missing, " and "))
	}
	return o, nil
}

// promptInit asks for each value no flag gave; the inferred default shows in brackets and an
// empty answer takes it. A default client stays out of ness.toml.
func promptInit(d Deps, o *initOpts, given initOpts) error {
	rd := bufio.NewReader(d.Stdin)
	ask := func(name, flag string, value *string) error {
		if flag != "" {
			return nil
		}
		prompt := name + ": "
		if *value != "" {
			prompt = name + " [" + *value + "]: "
		}
		if _, err := fmt.Fprint(d.Stdout, prompt); err != nil {
			return err
		}
		line, err := rd.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if v := strings.TrimSpace(line); v != "" {
			*value = v
		}
		return nil
	}
	client := cmp.Or(o.client, "claude")
	err := errors.Join(ask("project", given.project, &o.project), ask("base", given.base, &o.base),
		ask("gate", given.gate, &o.gate), ask("client", given.client, &client))
	if client != "claude" {
		o.client = client
	}
	return err
}

// askInit asks on a terminal for what no flag gave: a form when rich, else line prompts.
func askInit(d Deps, o *initOpts, given initOpts) error {
	switch {
	case !d.TTY || o.yes:
		return nil
	case d.rich():
		return formInit(d, o, given)
	}
	return promptInit(d, o, given)
}

// formInit asks for the same values as promptInit in one form, and offers the built-in profiles
// when no --profile was given (R-UI-026).
func formInit(d Deps, o *initOpts, given initOpts) error {
	var fields []huh.Field
	input := func(title, flag, desc string, value *string, required bool) {
		if flag != "" {
			return
		}
		in := huh.NewInput().Title(title).Description(desc).Value(value)
		if required {
			in = in.Validate(huh.ValidateNotEmpty())
		}
		fields = append(fields, in)
	}
	input("Project", given.project, "the project name", &o.project, true)
	input("Base branch", given.base, "the branch sessions start from and integrate into", &o.base, true)
	input("Gate", given.gate, "the command ness integrate runs, e.g. make validate", &o.gate, true)
	client := cmp.Or(o.client, "claude")
	if given.client == "" {
		fields = append(fields, huh.NewSelect[string]().Title("Client").Value(&client).
			Options(huh.NewOption("Claude Code", "claude"),
				huh.NewOption(`Codex (experimental: needs experimental = ["codex"])`, "codex")))
	}
	if len(given.profiles) == 0 {
		if names := builtinProfiles(); len(names) > 0 {
			fields = append(fields, huh.NewMultiSelect[string]().Title("Profiles").
				Description("x or space toggles, enter confirms").Options(huh.NewOptions(names...)...).
				Height(len(names)+2).
				Value(&o.profiles))
		}
	}
	if len(fields) == 0 { // every value came from a flag
		return nil
	}
	t := d.theme()
	say(d.out(), "%s %s", t.Badge("ness init", "accent"), t.Dim.Render("set up the harness in this repository"))
	if err := t.Form(d.Stdin, d.Stdout, huh.NewGroup(fields...)).Run(); err != nil {
		return refuse("init canceled: %v", err)
	}
	if client != "claude" {
		o.client = client
	}
	return nil
}

// builtinProfiles is the names of the built-in profiles, the example ones left out.
func builtinProfiles() []string {
	entries, err := fs.ReadDir(profiles.Builtin(), ".")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "example-") {
			names = append(names, e.Name())
		}
	}
	return names
}

func currentBranch(root string) string {
	args := []string{"-C", root, "symbolic-ref", "--quiet", "--short", "HEAD"}
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed git subcommand
	cmd.Env = store.CleanGitEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// initFile writes the keys init knows, in a fixed order.
// ponytail: strconv.Quote equals a TOML basic string for printable text; config.Parse rejects
// anything else before it is written.
func initFile(o initOpts, pin string) []byte {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "ness = %s\nproject = %s\nbase = %s\ngate = %s\n",
		strconv.Quote(pin), strconv.Quote(o.project), strconv.Quote(o.base), strconv.Quote(o.gate))
	if o.client != "" {
		_, _ = fmt.Fprintf(&b, "client = %s\n", strconv.Quote(o.client))
	}
	if len(o.profiles) > 0 {
		quoted := make([]string, len(o.profiles))
		for i, p := range o.profiles {
			quoted[i] = strconv.Quote(p)
		}
		_, _ = fmt.Fprintf(&b, "profiles = [%s]\n", strings.Join(quoted, ", "))
	}
	return []byte(b.String())
}
