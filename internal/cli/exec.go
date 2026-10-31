package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// SystemExec runs name with args in dir and returns its stdout. The external programs ness starts
// (herdr, claude, codex, gh, systemctl) go through here, with the git discovery variables stripped
// so a hook's GIT_DIR cannot point ness at another repository. Repository discovery runs git
// directly (project.go, init.go, store.Locate) because tests use real git there. A missing program
// is reported with exec.ErrNotFound; any other failure carries the program's stderr.
func SystemExec(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), name, args...) //nolint:gosec // the atoms pass fixed argv
	cmd.Dir = dir
	cmd.Env = store.CleanGitEnv(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// exec returns the process's exec seam.
func (d Deps) exec() func(dir, name string, args ...string) ([]byte, error) {
	if d.Exec == nil {
		return SystemExec
	}
	return d.Exec
}

// shell returns the process's command runner.
func (d Deps) shell() shell.Runner {
	if d.Shell == nil {
		return shell.Run
	}
	return d.Shell
}

// transport returns the injected HTTP transport, or the default one.
func (d Deps) transport() http.RoundTripper {
	if d.Transport == nil {
		return http.DefaultTransport
	}
	return d.Transport
}

// httpClient is the client the channel atoms post with: the injected transport, 30 s per request.
func (d Deps) httpClient() *http.Client {
	return &http.Client{Transport: d.transport(), Timeout: 30 * time.Second}
}
