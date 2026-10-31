package ness_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// shim writes an executable sh script name into shimDir.
func shim(t *testing.T, shimDir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(shimDir, name), []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil { //nolint:gosec // a shim must be executable
		t.Fatal(err)
	}
}

// runInstall runs `sh install.sh args` against the release server at url, installing into dir
// (unset when dir is empty), with PATH = shims + /usr/bin:/bin and HOME a temp dir. env adds
// variables, later ones winning; SHIM_UNAME adds a uname shim (-m prints it, -s prints Linux),
// SHIM_NO_CURL a curl that exits 127, PATH_PREFIX a directory put first on PATH.
func runInstall(t *testing.T, url, dir string, args, env []string) (string, error) {
	t.Helper()
	script, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	shims := t.TempDir()
	path := shims + ":/usr/bin:/bin"
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		switch k {
		case "SHIM_UNAME":
			shim(t, shims, "uname", `case "$1" in -m) echo "$SHIM_UNAME" ;; -s) echo Linux ;; *) exec /bin/uname "$@" ;; esac`)
		case "SHIM_NO_CURL":
			shim(t, shims, "curl", "exit 127")
		case "PATH_PREFIX":
			path = v + ":" + path
		}
	}
	vars := []string{"PATH=" + path, "HOME=" + t.TempDir(), "NESS_RELEASES_URL=" + url}
	if dir != "" {
		vars = append(vars, "NESS_INSTALL_DIR="+dir)
	}
	cmd := exec.CommandContext(t.Context(), "sh", append([]string{script}, args...)...)
	cmd.Env = append(vars, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// versionOf runs bin --version and returns its last field.
func versionOf(t *testing.T, bin string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), bin, "--version").Output()
	if err != nil {
		t.Fatalf("%s --version: %v", bin, err)
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		t.Fatalf("%s --version printed nothing", bin)
	}
	return f[len(f)-1]
}

func entries(t *testing.T, dir string) int {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(es)
}

func TestInstallSh(t *testing.T) {
	// covers: R-UPD-093
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.1.0"}, testkit.FakeRelease{Version: "0.3.0-rc.1", Prerelease: true}, testkit.FakeRelease{Version: "0.2.0"})
	for _, c := range []struct {
		name      string
		args, env []string
		want      string
	}{
		// rejects: the first tag of the listing taken whatever its prerelease flag
		{"latest stable, not the newer rc", nil, nil, "0.2.0"},
		// rejects: NESS_VERSION ignored
		{"pinned", nil, []string{"NESS_VERSION=0.1.0"}, "0.1.0"},
		// rejects: a v-prefixed pin used verbatim (vv0.1.0 tag, v0.1.0 asset name)
		{"pinned with v", nil, []string{"NESS_VERSION=v0.1.0"}, "0.1.0"},
		// rejects: --channel rc ignored, or rc picking the newest stable
		{"rc", []string{"--channel", "rc"}, nil, "0.3.0-rc.1"},
	} {
		dir := t.TempDir()
		out, err := runInstall(t, srv.URL, dir, c.args, c.env)
		if err != nil {
			t.Fatalf("%s: %v\n%s", c.name, err, out)
		}
		if v := versionOf(t, filepath.Join(dir, "ness")); v != c.want {
			t.Errorf("%s: installed %s, want %s", c.name, v, c.want)
		}
		if !strings.Contains(out, "ness "+c.want+" installed to "+dir+"/ness") {
			t.Errorf("%s: no installed line:\n%s", c.name, out)
		}
		// rejects: a silent install into a directory the shell will not find
		if !strings.Contains(out, `note: `+dir+` is not on your PATH; add it: export PATH="`+dir+`:$PATH"`) {
			t.Errorf("%s: no PATH note:\n%s", c.name, out)
		}
	}
}

func TestInstallShChecksumMismatch(t *testing.T) {
	// covers: R-UPD-093
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0", BadSum: true})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, nil)
	// rejects: an install that skips or ignores the sha256 check
	if err == nil || !strings.Contains(out, "install.sh: checksum mismatch for ness_0.2.0_linux_amd64.tar.gz") {
		t.Fatalf("%v\n%s", err, out)
	}
	// rejects: the binary copied before the check passed
	if n := entries(t, dir); n != 0 {
		t.Fatalf("install dir has %d entries, want none", n)
	}
}

func TestInstallShUnknownArch(t *testing.T) {
	// covers: R-UPD-093
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, []string{"SHIM_UNAME=riscv64"})
	// rejects: an unknown arch falling back to amd64, or a bare 404 from the download
	if err == nil || !strings.Contains(out, "install.sh: no ness build for linux/riscv64; build from source: go install github.com/enea-scaccabarozzi/ness/cmd/ness@latest") {
		t.Fatalf("%v\n%s", err, out)
	}
	if n := entries(t, dir); n != 0 {
		t.Fatalf("install dir has %d entries", n)
	}
}

func TestInstallShAarch64(t *testing.T) {
	// covers: R-UPD-093
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0", Arches: []string{"arm64"}})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, []string{"SHIM_UNAME=aarch64"})
	// rejects: uname -m used verbatim (no aarch64 asset exists), or the arch hard-coded to amd64
	if err != nil || versionOf(t, filepath.Join(dir, "ness")) != "0.2.0" {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestInstallShOnPath(t *testing.T) {
	// covers: R-UPD-093
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, []string{"PATH_PREFIX=" + dir})
	// rejects: the PATH note printed unconditionally
	if err != nil || strings.Contains(out, "is not on your PATH") {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestInstallShDefaultDir(t *testing.T) {
	// covers: R-UPD-093
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	home := t.TempDir()
	out, err := runInstall(t, srv.URL, "", nil, []string{"HOME=" + home})
	// rejects: a default other than ~/.local/bin, or a missing directory not created
	want := filepath.Join(home, ".local", "bin", "ness")
	if err != nil || !strings.Contains(out, "installed to "+want) || versionOf(t, want) != "0.2.0" {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestInstallShWget(t *testing.T) {
	// covers: R-UPD-093
	if _, err := exec.LookPath("wget"); err != nil {
		t.Skip("no wget on this system")
	}
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, []string{"SHIM_NO_CURL=1"})
	// rejects: curl as the only downloader
	if err != nil || versionOf(t, filepath.Join(dir, "ness")) != "0.2.0" {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestInstallShTruncated(t *testing.T) {
	// covers: R-UPD-093
	full, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	// rejects: top-level commands run as sh parses a `curl | sh` stream cut short, leaving a
	// partial install or a stray .ness.install
	for _, n := range []int{len(full) * 3 / 10, len(full) * 6 / 10, len(full) * 9 / 10, len(full) - len("main \"$@\"\n")} {
		script := filepath.Join(t.TempDir(), "install.sh")
		if err := os.WriteFile(script, full[:n], 0o600); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		cmd := exec.CommandContext(t.Context(), "sh", script)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "NESS_RELEASES_URL=" + srv.URL, "NESS_INSTALL_DIR=" + dir}
		out, _ := cmd.CombinedOutput() //nolint:errcheck // a cut may be a syntax error or a clean no-op
		if got := entries(t, dir); got != 0 || strings.Contains(string(out), "installed to") {
			t.Fatalf("cut at %d bytes: install dir has %d entries\n%s", n, got, out)
		}
	}
}
