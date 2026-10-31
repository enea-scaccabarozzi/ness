package cli

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// newSecretCmd is ness secret (R-CFG-083): the project's secrets file, which no subcommand ever
// prints a value of.
func newSecretCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Keep the values ness.toml references as secret:NAME"}
	set := &cobra.Command{
		Use:   "set NAME",
		Short: "Store a secret read from stdin (never from the command line)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return withSecrets(d, func(path string, _ *config.Config) error {
				if err := config.CheckSecretName(args[0]); err != nil {
					return usage("%v", err)
				}
				value, err := readSecret(d)
				if err != nil {
					return err
				}
				if value == "" {
					return usage("empty value on stdin: nothing stored")
				}
				return config.SetSecret(path, args[0], value)
			})
		},
	}
	unset := &cobra.Command{
		Use:   "unset NAME",
		Short: "Remove a stored secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return withSecrets(d, func(path string, _ *config.Config) error {
				found, err := config.UnsetSecret(path, args[0])
				if err != nil {
					return refuse("%v", err)
				}
				if !found {
					return refuse("no secret %s in %s", args[0], path)
				}
				return nil
			})
		},
	}
	cmd.AddCommand(set, newSecretListCmd(d), unset)
	return cmd
}

func newSecretListCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the stored names, marking those ness.toml references",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return withSecrets(d, func(path string, cfg *config.Config) error {
				names, err := config.SecretNames(path)
				if err != nil {
					return refuse("%v", err)
				}
				used, err := referenced(cfg)
				if err != nil {
					return err
				}
				for _, n := range names {
					if slices.Contains(used, n) {
						n += "\treferenced"
					}
					say(d.Stdout, "%s", n)
				}
				return nil
			})
		},
	}
}

// withSecrets runs fn with the project's secrets file and configuration.
func withSecrets(d Deps, fn func(path string, cfg *config.Config) error) error {
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return err
	}
	return fn(secretResolver(cfg.Project).Path, cfg)
}

// readSecret reads the value from stdin, without echo on a terminal; one trailing newline goes.
func readSecret(d Deps) (string, error) {
	if f, ok := d.Stdin.(*os.File); ok && d.TTY {
		if _, err := fmt.Fprint(d.Stderr, "value (not echoed): "); err != nil {
			return "", err
		}
		b, err := term.ReadPassword(f.Fd())
		say(d.Stderr, "")
		return string(b), err
	}
	b, err := io.ReadAll(d.Stdin)
	return strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r"), err
}

// referenced lists the names ness.toml references as secret:NAME.
func referenced(cfg *config.Config) ([]string, error) {
	var names []string
	for _, cn := range slices.Sorted(maps.Keys(cfg.Notify.Channels)) {
		if _, err := cfg.Notify.Channels[cn].Resolve(func(v string) (string, error) {
			if source, n, ok := config.SecretRef(v); ok && source == "secret" {
				names = append(names, n)
			}
			return v, nil
		}); err != nil {
			return nil, err
		}
	}
	return names, nil
}
